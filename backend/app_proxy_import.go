package backend

import (
	proxycore "ant-chrome/backend/internal/proxy"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	maxClashSubscriptionBytes = 8 * 1024 * 1024
	clashSubscriptionTimeout  = 25 * time.Second
)

// BrowserProxyFetchClashByURL 拉取 Clash 订阅 URL，并返回可直接导入的 YAML 文本与建议配置。
func (a *App) BrowserProxyFetchClashByURL(rawURL string) (map[string]interface{}, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("订阅 URL 不能为空")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return nil, fmt.Errorf("URL 格式无效")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsedURL.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("仅支持 http/https URL")
	}

	req, err := http.NewRequest(http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "clash-verge/2.0 ant-chrome/1.0")
	req.Header.Set("Accept", "application/yaml,text/yaml,text/plain,*/*")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{
		Timeout: clashSubscriptionTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("拉取订阅失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("拉取订阅失败: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxClashSubscriptionBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取订阅内容失败: %w", err)
	}
	if len(body) > maxClashSubscriptionBytes {
		return nil, fmt.Errorf("订阅内容过大（超过 8MB）")
	}

	content, payload, err := normalizeClashSubscriptionContent(body)
	if err != nil {
		return nil, err
	}

	proxyCount := clashProxyCount(payload)
	if proxyCount <= 0 {
		return nil, fmt.Errorf("未检测到可导入的 proxies 节点")
	}

	dnsYAML := extractClashDNSYAML(payload)
	suggestedGroup := suggestClashGroupName(payload, parsedURL.Hostname())

	return map[string]interface{}{
		"url":            parsedURL.String(),
		"content":        content,
		"proxyCount":     proxyCount,
		"dnsServers":     dnsYAML,
		"suggestedGroup": suggestedGroup,
	}, nil
}

// BrowserProxyImportClash 将 Clash YAML 文本导入代理池。
func (a *App) BrowserProxyImportClash(input ProxyImportClashRequest) (*ProxyImportReport, error) {
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, fmt.Errorf("Clash 内容不能为空")
	}

	normalizedContent, payload, err := normalizeClashSubscriptionContent([]byte(content))
	if err != nil {
		return nil, err
	}

	sourceURL := strings.TrimSpace(input.SourceURL)
	sourceID := strings.TrimSpace(input.SourceID)
	namePrefix := strings.TrimSpace(input.NamePrefix)
	if sourceURL != "" && sourceID == "" {
		sourceID = buildStableProxySourceID(sourceURL, namePrefix)
	}

	existing := a.getLatestProxies()
	oldSourceProxies := filterProxiesBySourceID(existing, sourceID)
	imported, report, removedIDs := buildClashProxyImportPlan(clashProxyImportPlanInput{
		Existing:               existing,
		OldSourceProxies:       oldSourceProxies,
		SourceID:               sourceID,
		SourceURL:              sourceURL,
		Content:                normalizedContent,
		Payload:                payload,
		NamePrefix:             namePrefix,
		GroupName:              strings.TrimSpace(input.GroupName),
		DnsServers:             strings.TrimSpace(input.DnsServers),
		SourceAutoRefresh:      input.SourceAutoRefresh,
		SourceRefreshIntervalM: normalizeSourceRefreshInterval(input.SourceRefreshIntervalM, input.SourceAutoRefresh),
		KeepRemoved:            input.KeepRemoved,
		KeepNames:              input.KeepNames,
		SkipNames:              input.SkipNames,
	})
	if report.Failed > 0 && len(imported) == 0 {
		return report, fmt.Errorf("未导入可用代理节点")
	}
	if len(imported) == 0 {
		return report, fmt.Errorf("未解析到可导入代理")
	}

	if err := a.applyProxyImportPlan(existing, imported, removedIDs, report); err != nil {
		return report, err
	}
	return report, nil
}

