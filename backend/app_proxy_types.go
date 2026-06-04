package backend

// ProxyValidationResult 代理验证结果
type ProxyValidationResult struct {
	Supported bool   `json:"supported"`
	ErrorMsg  string `json:"errorMsg"`
}

// ProxyTestResult 代理测试结果
type ProxyTestResult struct {
	ProxyId   string `json:"proxyId"`
	Ok        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error"`
}

// ProxyIPHealthResult 代理出口 IP 健康信息（透传第三方接口结果）
type ProxyIPHealthResult struct {
	ProxyId        string                 `json:"proxyId"`
	Ok             bool                   `json:"ok"`
	Source         string                 `json:"source"`
	Error          string                 `json:"error"`
	IP             string                 `json:"ip"`
	FraudScore     int64                  `json:"fraudScore"`
	IsResidential  bool                   `json:"isResidential"`
	IsBroadcast    bool                   `json:"isBroadcast"`
	Country        string                 `json:"country"`
	Region         string                 `json:"region"`
	City           string                 `json:"city"`
	AsOrganization string                 `json:"asOrganization"`
	RawData        map[string]interface{} `json:"rawData"`
	UpdatedAt      string                 `json:"updatedAt"`
}

// ProxySourceSummary 代理订阅来源摘要。
type ProxySourceSummary struct {
	SourceID               string `json:"sourceId"`
	SourceURL              string `json:"sourceUrl"`
	SourceNamePrefix       string `json:"sourceNamePrefix"`
	GroupName              string `json:"groupName"`
	DnsServers             string `json:"dnsServers"`
	SourceAutoRefresh      bool   `json:"sourceAutoRefresh"`
	SourceRefreshIntervalM int    `json:"sourceRefreshIntervalM"`
	SourceLastRefreshAt    string `json:"sourceLastRefreshAt"`
	ProxyCount             int    `json:"proxyCount"`
}

// ProxyImportClashRequest 导入 Clash 订阅/文本请求。
type ProxyImportClashRequest struct {
	SourceID               string   `json:"sourceId"`
	SourceURL              string   `json:"sourceUrl"`
	Content                string   `json:"content"`
	NamePrefix             string   `json:"namePrefix"`
	GroupName              string   `json:"groupName"`
	DnsServers             string   `json:"dnsServers"`
	SourceAutoRefresh      bool     `json:"sourceAutoRefresh"`
	SourceRefreshIntervalM int      `json:"sourceRefreshIntervalM"`
	KeepRemoved            bool     `json:"keepRemoved"`
	KeepNames              []string `json:"keepNames"`
	SkipNames              []string `json:"skipNames"`
}

// ProxySourceRefreshRequest 刷新订阅来源请求。
type ProxySourceRefreshRequest struct {
	SourceID               string   `json:"sourceId"`
	SourceAutoRefresh      *bool    `json:"sourceAutoRefresh,omitempty"`
	SourceRefreshIntervalM int      `json:"sourceRefreshIntervalM"`
	KeepRemoved            bool     `json:"keepRemoved"`
	KeepNames              []string `json:"keepNames"`
	SkipNames              []string `json:"skipNames"`
}

// ProxyImportReport 代理导入/刷新报告。
type ProxyImportReport struct {
	SourceID              string         `json:"sourceId"`
	SourceURL             string         `json:"sourceUrl"`
	Added                 int            `json:"added"`
	Updated               int            `json:"updated"`
	Removed               int            `json:"removed"`
	Skipped               int            `json:"skipped"`
	Failed                int            `json:"failed"`
	AffectedProfileCount  int            `json:"affectedProfileCount"`
	ReboundProfileCount   int            `json:"reboundProfileCount"`
	InvalidProfileCount   int            `json:"invalidProfileCount"`
	ImportedProxies       []BrowserProxy `json:"importedProxies"`
	SkippedProxyNames     []string       `json:"skippedProxyNames"`
	UnsupportedProxyNames []string       `json:"unsupportedProxyNames"`
	Errors                []string       `json:"errors"`
}

// ProxyReconcileReport 代理绑定重关联报告。
type ProxyReconcileReport struct {
	ChangedProfileCount int      `json:"changedProfileCount"`
	ReboundProfileCount int      `json:"reboundProfileCount"`
	InvalidProfileCount int      `json:"invalidProfileCount"`
	InvalidProfileIDs   []string `json:"invalidProfileIds"`
}
