package siteaccount

type Site struct {
	SiteID            string `json:"siteId"`
	SiteName          string `json:"siteName"`
	HomeURL           string `json:"homeUrl"`
	LoginURL          string `json:"loginUrl"`
	CheckinURL        string `json:"checkinUrl"`
	ReadingURL        string `json:"readingUrl"`
	CheckinButtonRule string `json:"checkinButtonRule"`
	Status            string `json:"status"`
	Remark            string `json:"remark"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type SiteInput struct {
	SiteID            string `json:"siteId"`
	SiteName          string `json:"siteName"`
	HomeURL           string `json:"homeUrl"`
	LoginURL          string `json:"loginUrl"`
	CheckinURL        string `json:"checkinUrl"`
	ReadingURL        string `json:"readingUrl"`
	CheckinButtonRule string `json:"checkinButtonRule"`
	Status            string `json:"status"`
	Remark            string `json:"remark"`
}

type SiteSummary struct {
	Site
	AccountCount     int `json:"accountCount"`
	AutoCheckinCount int `json:"autoCheckinCount"`
	AutoReadCount    int `json:"autoReadCount"`
}

type Account struct {
	AccountID          string `json:"accountId"`
	SiteID             string `json:"siteId"`
	SiteName           string `json:"siteName,omitempty"`
	ProfileID          string `json:"profileId"`
	ProfileName        string `json:"profileName,omitempty"`
	Username           string `json:"username"`
	Password           string `json:"password,omitempty"`
	HasPassword        bool   `json:"hasPassword"`
	Email              string `json:"email"`
	EmailPassword      string `json:"emailPassword,omitempty"`
	HasEmailPassword   bool   `json:"hasEmailPassword"`
	AccountURL         string `json:"accountUrl"`
	AutoCheckinEnabled bool   `json:"autoCheckinEnabled"`
	CheckinURL         string `json:"checkinUrl"`
	CheckinButtonRule  string `json:"checkinButtonRule"`
	AutoReadEnabled    bool   `json:"autoReadEnabled"`
	ReadingURL         string `json:"readingUrl"`
	Status             string `json:"status"`
	Remark             string `json:"remark"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type AccountInput struct {
	AccountID          string `json:"accountId"`
	SiteID             string `json:"siteId"`
	ProfileID          string `json:"profileId"`
	Username           string `json:"username"`
	Password           string `json:"password"`
	ClearPassword      bool   `json:"clearPassword"`
	Email              string `json:"email"`
	EmailPassword      string `json:"emailPassword"`
	ClearEmailPassword bool   `json:"clearEmailPassword"`
	AccountURL         string `json:"accountUrl"`
	AutoCheckinEnabled bool   `json:"autoCheckinEnabled"`
	CheckinURL         string `json:"checkinUrl"`
	CheckinButtonRule  string `json:"checkinButtonRule"`
	AutoReadEnabled    bool   `json:"autoReadEnabled"`
	ReadingURL         string `json:"readingUrl"`
	Status             string `json:"status"`
	Remark             string `json:"remark"`
}

type AccountFilter struct {
	SiteID      string `json:"siteId"`
	ProfileID   string `json:"profileId"`
	Status      string `json:"status"`
	Keyword     string `json:"keyword"`
	AutoCheckin string `json:"autoCheckin"`
	AutoRead    string `json:"autoRead"`
}

type TaskRun struct {
	RunID        string `json:"runId"`
	TaskType     string `json:"taskType"`
	SiteID       string `json:"siteId"`
	AccountID    string `json:"accountId"`
	ProfileID    string `json:"profileId"`
	Status       string `json:"status"`
	Summary      string `json:"summary"`
	Error        string `json:"error"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	DurationMs   int64  `json:"durationMs"`
	ArtifactPath string `json:"artifactPath"`
}

type CheckinRequest struct {
	AccountID   string        `json:"accountId"`
	SiteID      string        `json:"siteId"`
	ProfileID   string        `json:"profileId"`
	Filter      AccountFilter `json:"filter"`
	Concurrency int           `json:"concurrency"`
}

type CheckinBatchResult struct {
	Total     int       `json:"total"`
	Succeeded int       `json:"succeeded"`
	Failed    int       `json:"failed"`
	Runs      []TaskRun `json:"runs"`
}