// BrowserProxyRefreshSource 刷新指定 Clash URL 订阅来源。
func (a *App) BrowserProxyRefreshSource(input ProxySourceRefreshRequest) (*ProxyImportReport, error) {
	sourceID := strings.TrimSpace(input.SourceID)
	if sourceID == "" {
		return nil, fmt.Errorf("订阅来源 ID 不能为空")
	}

	sources := a.BrowserProxyListSources()
	var source *ProxySourceSummary
	for i := range sources {
		if strings.EqualFold(sources[i].SourceID, sourceID) {
			source = &sources[i]
			break
		}
	}
	if source == nil {
		return nil, fmt.Errorf("订阅来源不存在: %s", sourceID)
	}
	if strings.TrimSpace(source.SourceURL) == "" {
		return nil, fmt.Errorf("订阅来源缺少 URL")
	}

	fetched, err := a.BrowserProxyFetchClashByURL(source.SourceURL)
	if err != nil {
		return nil, err
	}
	content, _ := fetched["content"].(string)
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("订阅内容为空")
	}

	autoRefresh := source.SourceAutoRefresh
	if input.SourceAutoRefresh != nil {
		autoRefresh = *input.SourceAutoRefresh
	}
	interval := source.SourceRefreshIntervalM
	if input.SourceRefreshIntervalM > 0 {
		interval = input.SourceRefreshIntervalM
	}

	return a.BrowserProxyImportClash(ProxyImportClashRequest{
		SourceID:               source.SourceID,
		SourceURL:              source.SourceURL,
		Content:                content,
		NamePrefix:             source.SourceNamePrefix,
		GroupName:              source.GroupName,
		DnsServers:             source.DnsServers,
		SourceAutoRefresh:      autoRefresh,
		SourceRefreshIntervalM: interval,
		KeepRemoved:            input.KeepRemoved,
		KeepNames:              input.KeepNames,
		SkipNames:              input.SkipNames,
	})
}

// BrowserProxyListSources 返回代理池中的 URL 订阅来源。
func (a *App) BrowserProxyListSources() []ProxySourceSummary {
	proxies := a.getLatestProxies()
	sourceMap := make(map[string]*ProxySourceSummary)
	for _, item := range proxies {
		sourceID := strings.TrimSpace(item.SourceID)
		sourceURL := strings.TrimSpace(item.SourceURL)
		if sourceID == "" || sourceURL == "" {
			continue
		}
		summary := sourceMap[sourceID]
		if summary == nil {
			summary = &ProxySourceSummary{
				SourceID:               sourceID,
				SourceURL:              sourceURL,
				SourceNamePrefix:       strings.TrimSpace(item.SourceNamePrefix),
				GroupName:              strings.TrimSpace(item.GroupName),
				DnsServers:             strings.TrimSpace(item.DnsServers),
				SourceAutoRefresh:      item.SourceAutoRefresh,
				SourceRefreshIntervalM: item.SourceRefreshIntervalM,
				SourceLastRefreshAt:    strings.TrimSpace(item.SourceLastRefreshAt),
			}
			sourceMap[sourceID] = summary
		}
		summary.ProxyCount++
		if strings.TrimSpace(item.SourceLastRefreshAt) > summary.SourceLastRefreshAt {
			summary.SourceLastRefreshAt = strings.TrimSpace(item.SourceLastRefreshAt)
		}
	}

	list := make([]ProxySourceSummary, 0, len(sourceMap))
	for _, item := range sourceMap {
		list = append(list, *item)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].SourceURL == list[j].SourceURL {
			return list[i].SourceID < list[j].SourceID
		}
		return list[i].SourceURL < list[j].SourceURL
	})
	return list
}

func normalizeClashSubscriptionContent(body []byte) (string, interface{}, error) {
	baseText := strings.TrimSpace(strings.ReplaceAll(string(body), "\r\n", "\n"))
	if baseText == "" {
		return "", nil, fmt.Errorf("订阅内容为空")
	}

	tryTexts := make([]string, 0, 4)
	tryTexts = append(tryTexts, baseText)

	if unescaped, err := url.QueryUnescape(baseText); err == nil {
		unescaped = strings.TrimSpace(strings.ReplaceAll(unescaped, "\r\n", "\n"))
		if unescaped != "" && unescaped != baseText {
			tryTexts = append(tryTexts, unescaped)
		}
	}

	if decoded, ok := decodeBase64Text(baseText); ok {
		tryTexts = append(tryTexts, decoded)
	}

	for _, text := range tryTexts {
		payload, ok := parseClashPayload(text)
		if !ok {
			continue
		}
		if clashProxyCount(payload) > 0 {
			return text, payload, nil
		}
	}

	return "", nil, fmt.Errorf("URL 内容不是有效 Clash YAML（需包含 proxies）")
}

