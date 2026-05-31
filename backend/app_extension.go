package backend

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type BrowserExtension = config.BrowserExtension

type chromeExtensionManifest struct {
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Version         string            `json:"version"`
	ManifestVersion int               `json:"manifest_version"`
	DefaultLocale   string            `json:"default_locale"`
	Icons           map[string]string `json:"icons"`
}

type chromeLocaleMessage struct {
	Message string `json:"message"`
}

const chromeWebStoreDownloadProdVersion = "140.0.0.0"

func (a *App) BrowserExtensionList() []BrowserExtension {
	if a == nil || a.config == nil {
		return []BrowserExtension{}
	}

	a.maintenanceMu.Lock()
	items := append([]BrowserExtension{}, a.config.Browser.Extensions...)
	a.maintenanceMu.Unlock()

	out := make([]BrowserExtension, 0, len(items))
	for _, item := range items {
		out = append(out, a.hydrateBrowserExtension(item))
	}
	return out
}

func (a *App) BrowserExtensionImportLocalPackage() (*BrowserExtension, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("应用上下文未初始化")
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择插件安装包",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "浏览器插件包 (*.zip;*.crx)", Pattern: "*.zip;*.crx"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("打开文件对话框失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	return a.BrowserExtensionImportPath(path)
}

func (a *App) BrowserExtensionImportLocalDirectory() (*BrowserExtension, error) {
	if a.ctx == nil {
		return nil, fmt.Errorf("应用上下文未初始化")
	}

	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择插件目录",
	})
	if err != nil {
		return nil, fmt.Errorf("打开目录对话框失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	return a.BrowserExtensionImportPath(path)
}

func (a *App) BrowserExtensionImportChromeWebStoreURL(rawURL string) (*BrowserExtension, error) {
	if a == nil || a.config == nil {
		return nil, fmt.Errorf("应用配置未初始化")
	}

	extensionID, err := parseChromeWebStoreExtensionID(rawURL)
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "ant-browser-webstore-extension-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	packagePath := filepath.Join(tempDir, extensionID+".crx")
	if err := downloadChromeWebStoreCRX(extensionID, packagePath); err != nil {
		return nil, err
	}

	unpackDir := filepath.Join(tempDir, "unpacked")
	if err := extractBrowserExtensionPackage(packagePath, unpackDir); err != nil {
		return nil, err
	}

	extensionRoot, err := findBrowserExtensionRoot(unpackDir)
	if err != nil {
		return nil, err
	}

	sourceURL := normalizeChromeWebStoreSourceURL(rawURL, extensionID)
	return a.importBrowserExtensionFromRoot(extensionRoot, sourceURL, "chrome_web_store")
}

func (a *App) BrowserExtensionImportPath(sourcePath string) (*BrowserExtension, error) {
	if a == nil || a.config == nil {
		return nil, fmt.Errorf("应用配置未初始化")
	}

	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return nil, fmt.Errorf("插件路径不能为空")
	}

	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("解析插件路径失败: %w", err)
	}
	info, err := os.Stat(absSource)
	if err != nil {
		return nil, fmt.Errorf("读取插件路径失败: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "ant-browser-extension-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	sourceType := "directory"
	searchRoot := absSource
	if !info.IsDir() {
		sourceType = extensionPackageSourceType(absSource)
		if err := extractBrowserExtensionPackage(absSource, tempDir); err != nil {
			return nil, err
		}
		searchRoot = tempDir
	}

	extensionRoot, err := findBrowserExtensionRoot(searchRoot)
	if err != nil {
		return nil, err
	}

	return a.importBrowserExtensionFromRoot(extensionRoot, absSource, sourceType)
}

func (a *App) importBrowserExtensionFromRoot(extensionRoot string, sourcePath string, sourceType string) (*BrowserExtension, error) {
	log := logger.New("BrowserExtension")

	manifest, err := readChromeExtensionManifest(extensionRoot)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	record := BrowserExtension{
		ExtensionId:     newBrowserExtensionID(),
		Name:            manifest.Name,
		Description:     manifest.Description,
		Version:         manifest.Version,
		ManifestVersion: manifest.ManifestVersion,
		SourcePath:      strings.TrimSpace(sourcePath),
		SourceType:      sourceType,
		Enabled:         true,
		ImportedAt:      now,
		UpdatedAt:       now,
	}
	if record.Name == "" {
		record.Name = strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	}

	targetDir := filepath.Join(a.browserExtensionStorageRoot(), record.ExtensionId)
	if err := os.RemoveAll(targetDir); err != nil {
		return nil, fmt.Errorf("清理插件安装目录失败: %w", err)
	}
	if err := copyBrowserExtensionDir(extensionRoot, targetDir); err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, fmt.Errorf("复制插件文件失败: %w", err)
	}
	record.InstallPath = a.pathRelativeToAppState(targetDir)

	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	a.config.Browser.Extensions = append(a.config.Browser.Extensions, record)
	if err := a.saveBrowserExtensionsLocked(); err != nil {
		_ = os.RemoveAll(targetDir)
		log.Error("插件配置保存失败", logger.F("error", err.Error()))
		return nil, err
	}

	hydrated := a.hydrateBrowserExtension(record)
	log.Info("插件已导入",
		logger.F("extension_id", hydrated.ExtensionId),
		logger.F("name", hydrated.Name),
		logger.F("version", hydrated.Version),
		logger.F("install_path", hydrated.InstallPath),
	)
	return &hydrated, nil
}

