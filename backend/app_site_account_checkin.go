package backend

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/siteaccount"
)

const (
	siteAccountCheckinTaskType       = "checkin"
	siteAccountCheckinDefaultTimeout = 90 * time.Second
	siteAccountCheckinDefaultLimit   = 2
	siteAccountCheckinMaxLimit       = 5
)

const siteAccountCheckinScriptText = `
function normalizeText(value) {
  return String(value || '').trim()
}

function normalizeInt(value, fallback, min, max) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return fallback
  const rounded = Math.round(parsed)
  return Math.max(min, Math.min(max, rounded))
}

function buildRuleCandidates(rule) {
  const raw = normalizeText(rule)
  const defaults = ['签到', '打卡', '领取', 'Check in', 'Daily reward', 'check in']
  if (!raw) {
    return defaults.map((text) => ({ type: 'text', value: text }))
  }

  const lower = raw.toLowerCase()
  if (lower.startsWith('css=') || lower.startsWith('css:')) {
    return [{ type: 'css', value: raw.slice(4).trim() }]
  }
  if (lower.startsWith('text=') || lower.startsWith('text:')) {
    return [{ type: 'text', value: raw.slice(5).trim() }]
  }
  if (lower.startsWith('xpath=') || lower.startsWith('xpath:')) {
    return [{ type: 'xpath', value: raw.slice(6).trim() }]
  }
  if (raw.startsWith('//') || raw.startsWith('(//')) {
    return [{ type: 'xpath', value: raw }]
  }

  return [
    { type: 'css', value: raw },
    { type: 'text', value: raw },
  ]
}

function locatorForRule(page, rule) {
  if (rule.type === 'xpath') {
    return page.locator('xpath=' + rule.value).first()
  }
  if (rule.type === 'text') {
    if (typeof page.getByText === 'function') {
      return page.getByText(rule.value, { exact: false }).first()
    }
    return page.locator('text=' + rule.value).first()
  }
  return page.locator(rule.value).first()
}

async function clickByRules(page, rules, timeoutMs) {
  const errors = []
  for (const rule of rules) {
    if (!rule.value) continue
    try {
      const locator = locatorForRule(page, rule)
      await locator.waitFor({ state: 'visible', timeout: Math.min(timeoutMs, 8000) })
      await locator.scrollIntoViewIfNeeded({ timeout: 3000 }).catch(() => {})
      await locator.click({ timeout: Math.min(timeoutMs, 10000) })
      return rule
    } catch (error) {
      errors.push(rule.type + ':' + rule.value + ' -> ' + (error && error.message ? error.message : String(error)))
    }
  }
  throw new Error('未找到可点击的签到按钮；尝试规则：' + errors.join(' | '))
}

exports.run = async ({ launch, connect, params, log, artifact }) => {
  const checkinUrl = normalizeText(params.checkinUrl)
  if (!checkinUrl) {
    throw new Error('checkinUrl is required')
  }

  const timeoutMs = normalizeInt(params.timeoutMs, 60000, 5000, 180000)
  const waitAfterLoadMs = normalizeInt(params.waitAfterLoadMs, 1200, 0, 15000)
  const rules = buildRuleCandidates(params.buttonRule)

  const session = await launch({
    profileId: normalizeText(params.profileId),
    startUrls: [checkinUrl],
    skipDefaultStartUrls: true,
  })
  const connected = await connect(session)
  const context = connected.context
  let page = connected.page
  if (!page && context) {
    page = await context.newPage()
  }
  if (!page) {
    throw new Error('无法获取浏览器页面')
  }

  await page.goto(checkinUrl, { waitUntil: 'domcontentloaded', timeout: timeoutMs }).catch(async (error) => {
    const current = page.url()
    if (!current || current === 'about:blank') {
      throw error
    }
  })
  if (waitAfterLoadMs > 0) {
    await page.waitForTimeout(waitAfterLoadMs)
  }

  try {
    const clickedRule = await clickByRules(page, rules, timeoutMs)
    await page.waitForTimeout(800)
    const title = await page.title().catch(() => '')
    const url = page.url()
    log('site account checkin clicked', clickedRule, url)
    return {
      ok: true,
      summary: '签到已执行：' + clickedRule.type + ':' + clickedRule.value,
      clickedRule,
      title,
      url,
    }
  } catch (error) {
    let screenshotPath = ''
    try {
      screenshotPath = artifact('checkin-failed.png')
      await page.screenshot({ path: screenshotPath, fullPage: true })
    } catch {}
    return {
      ok: false,
      summary: '签到失败',
      error: error && error.message ? error.message : String(error),
      screenshotPath,
      url: page.url(),
      title: await page.title().catch(() => ''),
    }
  }
}
`

