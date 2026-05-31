package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func TestBrowserExtensionImportPathCopiesExtensionAndFeedsLaunchArgs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sourceDir := filepath.Join(root, "source-extension")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"manifest_version":3,"name":"Demo Extension","version":"1.2.3","description":"Imported in tests"}`
	if err := os.WriteFile(filepath.Join(sourceDir, "manifest.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	app := NewApp(root)
	app.config = config.DefaultConfig()

	extension, err := app.BrowserExtensionImportPath(sourceDir)
	if err != nil {
		t.Fatalf("BrowserExtensionImportPath returned error: %v", err)
	}
	if extension == nil {
		t.Fatal("expected imported extension")
	}
	if extension.Name != "Demo Extension" || extension.Version != "1.2.3" {
		t.Fatalf("unexpected extension metadata: %+v", extension)
	}
	if !extension.Enabled {
		t.Fatal("imported extension should be enabled by default")
	}
	if !strings.HasPrefix(filepath.ToSlash(extension.InstallPath), "data/extensions/") {
		t.Fatalf("expected relative managed install path, got %q", extension.InstallPath)
	}
	if !extension.PathExists {
		t.Fatal("hydrated extension should report path exists")
	}

	installDir := app.resolveBrowserExtensionInstallPath(extension.InstallPath)
	if _, err := os.Stat(filepath.Join(installDir, "manifest.json")); err != nil {
		t.Fatalf("expected copied manifest in managed directory: %v", err)
	}

	extensionDirs := app.browserEnabledExtensionDirs()
	if len(extensionDirs) != 1 || extensionDirs[0] != installDir {
		t.Fatalf("unexpected enabled extension dirs: %v", extensionDirs)
	}

	args := buildBrowserLaunchArgs(
		&BrowserProfile{ProfileId: "profile-1", FingerprintArgs: []string{"--fingerprint=42"}},
		filepath.Join(root, "profile-data"),
		9222,
		"",
		extensionDirs,
		nil,
		nil,
		nil,
		nil,
		true,
		false,
	)
	want := "--load-extension=" + installDir
	found := false
	for _, arg := range args {
		if arg == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected launch args to contain %q, got %v", want, args)
	}
}

func TestParseChromeWebStoreExtensionID(t *testing.T) {
	t.Parallel()

	const id = "lapnciffpekdengooeolaienkeoilfeo"
	cases := []string{
		id,
		"https://chromewebstore.google.com/detail/" + id,
		"https://chromewebstore.google.com/detail/demo-name/" + id + "?hl=zh-CN",
		"https://chrome.google.com/webstore/detail/demo-name/" + id,
	}

	for _, input := range cases {
		got, err := parseChromeWebStoreExtensionID(input)
		if err != nil {
			t.Fatalf("parseChromeWebStoreExtensionID(%q) returned error: %v", input, err)
		}
		if got != id {
			t.Fatalf("parseChromeWebStoreExtensionID(%q)=%q, want %q", input, got, id)
		}
	}
}

func TestParseChromeWebStoreExtensionIDRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"https://example.com/detail/lapnciffpekdengooeolaienkeoilfeo",
		"https://chromewebstore.google.com/detail/not-an-extension-id",
		"abcdefghijklmnopqrstuvwxyzzzzzzz",
	}

	for _, input := range cases {
		if got, err := parseChromeWebStoreExtensionID(input); err == nil {
			t.Fatalf("parseChromeWebStoreExtensionID(%q)=%q, want error", input, got)
		}
	}
}
