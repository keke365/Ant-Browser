package backend

import (
	"ant-chrome/backend/internal/siteaccount"
	"fmt"
	"strings"
)

type Site = siteaccount.Site
type SiteInput = siteaccount.SiteInput
type SiteSummary = siteaccount.SiteSummary
type SiteAccount = siteaccount.Account
type SiteAccountInput = siteaccount.AccountInput
type SiteAccountFilter = siteaccount.AccountFilter
type SiteAccountTaskRun = siteaccount.TaskRun

func (a *App) ensureSiteAccountStore() (*siteaccount.Store, error) {
	if a == nil || a.siteAccountStore == nil {
		return nil, fmt.Errorf("站点账号管理尚未初始化")
	}
	return a.siteAccountStore, nil
}

func (a *App) SiteList() ([]SiteSummary, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	return store.ListSites()
}

func (a *App) SiteGet(siteID string) (*Site, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	return store.GetSite(siteID)
}

func (a *App) SiteSave(input SiteInput) (*Site, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	return store.SaveSite(input)
}

func (a *App) SiteDelete(siteID string) error {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return err
	}
	return store.DeleteSite(siteID)
}

func (a *App) SiteAccountList(filter SiteAccountFilter) ([]SiteAccount, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	return store.ListAccounts(filter)
}

func (a *App) SiteAccountListByProfile(profileID string) ([]SiteAccount, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	return store.ListAccounts(siteaccount.AccountFilter{ProfileID: strings.TrimSpace(profileID)})
}

func (a *App) SiteAccountGet(accountID string, revealSensitive bool) (*SiteAccount, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	return store.GetAccount(accountID, revealSensitive)
}

func (a *App) SiteAccountSave(input SiteAccountInput) (*SiteAccount, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	profileID := strings.TrimSpace(input.ProfileID)
	if profileID == "" {
		return nil, fmt.Errorf("请选择指纹浏览器")
	}
	a.browserMgr.Mutex.Lock()
	_, exists := a.browserMgr.Profiles[profileID]
	a.browserMgr.Mutex.Unlock()
	if !exists {
		return nil, fmt.Errorf("指纹浏览器不存在，请刷新后重试")
	}
	return store.SaveAccount(input)
}

func (a *App) SiteAccountDelete(accountID string) error {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return err
	}
	return store.DeleteAccount(accountID)
}

func (a *App) SiteAccountQuickOpen(accountID string, target string) (*BrowserProfile, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	account, err := store.GetAccount(accountID, false)
	if err != nil {
		return nil, err
	}
	site, err := store.GetSite(account.SiteID)
	if err != nil {
		return nil, err
	}

	targetURL := resolveSiteAccountOpenURL(*site, *account, target)
	if targetURL == "" {
		return nil, fmt.Errorf("未配置可打开的网址")
	}
	return a.BrowserInstanceStartWithParams(account.ProfileID, nil, []string{targetURL}, true)
}

func (a *App) SiteAccountTaskRunList(accountID string, limit int) ([]SiteAccountTaskRun, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	return store.ListTaskRuns(accountID, limit)
}

func resolveSiteAccountOpenURL(site Site, account SiteAccount, target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "login":
		return firstNonEmpty(site.LoginURL, account.AccountURL, site.HomeURL)
	case "checkin":
		return firstNonEmpty(account.CheckinURL, site.CheckinURL, account.AccountURL, site.HomeURL)
	case "reading":
		return firstNonEmpty(account.ReadingURL, site.ReadingURL, account.AccountURL, site.HomeURL)
	case "home":
		return firstNonEmpty(site.HomeURL, account.AccountURL)
	default:
		return firstNonEmpty(account.AccountURL, site.HomeURL, site.LoginURL)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