func (a *App) BrowserExtensionSetEnabled(extensionId string, enabled bool) (*BrowserExtension, error) {
	if a == nil || a.config == nil {
		return nil, fmt.Errorf("应用配置未初始化")
	}
	extensionId = strings.TrimSpace(extensionId)
	if extensionId == "" {
		return nil, fmt.Errorf("插件 ID 不能为空")
	}

	a.maintenanceMu.Lock()
	defer a.maintenanceMu.Unlock()

	index := findBrowserExtensionIndex(a.config.Browser.Extensions, extensionId)
	if index < 0 {
		return nil, fmt.Errorf("未找到插件: %s", extensionId)
	}

	a.config.Browser.Extensions[index].Enabled = enabled
	a.config.Browser.Extensions[index].UpdatedAt = time.Now().Format(time.RFC3339)
	if err := a.saveBrowserExtensionsLocked(); err != nil {
		return nil, err
	}

	hydrated := a.hydrateBrowserExtension(a.config.Browser.Extensions[index])
	return &hydrated, nil
}

func (a *App) BrowserExtensionDelete(extensionId string, removeFiles bool) error {
	if a == nil || a.config == nil {
		return fmt.Errorf("应用配置未初始化")
	}
	extensionId = strings.TrimSpace(extensionId)
	if extensionId == "" {
		return fmt.Errorf("插件 ID 不能为空")
	}

	a.maintenanceMu.Lock()
	index := findBrowserExtensionIndex(a.config.Browser.Extensions, extensionId)
	if index < 0 {
		a.maintenanceMu.Unlock()
		return fmt.Errorf("未找到插件: %s", extensionId)
	}

	item := a.config.Browser.Extensions[index]
	next := append([]BrowserExtension{}, a.config.Browser.Extensions[:index]...)
	next = append(next, a.config.Browser.Extensions[index+1:]...)
	a.config.Browser.Extensions = next
	err := a.saveBrowserExtensionsLocked()
	a.maintenanceMu.Unlock()
	if err != nil {
		return err
	}

	if removeFiles {
		absPath := a.resolveBrowserExtensionInstallPath(item.InstallPath)
		if isBrowserExtensionManagedPath(a.browserExtensionStorageRoot(), absPath) {
			if err := os.RemoveAll(absPath); err != nil {
				return fmt.Errorf("删除插件文件失败: %w", err)
			}
		}
	}
	return nil
}

