package backend

import (
	"ant-chrome/backend/internal/logger"
	"strings"
	"time"
)

// reconcileProfileProxyBindings 对实例代理绑定执行幂等修复：
// 1. 同步已存在 proxyId 的绑定快照；
// 2. 当 proxyId 失效时按绑定快照/配置执行自动重关联；
// 3. 仅在有变更时持久化。
func (a *App) reconcileProfileProxyBindings() {
	_ = a.reconcileProfileProxyBindingsWithReport()
}

// BrowserProxyReconcileBindings 手动执行代理绑定重关联，并返回统计报告。
func (a *App) BrowserProxyReconcileBindings() (*ProxyReconcileReport, error) {
	return a.reconcileProfileProxyBindingsWithReport(), nil
}

func (a *App) reconcileProfileProxyBindingsWithReport() *ProxyReconcileReport {
	report := &ProxyReconcileReport{}
	if a == nil || a.browserMgr == nil {
		return report
	}

	log := logger.New("Browser")
	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	for _, profile := range a.browserMgr.Profiles {
		hadBinding := strings.TrimSpace(profile.ProxyId) != "" ||
			strings.TrimSpace(profile.ProxyBindSourceID) != "" ||
			strings.TrimSpace(profile.ProxyBindSourceURL) != "" ||
			strings.TrimSpace(profile.ProxyBindName) != ""
		changed, boundInPool, mode := a.browserMgr.ResolveProfileProxyBinding(profile)
		if changed {
			profile.UpdatedAt = time.Now().Format(time.RFC3339)
			report.ChangedProfileCount++
		}
		if boundInPool && mode != "" && mode != "proxy_id" {
			report.ReboundProfileCount++
			log.Info("实例代理重关联成功",
				logger.F("profile_id", profile.ProfileId),
				logger.F("profile_name", profile.ProfileName),
				logger.F("proxy_id", profile.ProxyId),
				logger.F("mode", mode),
			)
		}
		if hadBinding && !boundInPool {
			report.InvalidProfileCount++
			report.InvalidProfileIDs = append(report.InvalidProfileIDs, profile.ProfileId)
		}
	}

	if report.ChangedProfileCount == 0 {
		return report
	}
	if err := a.browserMgr.SaveProfiles(); err != nil {
		log.Error("实例代理绑定修复持久化失败", logger.F("error", err.Error()))
		return report
	}
	log.Info("实例代理绑定修复完成",
		logger.F("changed", report.ChangedProfileCount),
		logger.F("rebound", report.ReboundProfileCount),
		logger.F("invalid", report.InvalidProfileCount),
	)
	return report
}
