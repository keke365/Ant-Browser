package backend

import (
	"ant-chrome/backend/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) backupMergeProxiesFile(payloadRoot string, resetFirst bool, stats *backupMergeStats) error {
	srcPath := filepath.Join(payloadRoot, "system", "proxies.yaml")
	dstPath := a.resolveAppPath("proxies.yaml")

	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			if resetFirst {
				_ = os.Remove(dstPath)
			}
			return nil
		}
		return err
	}

	if resetFirst {
		return backupCopyFile(srcPath, dstPath)
	}

	incoming, err := config.LoadProxies(srcPath)
	if err != nil {
		return err
	}
	current, err := config.LoadProxies(dstPath)
	if err != nil {
		return err
	}

	merged := append([]config.BrowserProxy{}, current...)
	existingID := make(map[string]struct{}, len(current))
	existingCfg := make(map[string]struct{}, len(current))
	for _, p := range current {
		existingID[strings.ToLower(strings.TrimSpace(p.ProxyId))] = struct{}{}
		existingCfg[strings.ToLower(strings.TrimSpace(p.ProxyConfig))] = struct{}{}
	}
	for _, p := range incoming {
		idKey := strings.ToLower(strings.TrimSpace(p.ProxyId))
		cfgKey := strings.ToLower(strings.TrimSpace(p.ProxyConfig))
		if _, ok := existingID[idKey]; ok {
			stats.Skipped++
			continue
		}
		if cfgKey != "" {
			if _, ok := existingCfg[cfgKey]; ok {
				stats.Skipped++
				continue
			}
		}
		merged = append(merged, p)
		existingID[idKey] = struct{}{}
		if cfgKey != "" {
			existingCfg[cfgKey] = struct{}{}
		}
		stats.Imported++
	}

	return config.SaveProxies(dstPath, merged)
}