func (a *App) BrowserExtensionOpen(extensionId string) error {
	if a == nil || a.config == nil {
		return fmt.Errorf("应用配置未初始化")
	}
	extensionId = strings.TrimSpace(extensionId)
	if extensionId == "" {
		return fmt.Errorf("插件 ID 不能为空")
	}

	a.maintenanceMu.Lock()
	index := findBrowserExtensionIndex(a.config.Browser.Extensions, extensionId)
	if index < 0 {
		a.maintenanceMu.Unlock()
		return fmt.Errorf("未找到插件: %s", extensionId)
	}
	item := a.config.Browser.Extensions[index]
	a.maintenanceMu.Unlock()

	absPath := a.resolveBrowserExtensionInstallPath(item.InstallPath)
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("插件目录不可访问: %w", err)
	}
	return openPathInFileManager(absPath)
}

func (a *App) browserEnabledExtensionDirs() []string {
	if a == nil || a.config == nil {
		return nil
	}

	a.maintenanceMu.Lock()
	items := append([]BrowserExtension{}, a.config.Browser.Extensions...)
	a.maintenanceMu.Unlock()

	seen := map[string]struct{}{}
	dirs := make([]string, 0, len(items))
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		absPath := a.resolveBrowserExtensionInstallPath(item.InstallPath)
		if strings.Contains(absPath, ",") || !browserExtensionPathExists(absPath) {
			continue
		}
		key := strings.ToLower(filepath.Clean(absPath))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dirs = append(dirs, absPath)
	}
	return dirs
}

func appendBrowserExtensionLaunchArgs(args []string, extensionDirs []string) []string {
	extensionDirs = normalizeBrowserExtensionDirs(extensionDirs)
	if len(extensionDirs) == 0 {
		return args
	}
	return append(args, "--load-extension="+strings.Join(extensionDirs, ","))
}

func normalizeBrowserExtensionDirs(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		path := strings.TrimSpace(item)
		if path == "" || strings.Contains(path, ",") {
			continue
		}
		clean := filepath.Clean(path)
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func (a *App) hydrateBrowserExtension(item BrowserExtension) BrowserExtension {
	absPath := a.resolveBrowserExtensionInstallPath(item.InstallPath)
	item.PathExists = browserExtensionPathExists(absPath)
	if item.PathExists {
		item.IconDataURL = loadBrowserExtensionIconDataURL(absPath)
	}
	return item
}

func (a *App) browserExtensionStorageRoot() string {
	return filepath.Join(a.appDataDir(), "extensions")
}

func (a *App) resolveBrowserExtensionInstallPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return a.resolveAppPath(path)
}

func (a *App) pathRelativeToAppState(absPath string) string {
	clean := filepath.Clean(absPath)
	for _, root := range []string{a.appStateRootAbs(), a.appRootAbs()} {
		if root == "" {
			continue
		}
		if rel, err := filepath.Rel(root, clean); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return clean
}

func (a *App) saveBrowserExtensionsLocked() error {
	if a == nil || a.config == nil {
		return fmt.Errorf("应用配置未初始化")
	}
	return a.config.Save(a.resolveAppPath("config.yaml"))
}

func findBrowserExtensionIndex(items []BrowserExtension, extensionId string) int {
	for i, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.ExtensionId), extensionId) {
			return i
		}
	}
	return -1
}

func browserExtensionPathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(path, "manifest.json"))
	return err == nil && !info.IsDir()
}

func isBrowserExtensionManagedPath(root string, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

func extensionPackageSourceType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".crx":
		return "crx"
	case ".zip":
		return "zip"
	default:
		return "package"
	}
}

func parseChromeWebStoreExtensionID(rawURL string) (string, error) {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return "", fmt.Errorf("Chrome 应用商店链接不能为空")
	}
	value = strings.Trim(value, `"'`)
	if isChromeExtensionID(value) {
		return strings.ToLower(value), nil
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("Chrome 应用商店链接格式不正确")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "chromewebstore.google.com" && host != "chrome.google.com" {
		return "", fmt.Errorf("请输入 Chrome 应用商店插件链接")
	}

	if id := parsed.Query().Get("id"); isChromeExtensionID(id) {
		return strings.ToLower(id), nil
	}

	for _, segment := range strings.Split(parsed.EscapedPath(), "/") {
		if decoded, err := url.PathUnescape(segment); err == nil && isChromeExtensionID(decoded) {
			return strings.ToLower(decoded), nil
		}
	}

	return "", fmt.Errorf("未从链接中识别到插件 ID")
}