type SiteAccountCheckinRequest = siteaccount.CheckinRequest
type SiteAccountCheckinBatchResult = siteaccount.CheckinBatchResult

func (a *App) SiteAccountRunCheckin(input SiteAccountCheckinRequest) (*SiteAccountCheckinBatchResult, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	accounts, err := a.resolveSiteAccountCheckinTargets(store, input)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return &siteaccount.CheckinBatchResult{Runs: []siteaccount.TaskRun{}}, nil
	}

	runCtx := a.ctx
	if runCtx == nil {
		runCtx = context.Background()
	}
	if err := a.ensureSiteAccountAutomationReady(runCtx); err != nil {
		return nil, err
	}

	concurrency := input.Concurrency
	if concurrency <= 0 {
		concurrency = siteAccountCheckinDefaultLimit
	}
	if concurrency > siteAccountCheckinMaxLimit {
		concurrency = siteAccountCheckinMaxLimit
	}

	jobs := make(chan siteaccount.Account)
	results := make(chan siteaccount.TaskRun, len(accounts))
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for account := range jobs {
				results <- a.runSiteAccountCheckinOne(runCtx, store, account)
			}
		}()
	}

	for _, account := range accounts {
		jobs <- account
	}
	close(jobs)
	wg.Wait()
	close(results)

	out := siteaccount.CheckinBatchResult{Total: len(accounts), Runs: []siteaccount.TaskRun{}}
	for run := range results {
		out.Runs = append(out.Runs, run)
		if run.Status == "success" {
			out.Succeeded++
		} else {
			out.Failed++
		}
	}
	return &out, nil
}

func (a *App) SiteAccountRunCheckinByID(accountID string) (*SiteAccountCheckinBatchResult, error) {
	return a.SiteAccountRunCheckin(siteaccount.CheckinRequest{AccountID: strings.TrimSpace(accountID), Concurrency: 1})
}

func (a *App) SiteRunCheckin(siteID string) (*SiteAccountCheckinBatchResult, error) {
	return a.SiteAccountRunCheckin(siteaccount.CheckinRequest{SiteID: strings.TrimSpace(siteID)})
}

func (a *App) resolveSiteAccountCheckinTargets(store *siteaccount.Store, input siteaccount.CheckinRequest) ([]siteaccount.Account, error) {
	if accountID := strings.TrimSpace(input.AccountID); accountID != "" {
		account, err := store.GetAccount(accountID, false)
		if err != nil {
			return nil, err
		}
		return []siteaccount.Account{*account}, nil
	}

	filter := input.Filter
	if siteID := strings.TrimSpace(input.SiteID); siteID != "" {
		filter.SiteID = siteID
	}
	if profileID := strings.TrimSpace(input.ProfileID); profileID != "" {
		filter.ProfileID = profileID
	}
	if strings.TrimSpace(filter.AutoCheckin) == "" {
		filter.AutoCheckin = "enabled"
	}
	return store.ListAccounts(filter)
}

func (a *App) ensureSiteAccountAutomationReady(ctx context.Context) error {
	if a.automationMgr == nil {
		return fmt.Errorf("自动化运行时尚未初始化")
	}
	if a.config == nil || !a.config.Automation.Enabled {
		return fmt.Errorf("自动化支持尚未启用，请先在自动化设置中启用")
	}
	if err := a.automationMgr.EnsureInstalled(ctx); err != nil {
		return err
	}
	state := a.automationMgr.CurrentState()
	if !state.Ready {
		return fmt.Errorf("自动化运行时尚未就绪")
	}
	return nil
}

