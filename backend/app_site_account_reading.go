package backend

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/automation"
	"ant-chrome/backend/internal/siteaccount"
)

const (
	siteAccountReadingTaskType       = "reading"
	siteAccountReadingDefaultTimeout = 3 * time.Minute
	siteAccountReadingDefaultLimit   = 2
	siteAccountReadingMaxLimit       = 5
)

const siteAccountReadingScriptText = `
function normalizeText(value) {
  return String(value || '').trim()
}

function normalizeInt(value, fallback, min, max) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return fallback
  const rounded = Math.round(parsed)
  return Math.max(min, Math.min(max, rounded))
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function randomBetween(min, max) {
  return Math.round(min + Math.random() * Math.max(0, max - min))
}

exports.run = async ({ launch, connect, params, log, artifact }) => {
  const readingUrl = normalizeText(params.readingUrl)
  if (!readingUrl) {
    throw new Error('readingUrl is required')
  }

  const timeoutMs = normalizeInt(params.timeoutMs, 120000, 10000, 600000)
  const maxScrolls = normalizeInt(params.maxScrolls, 8, 1, 80)
  const minWaitMs = normalizeInt(params.minWaitMs, 1200, 200, 30000)
  const maxWaitMs = normalizeInt(params.maxWaitMs, 3500, minWaitMs, 60000)
  const minScrollPx = normalizeInt(params.minScrollPx, 260, 80, 3000)
  const maxScrollPx = normalizeInt(params.maxScrollPx, 760, minScrollPx, 5000)

  const session = await launch({
    profileId: normalizeText(params.profileId),
    startUrls: [readingUrl],
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

  await page.goto(readingUrl, { waitUntil: 'domcontentloaded', timeout: timeoutMs }).catch(async (error) => {
    const current = page.url()
    if (!current || current === 'about:blank') {
      throw error
    }
  })

  const deadline = Date.now() + timeoutMs
  let scrollCount = 0
  let lastY = -1
  let stableCount = 0

  while (Date.now() < deadline && scrollCount < maxScrolls) {
    const waitMs = randomBetween(minWaitMs, maxWaitMs)
    await sleep(Math.min(waitMs, Math.max(0, deadline - Date.now())))
    const distance = randomBetween(minScrollPx, maxScrollPx)
    await page.mouse.wheel(0, distance).catch(() => {})
    await page.evaluate((dy) => window.scrollBy({ top: dy, behavior: 'smooth' }), distance).catch(() => {})
    await sleep(350)
    const y = await page.evaluate(() => Math.round(window.scrollY || document.documentElement.scrollTop || 0)).catch(() => lastY)
    if (y === lastY) {
      stableCount += 1
    } else {
      stableCount = 0
    }
    lastY = y
    scrollCount += 1
    if (stableCount >= 3) {
      break
    }
  }

  let screenshotPath = ''
  if (params.captureScreenshot === true) {
    try {
      screenshotPath = artifact('reading-finished.png')
      await page.screenshot({ path: screenshotPath, fullPage: true })
    } catch {}
  }

  const title = await page.title().catch(() => '')
  const url = page.url()
  log('site account reading finished', { scrollCount, url })
  return {
    ok: true,
    summary: '阅读完成：滚动 ' + scrollCount + ' 次',
    scrollCount,
    title,
    url,
    screenshotPath,
  }
}
`

