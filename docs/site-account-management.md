# Site Account Management

## Background

Ant Browser already manages isolated browser profiles, proxy bindings, launch codes, and Playwright-based automation scripts. The new site account management feature should add a business layer above browser profiles:

- A site can contain multiple accounts.
- Each account is bound to one browser profile.
- The same browser profile should not be reused for multiple accounts under the same site.
- Users can quickly open the target site with the bound browser profile.
- Later automation tasks can read pages and check in with the correct account environment.

This feature should not merge account fields into the existing browser profile model. Browser profiles remain the isolated runtime environment; site accounts describe which real-world account is using that environment.

## Goals

- Manage sites and their accounts in one place.
- Bind each site account to a fingerprint browser profile.
- Prevent accidental profile reuse within the same site.
- Support filtering accounts by browser profile.
- Provide quick-open actions for account-related URLs.
- Provide a foundation for automatic reading and automatic check-in tasks.
- Record automation results so failures can be diagnosed.

## Non-Goals For The First Phase

- Full automatic login across all websites.
- Universal visual AI recognition for every check-in button.
- High-concurrency automation across a large number of browser profiles.
- Clash subscription and proxy node lifecycle management. That belongs to the proxy IP document.

## Suggested Phases

### Phase 1: Site And Account CRUD

- Add, edit, delete, and list sites.
- Add, edit, delete, and list accounts under a site.
- Bind one account to one browser profile.
- Enforce unique binding for `siteId + profileId`.
- Filter accounts by site, profile, keyword, status, and auto-check-in setting.
- Quick-open a site's homepage, login page, or account-specific URL with the bound browser profile.

### Phase 2: Automatic Check-In

- Allow an account to enable automatic check-in.
- Store the check-in URL on the site or account.
- Support user-defined check-in button rules.
- Execute check-in in a background task queue.
- Save task records with status, time, error message, and optional artifact paths.

### Phase 3: Automatic Reading

- Allow site-level or account-level reading configuration.
- Simulate reading behavior with controlled scrolling, waits, and page transitions.
- Limit duration, page count, and retry count.
- Save reading task records.

### Phase 4: Smarter Recognition And Scheduling

- Add optional automatic check-in button recognition by text and common patterns.
- Add scheduling rules such as daily windows, random delay, and cooldown.
- Add bulk operations and run reports.

## Data Model

### `sites`

Recommended fields:

| Field | Description |
| --- | --- |
| `siteId` | Unique site ID |
| `siteName` | Display name |
| `homeUrl` | Default homepage |
| `loginUrl` | Optional login page |
| `checkinUrl` | Default check-in page |
| `readingUrl` | Default reading entry page |
| `checkinButtonRule` | Default check-in button rule |
| `status` | `enabled` or `disabled` |
| `remark` | User notes |
| `createdAt` | Creation time |
| `updatedAt` | Update time |

### `site_accounts`

Recommended fields:

| Field | Description |
| --- | --- |
| `accountId` | Unique account ID |
| `siteId` | Related site ID |
| `profileId` | Bound browser profile ID |
| `username` | Login username |
| `password` | Login password, stored securely |
| `email` | Bound email |
| `emailPassword` | Email password, stored securely |
| `accountUrl` | Optional account-specific URL |
| `autoCheckinEnabled` | Whether automatic check-in is enabled |
| `checkinUrl` | Account-level override for check-in URL |
| `checkinButtonRule` | Account-level override for the check-in button rule |
| `autoReadEnabled` | Whether automatic reading is enabled |
| `readingUrl` | Account-level override for reading URL |
| `status` | `active`, `paused`, `invalid`, or `archived` |
| `remark` | User notes |
| `createdAt` | Creation time |
| `updatedAt` | Update time |

Recommended indexes and constraints:

- Unique index: `siteId + profileId`.
- Index: `profileId`, for filtering all accounts bound to one browser profile.
- Index: `siteId`, for listing accounts under a site.

### `site_account_task_runs`

Recommended fields:

| Field | Description |
| --- | --- |
| `runId` | Unique run ID |
| `taskType` | `checkin` or `reading` |
| `siteId` | Related site ID |
| `accountId` | Related account ID |
| `profileId` | Bound browser profile ID at execution time |
| `status` | `queued`, `running`, `success`, `failed`, `cancelled` |
| `summary` | Short execution summary |
| `error` | Failure reason |
| `startedAt` | Start time |
| `finishedAt` | Finish time |
| `durationMs` | Duration |
| `artifactPath` | Optional screenshot/log directory |

## Password And Sensitive Data

Account passwords and email passwords are sensitive data. Requirements should include:

- Store sensitive fields encrypted at rest.
- Mask sensitive fields in UI by default.
- Never write raw passwords to logs, task summaries, or automation artifacts.
- Require explicit user confirmation when exporting backups containing sensitive fields.
- Consider allowing users to disable password storage and only store usernames/notes.