func (a *App) runSiteAccountCheckinOne(parentCtx context.Context, store *siteaccount.Store, account siteaccount.Account) siteaccount.TaskRun {
	startedAt := time.Now()
	run := siteaccount.TaskRun{
		TaskType:  siteAccountCheckinTaskType,
		SiteID:    account.SiteID,
		AccountID: account.AccountID,
		ProfileID: account.ProfileID,
		Status:    "failed",
		StartedAt: startedAt.Format(time.RFC3339),
	}

	save := func() siteaccount.TaskRun {
		run.FinishedAt = time.Now().Format(time.RFC3339)
		run.DurationMs = time.Since(startedAt).Milliseconds()
		saved, err := store.SaveTaskRun(run)
		if err != nil {
			run.Error = appendSiteAccountError(run.Error, err.Error())
			return run
		}
		return saved
	}

	site, err := store.GetSite(account.SiteID)
	if err != nil {
		run.Summary = "签到失败"
		run.Error = err.Error()
		return save()
	}
	checkinURL := firstNonEmpty(account.CheckinURL, site.CheckinURL, account.AccountURL, site.HomeURL)
	if checkinURL == "" {
		run.Summary = "签到失败"
		run.Error = "未配置签到 URL"
		return save()
	}
	buttonRule := firstNonEmpty(account.CheckinButtonRule, site.CheckinButtonRule)

	ctx, cancel := context.WithTimeout(parentCtx, siteAccountCheckinDefaultTimeout)
	defer cancel()

	state := a.automationMgr.CurrentState()
	script := automation.ScriptRecord{
		ID:         "site-account-checkin-" + sanitizeSiteAccountScriptID(account.AccountID),
		Name:       "站点账号自动签到",
		Type:       "playwright-cdp",
		Status:     "ready",
		EntryFile:  "index.cjs",
		ScriptText: siteAccountCheckinScriptText,
	}
	scriptPath, artifactDir, cleanup, err := a.preparePlaywrightScriptWorkspace(state.RuntimeDir, script)
	if err != nil {
		run.Summary = "签到失败"
		run.Error = err.Error()
		return save()
	}
	defer cleanup()
	run.ArtifactPath = filepath.ToSlash(artifactDir)

	baseURL, authHeader, authValue, err := a.automationDemoEndpoint()
	if err != nil {
		run.Summary = "签到失败"
		run.Error = err.Error()
		return save()
	}

	taskResult, err := a.automationMgr.RunScriptTask(ctx, automation.ScriptTaskRequest{
		TaskKey:          "site-checkin:" + account.ProfileID,
		ScriptPath:       scriptPath,
		Selector:         map[string]any{"profileId": account.ProfileID},
		LaunchBaseURL:    baseURL,
		LaunchAuthHeader: authHeader,
		LaunchAuthValue:  authValue,
		ArtifactDir:      artifactDir,
		Timeout:          siteAccountCheckinDefaultTimeout,
		Params: map[string]any{
			"profileId":       account.ProfileID,
			"siteId":          account.SiteID,
			"siteName":        site.SiteName,
			"accountId":       account.AccountID,
			"username":        account.Username,
			"checkinUrl":      checkinURL,
			"buttonRule":      buttonRule,
			"timeoutMs":       int(siteAccountCheckinDefaultTimeout / time.Millisecond),
			"waitAfterLoadMs": 1200,
		},
	})
	if err != nil {
		run.Summary = "签到失败"
		run.Error = err.Error()
		return save()
	}

	run.StartedAt = firstNonEmpty(taskResult.StartedAt, run.StartedAt)
	run.FinishedAt = firstNonEmpty(taskResult.FinishedAt, time.Now().Format(time.RFC3339))
	run.DurationMs = taskResult.DurationMs
	run.Summary = firstNonEmpty(taskResult.Summary, "签到执行完成")
	if taskResult.OK && strings.TrimSpace(taskResult.Error) == "" {
		run.Status = "success"
	} else {
		run.Status = "failed"
		run.Error = firstNonEmpty(taskResult.Error, "签到脚本返回失败")
	}
	saved, saveErr := store.SaveTaskRun(run)
	if saveErr != nil {
		run.Error = appendSiteAccountError(run.Error, saveErr.Error())
		return run
	}
	return saved
}

func sanitizeSiteAccountScriptID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "account"
	}
	return b.String()
}

func appendSiteAccountError(base string, extra string) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	if base == "" {
		return extra
	}
	if extra == "" {
		return base
	}
	return base + "；" + extra
}