func isChromeExtensionID(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if len(value) != 32 {
		return false
	}
	for _, r := range value {
		if r < 'a' || r > 'p' {
			return false
		}
	}
	return true
}

func normalizeChromeWebStoreSourceURL(rawURL string, extensionID string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL != "" && !isChromeExtensionID(rawURL) {
		return rawURL
	}
	return "https://chromewebstore.google.com/detail/" + strings.ToLower(strings.TrimSpace(extensionID))
}

func downloadChromeWebStoreCRX(extensionID string, targetPath string) error {
	extensionID = strings.ToLower(strings.TrimSpace(extensionID))
	if !isChromeExtensionID(extensionID) {
		return fmt.Errorf("插件 ID 无效")
	}

	downloadURL := buildChromeWebStoreCRXDownloadURL(extensionID)
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}
	req.Header.Set("User-Agent", chromeWebStoreDownloadUserAgent())

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载 Chrome 应用商店插件失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("下载 Chrome 应用商店插件失败: HTTP %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("创建插件安装包失败: %w", err)
	}

	limited := io.LimitReader(resp.Body, 300*1024*1024+1)
	written, copyErr := io.Copy(out, limited)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("保存插件安装包失败: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("保存插件安装包失败: %w", closeErr)
	}
	if written > 300*1024*1024 {
		_ = os.Remove(targetPath)
		return fmt.Errorf("插件安装包超过 300MB，已取消导入")
	}
	if written == 0 {
		return fmt.Errorf("下载到的插件安装包为空")
	}

	return nil
}

func buildChromeWebStoreCRXDownloadURL(extensionID string) string {
	values := url.Values{}
	values.Set("response", "redirect")
	values.Set("prodversion", chromeWebStoreDownloadProdVersion)
	values.Set("acceptformat", "crx2,crx3")
	values.Set("x", "id="+extensionID+"&installsource=ondemand&uc")
	return "https://clients2.google.com/service/update2/crx?" + values.Encode()
}

func chromeWebStoreDownloadUserAgent() string {
	switch goruntime.GOOS {
	case "darwin":
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X 13_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	case "linux":
		return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	default:
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	}
}

func extractBrowserExtensionPackage(path string, targetDir string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取插件安装包失败: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("插件安装包为空")
	}

	if bytes.HasPrefix(data, []byte("PK\x03\x04")) || bytes.HasPrefix(data, []byte("PK\x05\x06")) || bytes.HasPrefix(data, []byte("PK\x07\x08")) {
		return extractBrowserExtensionZipBytes(data, targetDir)
	}
	if bytes.HasPrefix(data, []byte("Cr24")) {
		zipPayload, err := crxZipPayload(data)
		if err != nil {
			return err
		}
		return extractBrowserExtensionZipBytes(zipPayload, targetDir)
	}
	return fmt.Errorf("不支持的插件安装包格式，请选择 ZIP、CRX 或插件目录")
}

func crxZipPayload(data []byte) ([]byte, error) {
	if len(data) < 12 || !bytes.HasPrefix(data, []byte("Cr24")) {
		return nil, fmt.Errorf("无效的 CRX 文件")
	}

	version := binary.LittleEndian.Uint32(data[4:8])
	offset := 0
	switch version {
	case 2:
		if len(data) < 16 {
			return nil, fmt.Errorf("无效的 CRX2 文件头")
		}
		pubKeyLen := int(binary.LittleEndian.Uint32(data[8:12]))
		signatureLen := int(binary.LittleEndian.Uint32(data[12:16]))
		offset = 16 + pubKeyLen + signatureLen
	case 3:
		headerLen := int(binary.LittleEndian.Uint32(data[8:12]))
		offset = 12 + headerLen
	default:
		return nil, fmt.Errorf("暂不支持 CRX%d 插件包", version)
	}

	if offset <= 0 || offset >= len(data) {
		return nil, fmt.Errorf("CRX 文件头损坏")
	}
	payload := data[offset:]
	if !bytes.HasPrefix(payload, []byte("PK")) {
		return nil, fmt.Errorf("CRX 内部 ZIP 数据损坏")
	}
	return payload, nil
}