## User Flows

### Create A Site

1. User opens the site manager.
2. User creates a site with name and homepage URL.
3. User optionally fills login URL, check-in URL, reading URL, and default check-in button rule.
4. System validates URL format and saves the site.

### Create An Account

1. User opens a site detail page.
2. User adds an account.
3. User selects a browser profile.
4. System rejects the selection if the same profile is already bound to another account under this site.
5. User fills username, password, email, email password, and notes as needed.
6. User optionally enables automatic check-in and automatic reading.

### Quick Open

1. User clicks quick open on an account row.
2. System starts the bound browser profile if it is not running.
3. System opens the selected URL:
   - account URL if configured;
   - otherwise site homepage;
   - or login/check-in/reading URL according to the button action.

### Filter By Browser Profile

1. User selects or searches a browser profile.
2. System lists all site accounts bound to that profile.
3. User can open the site, edit account data, run check-in, or run reading tasks.

## Automatic Check-In

### Button Rule Types

Support user-defined rules first:

| Rule Type | Example | Notes |
| --- | --- | --- |
| CSS selector | `button.checkin` | Most stable when users know the page structure |
| Text contains | `签到` | Useful for simple sites |
| XPath | `//button[contains(., "签到")]` | Advanced fallback |

Automatic recognition can be added later by checking common text such as:

- `签到`
- `打卡`
- `领取`
- `Check in`
- `Daily reward`

### Execution Strategy

Automatic check-in should use a background queue:

- Limit concurrent browser profiles, default 1 or 2.
- Reuse already running profiles when possible.
- Start stopped profiles only when their task begins.
- Add random delay between accounts to avoid synchronized behavior.
- Apply per-task timeout.
- Retry failed tasks only a limited number of times.
- Allow pause, cancel, and resume of the whole batch.
- Optionally close profiles after task completion if they were opened only for automation.

This avoids the performance issue caused by constantly switching or launching many browsers at once.

### Result Handling

Each task run should store:

- Success or failure.
- Matched button rule.
- Final page URL.
- Error reason.
- Optional screenshot on failure.
- Start and finish time.

## Automatic Reading

Automatic reading should be configurable and bounded:

- Entry URL: site-level or account-level.
- Max duration per account.
- Max pages per account.
- Scroll distance and wait interval ranges.
- Optional click next article/list item behavior.
- Stop conditions: timeout, page count reached, repeated failures, or browser closed.

The first implementation should avoid pretending to understand all site content. It should provide controlled, repeatable browser behavior.

## UI Requirements

Suggested navigation:

- New sidebar entry: `站点账号`.
- Site list page.
- Site detail page with account table.
- Profile-centric account view.
- Task run history page or drawer.

Account table columns:

- Site name.
- Username.
- Bound browser profile.
- Email.
- Auto check-in status.
- Auto reading status.
- Last check-in result.
- Last reading result.
- Remark.
- Actions: quick open, open login page, run check-in, run reading, edit, delete.

## Backend API Requirements

Suggested Wails APIs:

- `SiteList`
- `SiteGet`
- `SiteSave`
- `SiteDelete`
- `SiteAccountList`
- `SiteAccountGet`
- `SiteAccountSave`
- `SiteAccountDelete`
- `SiteAccountListByProfile`
- `SiteAccountQuickOpen`
- `SiteAccountRunCheckin`
- `SiteAccountRunReading`
- `SiteAccountTaskRunList`
- `SiteAccountTaskRunCancel`

## Validation Rules

- Site name cannot be empty.
- Site homepage URL should be valid when provided.
- Account must reference an existing site.
- Account must reference an existing browser profile.
- `siteId + profileId` must be unique.
- Check-in task requires a check-in URL.
- Reading task requires a reading URL.
- User-defined selector rules should be validated before saving when possible.

## Open Questions

- Should the unique binding rule be per site or global across all sites? Recommended: per site.
- Should account passwords be mandatory, optional, or disabled by default?
- Should automatic tasks run only on demand first, or include schedules in phase 2?
- Should a browser profile opened by automation remain open after the task?
- Should task runs be stored in SQLite or follow the existing automation run file-store pattern?

## Acceptance Criteria

Phase 1 is complete when:

- Users can create sites.
- Users can create multiple accounts under one site.
- Users cannot bind the same browser profile to two accounts under the same site.
- Users can filter accounts by browser profile.
- Users can quick-open a configured URL with the bound browser profile.
- Sensitive fields are masked in UI and excluded from logs.

Phase 2 is complete when:

- Users can enable automatic check-in for selected accounts.
- Users can configure a check-in URL and button rule.
- Users can run check-in for one account, one site, or selected accounts.
- Runs execute through a bounded queue.
- Each run has a visible result record.