func decodeBase64Text(raw string) (string, bool) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", false
	}
	// 一些订阅会返回 URL-safe base64 或缺少 padding，这里都尝试一遍。
	padded := candidate
	if mod := len(padded) % 4; mod != 0 {
		padded += strings.Repeat("=", 4-mod)
	}

	encoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encoders {
		if data, err := enc.DecodeString(candidate); err == nil {
			decoded := strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n"))
			if decoded != "" {
				return decoded, true
			}
		}
		if data, err := enc.DecodeString(padded); err == nil {
			decoded := strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n"))
			if decoded != "" {
				return decoded, true
			}
		}
	}
	return "", false
}

func parseClashPayload(text string) (interface{}, bool) {
	var payload interface{}
	if err := yaml.Unmarshal([]byte(text), &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func clashProxyCount(payload interface{}) int {
	if m := toStringMap(payload); m != nil {
		if arr, ok := m["proxies"].([]interface{}); ok {
			return len(arr)
		}
		if arr, ok := m["proxy"].([]interface{}); ok {
			return len(arr)
		}
		if arr, ok := m["Proxy"].([]interface{}); ok {
			return len(arr)
		}
	}
	if arr, ok := payload.([]interface{}); ok {
		return len(arr)
	}
	return 0
}

func extractClashDNSYAML(payload interface{}) string {
	m := toStringMap(payload)
	if m == nil {
		return ""
	}
	dnsRaw, exists := m["dns"]
	if !exists || dnsRaw == nil {
		return ""
	}
	data, err := yaml.Marshal(map[string]interface{}{
		"dns": dnsRaw,
	})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func suggestClashGroupName(payload interface{}, fallbackHost string) string {
	fallbackHost = strings.TrimSpace(fallbackHost)
	m := toStringMap(payload)
	if m != nil {
		if groups, ok := m["proxy-groups"].([]interface{}); ok {
			for _, item := range groups {
				if groupMap := toStringMap(item); groupMap != nil {
					if name := strings.TrimSpace(getMapString(groupMap, "name")); name != "" {
						return name
					}
				}
			}
		}
	}
	if strings.HasPrefix(strings.ToLower(fallbackHost), "www.") {
		fallbackHost = fallbackHost[4:]
	}
	return fallbackHost
}

func toStringMap(value interface{}) map[string]interface{} {
	switch m := value.(type) {
	case map[string]interface{}:
		return m
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, v := range m {
			key := fmt.Sprint(k)
			out[key] = v
		}
		return out
	default:
		return nil
	}
}

func getMapString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

type clashProxyImportPlanInput struct {
	Existing               []BrowserProxy
	OldSourceProxies       []BrowserProxy
	SourceID               string
	SourceURL              string
	Content                string
	Payload                interface{}
	NamePrefix             string
	GroupName              string
	DnsServers             string
	SourceAutoRefresh      bool
	SourceRefreshIntervalM int
	KeepRemoved            bool
	KeepNames              []string
	SkipNames              []string
}

func buildClashProxyImportPlan(input clashProxyImportPlanInput) ([]BrowserProxy, *ProxyImportReport, []string) {
	report := &ProxyImportReport{
		SourceID:  input.SourceID,
		SourceURL: input.SourceURL,
	}
	nodes := extractClashProxyNodes(input.Payload)
	if len(nodes) == 0 {
		report.Failed = 1
		report.Errors = append(report.Errors, "未检测到 proxies 节点")
		return nil, report, nil
	}

	keepNameSet := stringSet(input.KeepNames)
	skipNameSet := stringSet(input.SkipNames)
	existingByID := make(map[string]BrowserProxy, len(input.Existing))
	for _, item := range input.Existing {
		if id := strings.TrimSpace(item.ProxyId); id != "" {
			existingByID[id] = item
		}
	}
	pickExistingID := createExistingProxyIDPicker(input.OldSourceProxies)
	baseSort := sourceBaseSortOrder(input.Existing, input.OldSourceProxies)
	refreshedAt := ""
	if input.SourceURL != "" {
		refreshedAt = time.Now().Format(time.RFC3339)
	}

	imported := make([]BrowserProxy, 0, len(nodes))
	for index, node := range nodes {
		proxyName := resolveClashProxyName(node, index, input.NamePrefix)
		if len(keepNameSet) > 0 {
			if _, ok := keepNameSet[strings.ToLower(proxyName)]; !ok {
				report.Skipped++
				report.SkippedProxyNames = append(report.SkippedProxyNames, proxyName)
				continue
			}
		}
		if _, ok := skipNameSet[strings.ToLower(proxyName)]; ok {
			report.Skipped++
			report.SkippedProxyNames = append(report.SkippedProxyNames, proxyName)
			continue
		}

		proxyConfig, supported, supportErr := clashNodeProxyConfig(node)
		if !supported {
			report.Failed++
			report.UnsupportedProxyNames = append(report.UnsupportedProxyNames, proxyName)
			if supportErr != "" {
				report.Errors = append(report.Errors, fmt.Sprintf("%s: %s", proxyName, supportErr))
			}
			continue
		}

		proxyID := pickExistingID(proxyName, proxyConfig)
		if proxyID == "" && input.SourceID != "" {
			proxyID = buildStableProxyID(input.SourceID, proxyName, node)
		}
		if proxyID == "" {
			proxyID = generateUUID()
		}

		item := BrowserProxy{
			ProxyId:                proxyID,
			ProxyName:              proxyName,
			ProxyConfig:            proxyConfig,
			DnsServers:             input.DnsServers,
			GroupName:              input.GroupName,
			SourceID:               input.SourceID,
			SourceURL:              input.SourceURL,
			SourceNamePrefix:       input.NamePrefix,
			SourceAutoRefresh:      input.SourceAutoRefresh && input.SourceURL != "",
			SourceRefreshIntervalM: input.SourceRefreshIntervalM,
			SourceLastRefreshAt:    refreshedAt,
			SortOrder:              baseSort + len(imported),
		}
		if item.SourceURL == "" {
			item.SourceID = ""
			item.SourceNamePrefix = ""
			item.SourceAutoRefresh = false
			item.SourceRefreshIntervalM = 0
			item.SourceLastRefreshAt = ""
		}
		if _, exists := existingByID[item.ProxyId]; exists {
			report.Updated++
		} else {
			report.Added++
		}
		imported = append(imported, item)
	}

	importedIDs := make(map[string]struct{}, len(imported))
	for _, item := range imported {
		importedIDs[item.ProxyId] = struct{}{}
	}
	removedIDs := make([]string, 0)
	if input.SourceID != "" && !input.KeepRemoved {
		for _, old := range input.OldSourceProxies {
			if _, kept := importedIDs[old.ProxyId]; kept {
				continue
			}
			if strings.EqualFold(old.ProxyId, "__direct__") {
				continue
			}
			removedIDs = append(removedIDs, old.ProxyId)
		}
		report.Removed = len(removedIDs)
	}
	report.ImportedProxies = imported
	return imported, report, removedIDs
}

func (a *App) applyProxyImportPlan(existing []BrowserProxy, imported []BrowserProxy, removedIDs []string, report *ProxyImportReport) error {
	if a.browserMgr != nil && a.browserMgr.ProxyDAO != nil {
		if err := a.browserMgr.ProxyDAO.UpsertMany(imported); err != nil {
			return err
		}
		for _, proxyID := range removedIDs {
			if err := a.browserMgr.ProxyDAO.Delete(proxyID); err != nil {
				return err
			}
		}
		if list, err := a.browserMgr.ProxyDAO.List(); err == nil {
			a.config.Browser.Proxies = list
		}
		reconcileReport := a.reconcileProfileProxyBindingsWithReport()
		report.AffectedProfileCount = reconcileReport.ChangedProfileCount + reconcileReport.InvalidProfileCount
		report.ReboundProfileCount = reconcileReport.ReboundProfileCount
		report.InvalidProfileCount = reconcileReport.InvalidProfileCount
		return nil
	}

	merged := mergeProxyImportList(existing, imported, removedIDs)
	if err := a.SaveBrowserProxies(merged); err != nil {
		return err
	}
	reconcileReport := a.reconcileProfileProxyBindingsWithReport()
	report.AffectedProfileCount = reconcileReport.ChangedProfileCount + reconcileReport.InvalidProfileCount
	report.ReboundProfileCount = reconcileReport.ReboundProfileCount
	report.InvalidProfileCount = reconcileReport.InvalidProfileCount
	return nil
}

func mergeProxyImportList(existing []BrowserProxy, imported []BrowserProxy, removedIDs []string) []BrowserProxy {
	removedSet := make(map[string]struct{}, len(removedIDs))
	for _, id := range removedIDs {
		removedSet[strings.TrimSpace(id)] = struct{}{}
	}
	importedByID := make(map[string]BrowserProxy, len(imported))
	for _, item := range imported {
		importedByID[item.ProxyId] = item
	}

	merged := make([]BrowserProxy, 0, len(existing)+len(imported))
	seen := make(map[string]struct{}, len(existing)+len(imported))
	for _, item := range existing {
		id := strings.TrimSpace(item.ProxyId)
		if _, removed := removedSet[id]; removed {
			continue
		}
		if replacement, ok := importedByID[id]; ok {
			merged = append(merged, replacement)
			seen[id] = struct{}{}
			continue
		}
		merged = append(merged, item)
		seen[id] = struct{}{}
	}
	for _, item := range imported {
		if _, ok := seen[item.ProxyId]; ok {
			continue
		}
		merged = append(merged, item)
	}
	return merged
}

func extractClashProxyNodes(payload interface{}) []map[string]interface{} {
	toNodes := func(raw interface{}) []map[string]interface{} {
		arr, ok := raw.([]interface{})
		if !ok {
			return nil
		}
		nodes := make([]map[string]interface{}, 0, len(arr))
		for _, item := range arr {
			if node := toStringMap(item); node != nil {
				nodes = append(nodes, node)
			}
		}
		return nodes
	}

	if m := toStringMap(payload); m != nil {
		for _, key := range []string{"proxies", "proxy", "Proxy"} {
			if nodes := toNodes(m[key]); len(nodes) > 0 {
				return nodes
			}
			if node := toStringMap(m[key]); node != nil {
				return []map[string]interface{}{node}
			}
		}
		if strings.TrimSpace(getMapString(m, "type")) != "" {
			return []map[string]interface{}{m}
		}
	}
	return toNodes(payload)
}

func clashNodeProxyConfig(node map[string]interface{}) (string, bool, string) {
	nodeType := strings.ToLower(strings.TrimSpace(getMapString(node, "type")))
	if nodeType == "socks" {
		nodeType = "socks5"
	}
	if nodeType == "http" || nodeType == "https" || nodeType == "socks5" {
		if direct := buildStandardProxyConfigFromClash(node, nodeType); direct != "" {
			return direct, true, ""
		}
	}

	data, err := yaml.Marshal([]map[string]interface{}{node})
	if err != nil {
		return "", false, fmt.Sprintf("节点序列化失败: %v", err)
	}
	proxyConfig := strings.TrimSpace(string(data))
	if ok, msg := proxycore.ValidateProxyConfig(proxyConfig, nil, ""); !ok {
		return "", false, msg
	}
	return proxyConfig, true, ""
}

func buildStandardProxyConfigFromClash(node map[string]interface{}, scheme string) string {
	host := strings.TrimSpace(getMapString(node, "server"))
	port := getMapIntValue(node, "port")
	if host == "" || port <= 0 || port > 65535 {
		return ""
	}
	username := strings.TrimSpace(getMapString(node, "username"))
	if username == "" {
		username = strings.TrimSpace(getMapString(node, "user"))
	}
	password := getMapString(node, "password")
	hostPort := fmt.Sprintf("%s:%d", host, port)
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		hostPort = fmt.Sprintf("[%s]:%d", host, port)
	}
	if username != "" {
		return fmt.Sprintf("%s://%s@%s", scheme, url.UserPassword(username, password).String(), hostPort)
	}
	return fmt.Sprintf("%s://%s", scheme, hostPort)
}

func resolveClashProxyName(node map[string]interface{}, index int, prefix string) string {
	name := strings.TrimSpace(getMapString(node, "name"))
	if name == "" {
		name = fmt.Sprintf("导入代理 %d", index+1)
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return name
	}
	return prefix + "-" + name
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func filterProxiesBySourceID(proxies []BrowserProxy, sourceID string) []BrowserProxy {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return nil
	}
	out := make([]BrowserProxy, 0)
	for _, item := range proxies {
		if strings.EqualFold(strings.TrimSpace(item.SourceID), sourceID) {
			out = append(out, item)
		}
	}
	return out
}

func sourceBaseSortOrder(existing []BrowserProxy, oldSourceProxies []BrowserProxy) int {
	if len(oldSourceProxies) == 0 {
		return len(existing)
	}
	base := oldSourceProxies[0].SortOrder
	for _, item := range oldSourceProxies[1:] {
		if item.SortOrder < base {
			base = item.SortOrder
		}
	}
	return base
}

func createExistingProxyIDPicker(oldSourceProxies []BrowserProxy) func(string, string) string {
	exactMap := make(map[string][]BrowserProxy)
	nameMap := make(map[string][]BrowserProxy)
	for _, item := range oldSourceProxies {
		exactKey := strings.TrimSpace(item.ProxyName) + "|||" + strings.TrimSpace(item.ProxyConfig)
		exactMap[exactKey] = append(exactMap[exactKey], item)
		nameKey := strings.TrimSpace(item.ProxyName)
		nameMap[nameKey] = append(nameMap[nameKey], item)
	}
	return func(name string, configText string) string {
		exactKey := strings.TrimSpace(name) + "|||" + strings.TrimSpace(configText)
		if list := exactMap[exactKey]; len(list) > 0 {
			item := list[0]
			exactMap[exactKey] = list[1:]
			return item.ProxyId
		}
		nameKey := strings.TrimSpace(name)
		if list := nameMap[nameKey]; len(list) > 0 {
			item := list[0]
			nameMap[nameKey] = list[1:]
			return item.ProxyId
		}
		return ""
	}
}

func buildStableProxySourceID(sourceURL string, namePrefix string) string {
	key := normalizeProxySourceURL(sourceURL) + "|||" + strings.TrimSpace(namePrefix)
	return "src-" + shortSHA1(key, 12)
}

func buildStableProxyID(sourceID string, proxyName string, node map[string]interface{}) string {
	keyParts := []string{
		strings.TrimSpace(sourceID),
		strings.TrimSpace(proxyName),
		strings.ToLower(strings.TrimSpace(getMapString(node, "type"))),
		strings.TrimSpace(getMapString(node, "server")),
		strconv.Itoa(getMapIntValue(node, "port")),
	}
	return "proxy-" + shortSHA1(strings.Join(keyParts, "|||"), 16)
}

func normalizeProxySourceURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.Fragment = ""
	return parsed.String()
}

func shortSHA1(value string, length int) string {
	sum := sha1.Sum([]byte(value))
	encoded := hex.EncodeToString(sum[:])
	if length <= 0 || length >= len(encoded) {
		return encoded
	}
	return encoded[:length]
}

func normalizeSourceRefreshInterval(value int, autoRefresh bool) int {
	if !autoRefresh {
		return 0
	}
	if value <= 0 {
		return 60
	}
	if value < 5 {
		return 5
	}
	if value > 24*60 {
		return 24 * 60
	}
	return value
}

func getMapIntValue(m map[string]interface{}, key string) int {
	if m == nil {
		return 0
	}
	value, ok := m[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(v))
		return parsed
	default:
		parsed, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
		return parsed
	}
}