func extractBrowserExtensionZipBytes(data []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("解析 ZIP 插件包失败: %w", err)
	}
	return extractBrowserExtensionZipEntries(reader.File, targetDir)
}

func extractBrowserExtensionZipEntries(files []*zip.File, targetDir string) error {
	cleanTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cleanTarget, 0755); err != nil {
		return err
	}

	for _, file := range files {
		rawName := strings.ReplaceAll(file.Name, "\\", "/")
		if strings.HasPrefix(rawName, "/") || filepath.IsAbs(rawName) {
			return fmt.Errorf("插件包包含非法路径: %s", file.Name)
		}
		name := filepath.Clean(rawName)
		if name == "." || name == "" {
			continue
		}
		if filepath.IsAbs(name) || strings.HasPrefix(name, "..") {
			return fmt.Errorf("插件包包含非法路径: %s", file.Name)
		}

		targetPath := filepath.Join(cleanTarget, name)
		if !isPathInsideOrEqual(cleanTarget, targetPath) {
			return fmt.Errorf("插件包包含越界路径: %s", file.Name)
		}

		info := file.FileInfo()
		if info.IsDir() {
			if err := os.MkdirAll(targetPath, info.Mode().Perm()|0700); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			_ = in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		_ = in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func findBrowserExtensionRoot(baseDir string) (string, error) {
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}

	type candidate struct {
		path  string
		depth int
	}
	candidates := []candidate{}
	err = filepath.WalkDir(baseDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != baseDir {
				rel, err := filepath.Rel(baseDir, path)
				if err == nil && pathDepth(rel) > 4 {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.EqualFold(entry.Name(), "manifest.json") {
			return nil
		}
		dir := filepath.Dir(path)
		rel, _ := filepath.Rel(baseDir, dir)
		candidates = append(candidates, candidate{path: dir, depth: pathDepth(rel)})
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("未找到 manifest.json，请选择有效的浏览器插件目录或安装包")
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].depth == candidates[j].depth {
			return candidates[i].path < candidates[j].path
		}
		return candidates[i].depth < candidates[j].depth
	})
	return candidates[0].path, nil
}

func readChromeExtensionManifest(root string) (chromeExtensionManifest, error) {
	data, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return chromeExtensionManifest{}, fmt.Errorf("读取 manifest.json 失败: %w", err)
	}

	var manifest chromeExtensionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return chromeExtensionManifest{}, fmt.Errorf("解析 manifest.json 失败: %w", err)
	}
	if manifest.ManifestVersion <= 0 {
		return chromeExtensionManifest{}, fmt.Errorf("manifest.json 缺少 manifest_version")
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return chromeExtensionManifest{}, fmt.Errorf("manifest.json 缺少 version")
	}

	manifest.Name = strings.TrimSpace(resolveChromeLocaleMessage(root, manifest.DefaultLocale, manifest.Name))
	manifest.Description = strings.TrimSpace(resolveChromeLocaleMessage(root, manifest.DefaultLocale, manifest.Description))
	manifest.Version = strings.TrimSpace(manifest.Version)
	if manifest.Name == "" {
		manifest.Name = "未命名插件"
	}
	return manifest, nil
}

func resolveChromeLocaleMessage(root string, defaultLocale string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "__MSG_") || !strings.HasSuffix(value, "__") {
		return value
	}
	key := strings.TrimSuffix(strings.TrimPrefix(value, "__MSG_"), "__")
	if key == "" {
		return value
	}

	for _, locale := range chromeLocaleCandidates(defaultLocale) {
		messages, err := readChromeLocaleMessages(root, locale)
		if err != nil {
			continue
		}
		if msg, ok := messages[key]; ok && strings.TrimSpace(msg.Message) != "" {
			return msg.Message
		}
		for candidateKey, msg := range messages {
			if strings.EqualFold(candidateKey, key) && strings.TrimSpace(msg.Message) != "" {
				return msg.Message
			}
		}
	}
	return value
}

