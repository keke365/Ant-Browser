package siteaccount

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	db      *sql.DB
	secrets *secretKeeper
}

func NewStore(db *sql.DB, keyPath string) *Store {
	return &Store{
		db:      db,
		secrets: newSecretKeeper(keyPath),
	}
}

func (s *Store) ListSites() ([]SiteSummary, error) {
	rows, err := s.db.Query(`
		SELECT s.site_id, s.site_name, s.home_url, s.login_url, s.checkin_url, s.reading_url,
		       s.checkin_button_rule, s.status, s.remark, s.created_at, s.updated_at,
		       COUNT(a.account_id) AS account_count,
		       COALESCE(SUM(CASE WHEN a.auto_checkin_enabled = 1 THEN 1 ELSE 0 END), 0) AS auto_checkin_count,
		       COALESCE(SUM(CASE WHEN a.auto_read_enabled = 1 THEN 1 ELSE 0 END), 0) AS auto_read_count
		FROM sites s
		LEFT JOIN site_accounts a ON a.site_id = s.site_id
		GROUP BY s.site_id
		ORDER BY lower(s.site_name) ASC, s.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询站点列表失败: %w", err)
	}
	defer rows.Close()

	items := []SiteSummary{}
	for rows.Next() {
		var item SiteSummary
		if err := rows.Scan(
			&item.SiteID, &item.SiteName, &item.HomeURL, &item.LoginURL, &item.CheckinURL, &item.ReadingURL,
			&item.CheckinButtonRule, &item.Status, &item.Remark, &item.CreatedAt, &item.UpdatedAt,
			&item.AccountCount, &item.AutoCheckinCount, &item.AutoReadCount,
		); err != nil {
			return nil, fmt.Errorf("读取站点列表失败: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetSite(siteID string) (*Site, error) {
	row := s.db.QueryRow(`
		SELECT site_id, site_name, home_url, login_url, checkin_url, reading_url,
		       checkin_button_rule, status, remark, created_at, updated_at
		FROM sites WHERE site_id = ?`, strings.TrimSpace(siteID))
	site, err := scanSite(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("站点不存在")
	}
	if err != nil {
		return nil, fmt.Errorf("查询站点失败: %w", err)
	}
	return site, nil
}

func (s *Store) SaveSite(input SiteInput) (*Site, error) {
	site, err := normalizeSiteInput(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().Format(time.RFC3339)
	if site.SiteID == "" {
		site.SiteID = uuid.NewString()
		site.CreatedAt = now
	}
	site.UpdatedAt = now
	if site.CreatedAt == "" {
		var createdAt string
		_ = s.db.QueryRow(`SELECT created_at FROM sites WHERE site_id = ?`, site.SiteID).Scan(&createdAt)
		if createdAt != "" {
			site.CreatedAt = createdAt
		} else {
			site.CreatedAt = now
		}
	}

	_, err = s.db.Exec(`
		INSERT INTO sites (
		  site_id, site_name, home_url, login_url, checkin_url, reading_url,
		  checkin_button_rule, status, remark, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(site_id) DO UPDATE SET
		  site_name = excluded.site_name,
		  home_url = excluded.home_url,
		  login_url = excluded.login_url,
		  checkin_url = excluded.checkin_url,
		  reading_url = excluded.reading_url,
		  checkin_button_rule = excluded.checkin_button_rule,
		  status = excluded.status,
		  remark = excluded.remark,
		  updated_at = excluded.updated_at`,
		site.SiteID, site.SiteName, site.HomeURL, site.LoginURL, site.CheckinURL, site.ReadingURL,
		site.CheckinButtonRule, site.Status, site.Remark, site.CreatedAt, site.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("保存站点失败: %w", err)
	}
	return s.GetSite(site.SiteID)
}

func (s *Store) DeleteSite(siteID string) error {
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		return fmt.Errorf("站点 ID 不能为空")
	}
	_, err := s.db.Exec(`DELETE FROM sites WHERE site_id = ?`, siteID)
	if err != nil {
		return fmt.Errorf("删除站点失败: %w", err)
	}
	return nil
}

func (s *Store) ListAccounts(filter AccountFilter) ([]Account, error) {
	where := []string{"1=1"}
	args := []any{}
	if value := strings.TrimSpace(filter.SiteID); value != "" {
		where = append(where, "a.site_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.ProfileID); value != "" {
		where = append(where, "a.profile_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" && !strings.EqualFold(value, "all") {
		where = append(where, "a.status = ?")
		args = append(args, value)
	}
	switch strings.ToLower(strings.TrimSpace(filter.AutoCheckin)) {
	case "enabled", "true", "1":
		where = append(where, "a.auto_checkin_enabled = 1")
	case "disabled", "false", "0":
		where = append(where, "a.auto_checkin_enabled = 0")
	}
	switch strings.ToLower(strings.TrimSpace(filter.AutoRead)) {
	case "enabled", "true", "1":
		where = append(where, "a.auto_read_enabled = 1")
	case "disabled", "false", "0":
		where = append(where, "a.auto_read_enabled = 0")
	}
	if keyword := strings.ToLower(strings.TrimSpace(filter.Keyword)); keyword != "" {
		pattern := "%" + keyword + "%"
		where = append(where, `(lower(a.username) LIKE ? OR lower(a.email) LIKE ? OR lower(a.remark) LIKE ? OR lower(s.site_name) LIKE ? OR lower(COALESCE(p.profile_name,'')) LIKE ?)`)
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}

	query := fmt.Sprintf(`
		SELECT a.account_id, a.site_id, s.site_name, a.profile_id, COALESCE(p.profile_name, ''),
		       a.username, a.password, a.email, a.email_password, a.account_url,
		       a.auto_checkin_enabled, a.checkin_url, a.checkin_button_rule,
		       a.auto_read_enabled, a.reading_url, a.status, a.remark, a.created_at, a.updated_at
		FROM site_accounts a
		INNER JOIN sites s ON s.site_id = a.site_id
		LEFT JOIN browser_profiles p ON p.profile_id = a.profile_id
		WHERE %s
		ORDER BY lower(s.site_name) ASC, lower(a.username) ASC, a.created_at DESC`, strings.Join(where, " AND "))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询账号列表失败: %w", err)
	}
	defer rows.Close()

	items := []Account{}
	for rows.Next() {
		account, err := s.scanAccount(rows, false)
		if err != nil {
			return nil, err
		}
		items = append(items, *account)
	}
	return items, rows.Err()
}

func (s *Store) GetAccount(accountID string, revealSensitive bool) (*Account, error) {
	row := s.db.QueryRow(`
		SELECT a.account_id, a.site_id, s.site_name, a.profile_id, COALESCE(p.profile_name, ''),
		       a.username, a.password, a.email, a.email_password, a.account_url,
		       a.auto_checkin_enabled, a.checkin_url, a.checkin_button_rule,
		       a.auto_read_enabled, a.reading_url, a.status, a.remark, a.created_at, a.updated_at
		FROM site_accounts a
		INNER JOIN sites s ON s.site_id = a.site_id
		LEFT JOIN browser_profiles p ON p.profile_id = a.profile_id
		WHERE a.account_id = ?`, strings.TrimSpace(accountID))
	account, err := s.scanAccount(row, revealSensitive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("账号不存在")
	}
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *Store) SaveAccount(input AccountInput) (*Account, error) {
	account, err := s.normalizeAccountInput(input)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	existing, _ := s.GetAccount(account.AccountID, true)
	if account.AccountID == "" {
		account.AccountID = uuid.NewString()
		account.CreatedAt = now
	} else if existing != nil {
		account.CreatedAt = existing.CreatedAt
		if input.Password == "" && !input.ClearPassword {
			account.Password = existing.Password
		}
		if input.EmailPassword == "" && !input.ClearEmailPassword {
			account.EmailPassword = existing.EmailPassword
		}
	}
	if account.CreatedAt == "" {
		account.CreatedAt = now
	}
	account.UpdatedAt = now

	encryptedPassword, err := s.secrets.encrypt(account.Password)
	if err != nil {
		return nil, err
	}
	encryptedEmailPassword, err := s.secrets.encrypt(account.EmailPassword)
	if err != nil {
		return nil, err
	}

	autoCheckin := boolToInt(account.AutoCheckinEnabled)
	autoRead := boolToInt(account.AutoReadEnabled)
	_, err = s.db.Exec(`
		INSERT INTO site_accounts (
		  account_id, site_id, profile_id, username, password, email, email_password, account_url,
		  auto_checkin_enabled, checkin_url, checkin_button_rule,
		  auto_read_enabled, reading_url, status, remark, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id) DO UPDATE SET
		  site_id = excluded.site_id,
		  profile_id = excluded.profile_id,
		  username = excluded.username,
		  password = excluded.password,
		  email = excluded.email,
		  email_password = excluded.email_password,
		  account_url = excluded.account_url,
		  auto_checkin_enabled = excluded.auto_checkin_enabled,
		  checkin_url = excluded.checkin_url,
		  checkin_button_rule = excluded.checkin_button_rule,
		  auto_read_enabled = excluded.auto_read_enabled,
		  reading_url = excluded.reading_url,
		  status = excluded.status,
		  remark = excluded.remark,
		  updated_at = excluded.updated_at`,
		account.AccountID, account.SiteID, account.ProfileID, account.Username, encryptedPassword,
		account.Email, encryptedEmailPassword, account.AccountURL, autoCheckin, account.CheckinURL,
		account.CheckinButtonRule, autoRead, account.ReadingURL, account.Status, account.Remark,
		account.CreatedAt, account.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, fmt.Errorf("该站点下已存在绑定同一指纹浏览器的账号")
		}
		return nil, fmt.Errorf("保存账号失败: %w", err)
	}
	return s.GetAccount(account.AccountID, false)
}

func (s *Store) DeleteAccount(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("账号 ID 不能为空")
	}
	_, err := s.db.Exec(`DELETE FROM site_accounts WHERE account_id = ?`, accountID)
	if err != nil {
		return fmt.Errorf("删除账号失败: %w", err)
	}
	return nil
}

func (s *Store) SaveTaskRun(input TaskRun) (TaskRun, error) {
	run := normalizeTaskRun(input)
	_, err := s.db.Exec(`
		INSERT INTO site_account_task_runs (
		  run_id, task_type, site_id, account_id, profile_id, status, summary, error,
		  started_at, finished_at, duration_ms, artifact_path
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
		  task_type = excluded.task_type,
		  site_id = excluded.site_id,
		  account_id = excluded.account_id,
		  profile_id = excluded.profile_id,
		  status = excluded.status,
		  summary = excluded.summary,
		  error = excluded.error,
		  started_at = excluded.started_at,
		  finished_at = excluded.finished_at,
		  duration_ms = excluded.duration_ms,
		  artifact_path = excluded.artifact_path`,
		run.RunID, run.TaskType, run.SiteID, run.AccountID, run.ProfileID, run.Status,
		run.Summary, run.Error, run.StartedAt, run.FinishedAt, run.DurationMs, run.ArtifactPath,
	)
	if err != nil {
		return TaskRun{}, fmt.Errorf("保存站点账号任务记录失败: %w", err)
	}
	return run, nil
}

func (s *Store) ListTaskRuns(accountID string, limit int) ([]TaskRun, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	where := ""
	args := []any{}
	if accountID = strings.TrimSpace(accountID); accountID != "" {
		where = "WHERE account_id = ?"
		args = append(args, accountID)
	}
	args = append(args, limit)
	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT run_id, task_type, site_id, account_id, profile_id, status, summary, error,
		       started_at, finished_at, duration_ms, artifact_path
		FROM site_account_task_runs
		%s
		ORDER BY started_at DESC
		LIMIT ?`, where), args...)
	if err != nil {
		return nil, fmt.Errorf("查询站点账号任务记录失败: %w", err)
	}
	defer rows.Close()

	items := []TaskRun{}
	for rows.Next() {
		var item TaskRun
		if err := rows.Scan(&item.RunID, &item.TaskType, &item.SiteID, &item.AccountID, &item.ProfileID,
			&item.Status, &item.Summary, &item.Error, &item.StartedAt, &item.FinishedAt,
			&item.DurationMs, &item.ArtifactPath); err != nil {
			return nil, fmt.Errorf("读取站点账号任务记录失败: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func normalizeTaskRun(input TaskRun) TaskRun {
	now := time.Now().Format(time.RFC3339)
	run := TaskRun{
		RunID:        strings.TrimSpace(input.RunID),
		TaskType:     strings.TrimSpace(input.TaskType),
		SiteID:       strings.TrimSpace(input.SiteID),
		AccountID:    strings.TrimSpace(input.AccountID),
		ProfileID:    strings.TrimSpace(input.ProfileID),
		Status:       strings.TrimSpace(input.Status),
		Summary:      strings.TrimSpace(input.Summary),
		Error:        strings.TrimSpace(input.Error),
		StartedAt:    strings.TrimSpace(input.StartedAt),
		FinishedAt:   strings.TrimSpace(input.FinishedAt),
		DurationMs:   input.DurationMs,
		ArtifactPath: strings.TrimSpace(input.ArtifactPath),
	}
	if run.RunID == "" {
		run.RunID = uuid.NewString()
	}
	if run.TaskType == "" {
		run.TaskType = "checkin"
	}
	switch run.Status {
	case "queued", "running", "success", "failed", "cancelled":
	default:
		run.Status = "failed"
	}
	if run.StartedAt == "" {
		run.StartedAt = now
	}
	if run.FinishedAt == "" {
		run.FinishedAt = run.StartedAt
	}
	if run.DurationMs < 0 {
		run.DurationMs = 0
	}
	return run
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSite(s rowScanner) (*Site, error) {
	var site Site
	err := s.Scan(&site.SiteID, &site.SiteName, &site.HomeURL, &site.LoginURL, &site.CheckinURL,
		&site.ReadingURL, &site.CheckinButtonRule, &site.Status, &site.Remark,
		&site.CreatedAt, &site.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &site, nil
}

func (s *Store) scanAccount(scanner rowScanner, revealSensitive bool) (*Account, error) {
	var account Account
	var passwordStored string
	var emailPasswordStored string
	var autoCheckin int
	var autoRead int
	err := scanner.Scan(&account.AccountID, &account.SiteID, &account.SiteName, &account.ProfileID, &account.ProfileName,
		&account.Username, &passwordStored, &account.Email, &emailPasswordStored, &account.AccountURL,
		&autoCheckin, &account.CheckinURL, &account.CheckinButtonRule, &autoRead, &account.ReadingURL,
		&account.Status, &account.Remark, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		return nil, err
	}
	account.AutoCheckinEnabled = autoCheckin == 1
	account.AutoReadEnabled = autoRead == 1
	account.HasPassword = strings.TrimSpace(passwordStored) != ""
	account.HasEmailPassword = strings.TrimSpace(emailPasswordStored) != ""
	if revealSensitive {
		password, err := s.secrets.decrypt(passwordStored)
		if err != nil {
			return nil, err
		}
		emailPassword, err := s.secrets.decrypt(emailPasswordStored)
		if err != nil {
			return nil, err
		}
		account.Password = password
		account.EmailPassword = emailPassword
	}
	return &account, nil
}

func normalizeSiteInput(input SiteInput) (*Site, error) {
	site := &Site{
		SiteID:            strings.TrimSpace(input.SiteID),
		SiteName:          strings.TrimSpace(input.SiteName),
		HomeURL:           strings.TrimSpace(input.HomeURL),
		LoginURL:          strings.TrimSpace(input.LoginURL),
		CheckinURL:        strings.TrimSpace(input.CheckinURL),
		ReadingURL:        strings.TrimSpace(input.ReadingURL),
		CheckinButtonRule: strings.TrimSpace(input.CheckinButtonRule),
		Status:            normalizeSiteStatus(input.Status),
		Remark:            strings.TrimSpace(input.Remark),
	}
	if site.SiteName == "" {
		return nil, fmt.Errorf("站点名称不能为空")
	}
	for label, value := range map[string]string{
		"主页 URL": site.HomeURL, "登录 URL": site.LoginURL, "签到 URL": site.CheckinURL, "阅读 URL": site.ReadingURL,
	} {
		if err := validateHTTPURL(label, value); err != nil {
			return nil, err
		}
	}
	return site, nil
}

func (s *Store) normalizeAccountInput(input AccountInput) (*Account, error) {
	account := &Account{
		AccountID:          strings.TrimSpace(input.AccountID),
		SiteID:             strings.TrimSpace(input.SiteID),
		ProfileID:          strings.TrimSpace(input.ProfileID),
		Username:           strings.TrimSpace(input.Username),
		Password:           input.Password,
		Email:              strings.TrimSpace(input.Email),
		EmailPassword:      input.EmailPassword,
		AccountURL:         strings.TrimSpace(input.AccountURL),
		AutoCheckinEnabled: input.AutoCheckinEnabled,
		CheckinURL:         strings.TrimSpace(input.CheckinURL),
		CheckinButtonRule:  strings.TrimSpace(input.CheckinButtonRule),
		AutoReadEnabled:    input.AutoReadEnabled,
		ReadingURL:         strings.TrimSpace(input.ReadingURL),
		Status:             normalizeAccountStatus(input.Status),
		Remark:             strings.TrimSpace(input.Remark),
	}
	if account.SiteID == "" {
		return nil, fmt.Errorf("请选择站点")
	}
	if account.ProfileID == "" {
		return nil, fmt.Errorf("请选择指纹浏览器")
	}
	if account.Username == "" {
		return nil, fmt.Errorf("账号用户名不能为空")
	}
	for label, value := range map[string]string{
		"账号 URL": account.AccountURL, "签到 URL": account.CheckinURL, "阅读 URL": account.ReadingURL,
	} {
		if err := validateHTTPURL(label, value); err != nil {
			return nil, err
		}
	}
	if _, err := s.GetSite(account.SiteID); err != nil {
		return nil, err
	}
	return account, nil
}

func normalizeSiteStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "disabled":
		return "disabled"
	default:
		return "enabled"
	}
}

func normalizeAccountStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "paused", "invalid", "archived":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "active"
	}
}

func validateHTTPURL(label string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("%s 格式无效", label)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%s 仅支持 http/https", label)
	}
	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
