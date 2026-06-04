package backend

import "testing"

func TestBuildClashProxyImportPlanKeepsIDsAndRemovesMissing(t *testing.T) {
	content := `
proxies:
  - name: A
    type: http
    server: 127.0.0.1
    port: 8080
  - name: B
    type: socks5
    server: 127.0.0.1
    port: 1080
`
	_, payload, err := normalizeClashSubscriptionContent([]byte(content))
	if err != nil {
		t.Fatalf("normalize content: %v", err)
	}

	existing := []BrowserProxy{
		{ProxyId: "old-a", ProxyName: "Sub-A", ProxyConfig: "http://127.0.0.1:8080", SourceID: "src-1", SourceURL: "https://sub.invalid/a", SortOrder: 3},
		{ProxyId: "old-c", ProxyName: "Sub-C", ProxyConfig: "http://127.0.0.1:9090", SourceID: "src-1", SourceURL: "https://sub.invalid/a", SortOrder: 4},
	}
	imported, report, removed := buildClashProxyImportPlan(clashProxyImportPlanInput{
		Existing:               existing,
		OldSourceProxies:       existing,
		SourceID:               "src-1",
		SourceURL:              "https://sub.invalid/a",
		Payload:                payload,
		NamePrefix:             "Sub",
		GroupName:              "订阅",
		SourceAutoRefresh:      true,
		SourceRefreshIntervalM: 60,
	})

	if len(imported) != 2 {
		t.Fatalf("imported len = %d, want 2", len(imported))
	}
	if imported[0].ProxyId != "old-a" {
		t.Fatalf("first proxy id = %q, want old-a", imported[0].ProxyId)
	}
	if imported[0].ProxyConfig != "http://127.0.0.1:8080" {
		t.Fatalf("http clash node should convert to direct browser proxy, got %q", imported[0].ProxyConfig)
	}
	if imported[1].ProxyConfig != "socks5://127.0.0.1:1080" {
		t.Fatalf("socks clash node should convert to direct browser proxy, got %q", imported[1].ProxyConfig)
	}
	if len(removed) != 1 || removed[0] != "old-c" {
		t.Fatalf("removed = %#v, want old-c", removed)
	}
	if report.Added != 1 || report.Updated != 1 || report.Removed != 1 || report.Failed != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestBuildClashProxyImportPlanSkipsUnsupported(t *testing.T) {
	content := `
proxies:
  - name: Bad
    type: ssr
    server: example.invalid
    port: 8388
`
	_, payload, err := normalizeClashSubscriptionContent([]byte(content))
	if err != nil {
		t.Fatalf("normalize content: %v", err)
	}

	imported, report, removed := buildClashProxyImportPlan(clashProxyImportPlanInput{
		Payload: payload,
	})

	if len(imported) != 0 {
		t.Fatalf("imported len = %d, want 0", len(imported))
	}
	if len(removed) != 0 {
		t.Fatalf("removed = %#v, want none", removed)
	}
	if report.Failed != 1 || len(report.UnsupportedProxyNames) != 1 || report.UnsupportedProxyNames[0] != "Bad" {
		t.Fatalf("unexpected report: %+v", report)
	}
}