func chromeLocaleCandidates(defaultLocale string) []string {
	candidates := []string{}
	add := func(locale string) {
		locale = strings.TrimSpace(locale)
		if locale == "" {
			return
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, locale) {
				return
			}
		}
		candidates = append(candidates, locale)
	}

	add(defaultLocale)
	add("zh_CN")
	add("zh")
	add("en")
	add("en_US")
	return candidates
}

func readChromeLocaleMessages(root string, locale string) (map[string]chromeLocaleMessage, error) {
	data, err := os.ReadFile(filepath.Join(root, "_locales", locale, "messages.json"))
	if err != nil {
		return nil, err
	}
	var messages map[string]chromeLocaleMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func loadBrowserExtensionIconDataURL(root string) string {
	manifest, err := readChromeExtensionManifest(root)
	if err != nil {
		return ""
	}
	iconRel := chooseBrowserExtensionIcon(manifest.Icons)
	if iconRel == "" {
		return ""
	}
	iconPath, ok := safeJoinBrowserExtensionPath(root, iconRel)
	if !ok {
		return ""
	}
	info, err := os.Stat(iconPath)
	if err != nil || info.IsDir() || info.Size() <= 0 || info.Size() > 512*1024 {
		return ""
	}
	data, err := os.ReadFile(iconPath)
	if err != nil {
		return ""
	}

	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(iconPath)))
	if mimeType == "" {
		mimeType = "image/png"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func chooseBrowserExtensionIcon(icons map[string]string) string {
	if len(icons) == 0 {
		return ""
	}

	type iconCandidate struct {
		size int
		path string
	}
	candidates := make([]iconCandidate, 0, len(icons))
	for sizeText, iconPath := range icons {
		iconPath = strings.TrimSpace(iconPath)
		if iconPath == "" {
			continue
		}
		size, _ := strconv.Atoi(strings.TrimSpace(sizeText))
		candidates = append(candidates, iconCandidate{size: size, path: iconPath})
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].size == candidates[j].size {
			return candidates[i].path < candidates[j].path
		}
		return candidates[i].size > candidates[j].size
	})
	return candidates[0].path
}

func copyBrowserExtensionDir(src string, dst string) error {
	src, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	dst, err = filepath.Abs(dst)
	if err != nil {
		return err
	}
	if !isPathInsideOrEqual(filepath.Dir(dst), dst) {
		return fmt.Errorf("目标目录无效")
	}

	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := dst
		if rel != "." {
			target = filepath.Join(dst, rel)
		}
		if !isPathInsideOrEqual(dst, target) {
			return fmt.Errorf("插件目录包含越界路径: %s", path)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm()|0700)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return copyBrowserExtensionFile(path, target, info.Mode().Perm())
	})
}

func copyBrowserExtensionFile(src string, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func safeJoinBrowserExtensionPath(root string, rel string) (string, bool) {
	rel = strings.TrimSpace(rel)
	if rel == "" || filepath.IsAbs(rel) {
		return "", false
	}
	cleanRoot := filepath.Clean(root)
	fullPath := filepath.Join(cleanRoot, rel)
	if !isPathInsideOrEqual(cleanRoot, fullPath) {
		return "", false
	}
	return fullPath, true
}

func isPathInsideOrEqual(root string, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == path {
		return true
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

func pathDepth(rel string) int {
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" {
		return 0
	}
	return strings.Count(filepath.ToSlash(rel), "/") + 1
}

func newBrowserExtensionID() string {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "ext-" + time.Now().Format("20060102150405")
	}
	return fmt.Sprintf("ext-%s-%x", time.Now().Format("20060102150405"), suffix)
}