func backupFindDatabaseFile(payloadRoot string) string {
	candidates := []string{
		filepath.Join(payloadRoot, "app", "database", "app.db"),
		filepath.Join(payloadRoot, "app", "data", "app.db"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func (a *App) backupMergeDatabaseFromSource(srcDBPath string, resetFirst bool, stats *backupMergeStats) error {
	if a.db == nil || a.db.GetConn() == nil {
		return fmt.Errorf("数据库未初始化")
	}
	tx, err := a.db.GetConn().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`ATTACH DATABASE ? AS src`, srcDBPath); err != nil {
		return fmt.Errorf("挂载备份数据库失败: %w", err)
	}
	defer tx.Exec(`DETACH DATABASE src`)

	mergeTables := []struct {
		name       string
		insertAll  string
		insertSafe string
	}{
		{
			name: "sites",
			insertAll: `INSERT INTO sites (site_id, site_name, home_url, login_url, checkin_url, reading_url, checkin_button_rule, status, remark, created_at, updated_at)
SELECT site_id, site_name, home_url, login_url, checkin_url, reading_url, checkin_button_rule, status, remark, created_at, updated_at FROM src.sites`,
			insertSafe: `INSERT INTO sites (site_id, site_name, home_url, login_url, checkin_url, reading_url, checkin_button_rule, status, remark, created_at, updated_at)
SELECT s.site_id, s.site_name, s.home_url, s.login_url, s.checkin_url, s.reading_url, s.checkin_button_rule, s.status, s.remark, s.created_at, s.updated_at
FROM src.sites s
WHERE NOT EXISTS (
  SELECT 1 FROM sites t
  WHERE t.site_id = s.site_id OR lower(t.site_name) = lower(s.site_name)
)`,
		},
		{
			name: "site_accounts",
			insertAll: `INSERT INTO site_accounts (account_id, site_id, profile_id, username, password, email, email_password, account_url, auto_checkin_enabled, checkin_url, checkin_button_rule, auto_read_enabled, reading_url, status, remark, created_at, updated_at)
SELECT account_id, site_id, profile_id, username, password, email, email_password, account_url, auto_checkin_enabled, checkin_url, checkin_button_rule, auto_read_enabled, reading_url, status, remark, created_at, updated_at FROM src.site_accounts`,
			insertSafe: `INSERT INTO site_accounts (account_id, site_id, profile_id, username, password, email, email_password, account_url, auto_checkin_enabled, checkin_url, checkin_button_rule, auto_read_enabled, reading_url, status, remark, created_at, updated_at)
SELECT s.account_id, s.site_id, s.profile_id, s.username, s.password, s.email, s.email_password, s.account_url, s.auto_checkin_enabled, s.checkin_url, s.checkin_button_rule, s.auto_read_enabled, s.reading_url, s.status, s.remark, s.created_at, s.updated_at
FROM src.site_accounts s
WHERE EXISTS (
  SELECT 1 FROM sites t WHERE t.site_id = s.site_id
) AND NOT EXISTS (
  SELECT 1 FROM site_accounts t
  WHERE t.account_id = s.account_id OR (t.site_id = s.site_id AND t.profile_id = s.profile_id)
)`,
		},
		{
			name: "site_account_task_runs",
			insertAll: `INSERT INTO site_account_task_runs (run_id, task_type, site_id, account_id, profile_id, status, summary, error, started_at, finished_at, duration_ms, artifact_path)
SELECT run_id, task_type, site_id, account_id, profile_id, status, summary, error, started_at, finished_at, duration_ms, artifact_path FROM src.site_account_task_runs`,
			insertSafe: `INSERT INTO site_account_task_runs (run_id, task_type, site_id, account_id, profile_id, status, summary, error, started_at, finished_at, duration_ms, artifact_path)
SELECT s.run_id, s.task_type, s.site_id, s.account_id, s.profile_id, s.status, s.summary, s.error, s.started_at, s.finished_at, s.duration_ms, s.artifact_path
FROM src.site_account_task_runs s
WHERE NOT EXISTS (
  SELECT 1 FROM site_account_task_runs t WHERE t.run_id = s.run_id
)`,
		},
		{
			name: "browser_groups",
			insertAll: `INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at)
SELECT group_id, group_name, parent_id, sort_order, created_at, updated_at FROM src.browser_groups`,
			insertSafe: `INSERT INTO browser_groups (group_id, group_name, parent_id, sort_order, created_at, updated_at)
SELECT s.group_id, s.group_name, s.parent_id, s.sort_order, s.created_at, s.updated_at
FROM src.browser_groups s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_groups t
  WHERE t.group_id = s.group_id OR (t.parent_id = s.parent_id AND lower(t.group_name) = lower(s.group_name))
)`,
		},
		{
			name: "browser_cores",
			insertAll: `INSERT INTO browser_cores (core_id, core_name, core_path, is_default, sort_order, created_at)
SELECT core_id, core_name, core_path, is_default, sort_order, created_at FROM src.browser_cores`,
			insertSafe: `INSERT INTO browser_cores (core_id, core_name, core_path, is_default, sort_order, created_at)
SELECT s.core_id, s.core_name, s.core_path, s.is_default, s.sort_order, s.created_at
FROM src.browser_cores s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_cores t
  WHERE t.core_id = s.core_id OR lower(t.core_path) = lower(s.core_path)
)`,
		},
		{
			name: "browser_proxies",
			insertAll: `INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, dns_servers, group_name, source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at, last_latency_ms, last_test_ok, last_tested_at, last_ip_health_json, sort_order, created_at)
SELECT proxy_id, proxy_name, proxy_config, dns_servers, COALESCE(group_name,''), COALESCE(source_id,''), COALESCE(source_url,''), COALESCE(source_name_prefix,''), COALESCE(source_auto_refresh,0), COALESCE(source_refresh_interval_m,0), COALESCE(source_last_refresh_at,''), COALESCE(last_latency_ms,-1), COALESCE(last_test_ok,0), COALESCE(last_tested_at,''), COALESCE(last_ip_health_json,''), sort_order, created_at
FROM src.browser_proxies`,
			insertSafe: `INSERT INTO browser_proxies (proxy_id, proxy_name, proxy_config, dns_servers, group_name, source_id, source_url, source_name_prefix, source_auto_refresh, source_refresh_interval_m, source_last_refresh_at, last_latency_ms, last_test_ok, last_tested_at, last_ip_health_json, sort_order, created_at)
SELECT s.proxy_id, s.proxy_name, s.proxy_config, s.dns_servers, COALESCE(s.group_name,''), COALESCE(s.source_id,''), COALESCE(s.source_url,''), COALESCE(s.source_name_prefix,''), COALESCE(s.source_auto_refresh,0), COALESCE(s.source_refresh_interval_m,0), COALESCE(s.source_last_refresh_at,''), COALESCE(s.last_latency_ms,-1), COALESCE(s.last_test_ok,0), COALESCE(s.last_tested_at,''), COALESCE(s.last_ip_health_json,''), s.sort_order, s.created_at
FROM src.browser_proxies s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_proxies t
  WHERE t.proxy_id = s.proxy_id OR lower(t.proxy_config) = lower(s.proxy_config)
)`,
		},
		{
			name: "browser_profiles",
			insertAll: `INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at)
SELECT profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, COALESCE(group_id,''), created_at, updated_at
FROM src.browser_profiles`,
			insertSafe: `INSERT INTO browser_profiles (profile_id, profile_name, user_data_dir, core_id, fingerprint_args, proxy_id, proxy_config, launch_args, tags, keywords, group_id, created_at, updated_at)
SELECT s.profile_id, s.profile_name, s.user_data_dir, s.core_id, s.fingerprint_args, s.proxy_id, s.proxy_config, s.launch_args, s.tags, s.keywords, COALESCE(s.group_id,''), s.created_at, s.updated_at
FROM src.browser_profiles s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_profiles t
  WHERE t.profile_id = s.profile_id OR lower(t.user_data_dir) = lower(s.user_data_dir)
)`,
		},
		{
			name: "browser_bookmarks",
			insertAll: `INSERT INTO browser_bookmarks (name, url, sort_order)
SELECT name, url, sort_order FROM src.browser_bookmarks`,
			insertSafe: `INSERT INTO browser_bookmarks (name, url, sort_order)
SELECT s.name, s.url, s.sort_order
FROM src.browser_bookmarks s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_bookmarks t WHERE lower(t.url) = lower(s.url)
)`,
		},
		{
			name: "launch_codes",
			insertAll: `INSERT INTO launch_codes (profile_id, code, created_at, updated_at)
SELECT profile_id, code, created_at, updated_at FROM src.launch_codes`,
			insertSafe: `INSERT INTO launch_codes (profile_id, code, created_at, updated_at)
SELECT s.profile_id, s.code, s.created_at, s.updated_at
FROM src.launch_codes s
WHERE NOT EXISTS (
  SELECT 1 FROM launch_codes t
  WHERE t.profile_id = s.profile_id OR t.code = s.code
)`,
		},
	}

	for _, item := range mergeTables {
		exists, err := backupSrcTableExists(tx, item.name)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		total, err := backupCountRows(tx, "src."+item.name)
		if err != nil {
			return err
		}
		if total == 0 {
			continue
		}

		sqlText := item.insertAll
		if !resetFirst {
			sqlText = item.insertSafe
		}
		if item.name == "browser_bookmarks" {
			hasOpenOnStart, err := backupSrcColumnExists(tx, item.name, "open_on_start")
			if err != nil {
				return err
			}
			if hasOpenOnStart {
				if resetFirst {
					sqlText = `INSERT INTO browser_bookmarks (name, url, open_on_start, sort_order)
SELECT name, url, COALESCE(open_on_start,0), sort_order FROM src.browser_bookmarks`
				} else {
					sqlText = `INSERT INTO browser_bookmarks (name, url, open_on_start, sort_order)
SELECT s.name, s.url, COALESCE(s.open_on_start,0), s.sort_order
FROM src.browser_bookmarks s
WHERE NOT EXISTS (
  SELECT 1 FROM browser_bookmarks t WHERE lower(t.url) = lower(s.url)
)`
				}
			}
		}
		res, err := tx.Exec(sqlText)
		if err != nil {
			return fmt.Errorf("导入数据表失败(%s): %w", item.name, err)
		}
		affected, _ := res.RowsAffected()
		inserted := int(affected)
		if inserted < 0 {
			inserted = total
		}
		stats.Imported += inserted
		if !resetFirst && total > inserted {
			stats.Skipped += total - inserted
		}
	}

	return tx.Commit()
}