func (a *App) SiteAccountRunReading(input SiteAccountCheckinRequest) (*SiteAccountCheckinBatchResult, error) {
	store, err := a.ensureSiteAccountStore()
	if err != nil {
		return nil, err
	}
	accounts, err := a.resolveSiteAccountReadingTargets(store, input)
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
		concurrency = siteAccountReadingDefaultLimit
	}
	if concurrency > siteAccountReadingMaxLimit {
		concurrency = siteAccountReadingMaxLimit
	}

	jobs := make(chan siteaccount.Account)
	results := make(chan siteaccount.TaskRun, len(accounts))
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for account := range jobs {
				results <- a.runSiteAccountReadingOne(runCtx, store, account)
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

func (a *App) SiteAccountRunReadingByID(accountID string) (*SiteAccountCheckinBatchResult, error) {
	return a.SiteAccountRunReading(siteaccount.CheckinRequest{AccountID: strings.TrimSpace(accountID), Concurrency: 1})
}

func (a *App) SiteRunReading(siteID string) (*SiteAccountCheckinBatchResult, error) {
	return a.SiteAccountRunReading(siteaccount.CheckinRequest{SiteID: strings.TrimSpace(siteID)})
}

func (a *App) resolveSiteAccountReadingTargets(store *siteaccount.Store, input siteaccount.CheckinRequest) ([]siteaccount.Account, error) {
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
	if strings.TrimSpace(filter.AutoRead) == "" {
		filter.AutoRead = "enabled"
	}
	return store.ListAccounts(filter)
}

func (a *App) runSiteAccountReadingOne(parentCtx context.Context, store *siteaccount.Store, account siteaccount.Account) siteaccount.TaskRun {
	startedAt := time.Now()
	run := siteaccount.TaskRun{
		TaskType:  siteAccountReadingTaskType,
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
		run.Summary = "阅读失败"
		run.Error = err.Error()
		return save()
	}
	readingURL := firstNonEmpty(account.ReadingURL, site.ReadingURL, account.AccountURL, site.HomeURL)
	if readingURL == "" {
		run.Summary = "阅读失败"
		run.Error = "未配置阅读 URL"
		return save()
	}

	ctx, cancel := context.WithTimeout(parentCtx, siteAccountReadingDefaultTimeout)
	defer cancel()

	state := a.automationMgr.CurrentState()
	script := automation.ScriptRecord{
		ID:         "site-account-reading-" + sanitizeSiteAccountScriptID(account.AccountID),
		Name:       "站点账号自动阅读",
		Type:       "playwright-cdp",
		Status:     "ready",
		EntryFile:  "index.cjs",
		ScriptText: siteAccountReadingScriptText,
	}
	scriptPath, artifactDir, cleanup, err := a.preparePlaywrightScriptWorkspace(state.RuntimeDir, script)
	if err != nil {
		run.Summary = "阅读失败"
		run.Error = err.Error()
		return save()
	}
	defer cleanup()
	run.ArtifactPath = filepath.ToSlash(artifactDir)

	baseURL, authHeader, authValue, err := a.automationDemoEndpoint()
	if err != nil {
		run.Summary = "阅读失败"
		run.Error = err.Error()
		return save()
	}

	taskResult, err := a.automationMgr.RunScriptTask(ctx, automation.ScriptTaskRequest{
		TaskKey:          "site-reading:" + account.ProfileID,
		ScriptPath:       scriptPath,
		Selector:         map[string]any{"profileId": account.ProfileID},
		LaunchBaseURL:    baseURL,
		LaunchAuthHeader: authHeader,
		LaunchAuthValue:  authValue,
		ArtifactDir:      artifactDir,
		Timeout:          siteAccountReadingDefaultTimeout,
		Params: map[string]any{
			"profileId":  account.ProfileID,
			"siteId":     account.SiteID,
			"siteName":   site.SiteName,
			"accountId":  account.AccountID,
			"username":   account.Username,
			"readingUrl": readingURL,
			"timeoutMs":  int(siteAccountReadingDefaultTimeout / time.Millisecond),
			"maxScrolls": 8,
			"minWaitMs":  1200,
			"maxWaitMs":  3500,
		},
	})
	if err != nil {
		run.Summary = "阅读失败"
		run.Error = err.Error()
		return save()
	}

	run.StartedAt = firstNonEmpty(taskResult.StartedAt, run.StartedAt)
	run.FinishedAt = firstNonEmpty(taskResult.FinishedAt, time.Now().Format(time.RFC3339))
	run.DurationMs = taskResult.DurationMs
	run.Summary = firstNonEmpty(taskResult.Summary, "阅读执行完成")
	if taskResult.OK && strings.TrimSpace(taskResult.Error) == "" {
		run.Status = "success"
	} else {
		run.Status = "failed"
		run.Error = firstNonEmpty(taskResult.Error, "阅读脚本返回失败")
	}
	saved, saveErr := store.SaveTaskRun(run)
	if saveErr != nil {
		run.Error = appendSiteAccountError(run.Error, saveErr.Error())
		return run
	}
	return saved
}
