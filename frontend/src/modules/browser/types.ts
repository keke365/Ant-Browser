export interface BrowserProfile {
  profileId: string
  profileName: string
  userDataDir: string
  coreId: string
  fingerprintArgs: string[]
  proxyId: string
  proxyConfig: string
  proxyBindSourceId?: string
  proxyBindSourceUrl?: string
  proxyBindName?: string
  proxyBindUpdatedAt?: string
  launchArgs: string[]
  tags: string[]
  keywords: string[]
  groupId?: string
  running: boolean
  debugPort: number
  debugReady: boolean
  pid: number
  runtimeWarning: string
  lastError: string
  createdAt: string
  updatedAt: string
  lastStartAt?: string
  lastStopAt?: string
  launchCode?: string
}

export interface BrowserProfileInput {
  profileName: string
  userDataDir: string
  coreId: string
  fingerprintArgs: string[]
  proxyId: string
  proxyConfig: string
  launchArgs: string[]
  tags: string[]
  keywords: string[]
  groupId?: string
}

export interface BrowserTab {
  tabId: string
  title: string
  url: string
  active: boolean
}

export interface BrowserSettings {
  userDataRoot: string
  defaultFingerprintArgs: string[]
  defaultLaunchArgs: string[]
  defaultStartUrls: string[]
  restoreLastSession: boolean
  startReadyTimeoutMs: number
  startStableWindowMs: number
}

export interface ProxyCheckTarget {
  id: string
  name: string
  type: string
  url: string
  parser?: string
  timeoutMs?: number
  expectedStatus?: number[]
}

export interface ProxyCheckSettings {
  bridgeStartTimeoutMs: number
  speedTargetId: string
  ipHealthTargetId: string
  targets: ProxyCheckTarget[]
}

export interface BrowserCore {
  coreId: string
  coreName: string
  corePath: string
  isDefault: boolean
}

export interface BrowserCoreInput {
  coreId: string
  coreName: string
  corePath: string
  isDefault: boolean
}

export interface BrowserCoreValidateResult {
  valid: boolean
  message: string
}

export interface BrowserProxy {
  proxyId: string
  proxyName: string
  proxyConfig: string
  dnsServers?: string
  groupName?: string
  sourceId?: string
  sourceUrl?: string
  sourceNamePrefix?: string
  sourceAutoRefresh?: boolean
  sourceRefreshIntervalM?: number
  sourceLastRefreshAt?: string
  lastLatencyMs?: number
  lastTestOk?: boolean
  lastTestedAt?: string
  lastIPHealthJson?: string
}

export interface ProxySourceSummary {
  sourceId: string
  sourceUrl: string
  sourceNamePrefix: string
  groupName: string
  dnsServers: string
  sourceAutoRefresh: boolean
  sourceRefreshIntervalM: number
  sourceLastRefreshAt: string
  proxyCount: number
}

export interface ProxyImportClashRequest {
  sourceId?: string
  sourceUrl?: string
  content: string
  namePrefix?: string
  groupName?: string
  dnsServers?: string
  sourceAutoRefresh?: boolean
  sourceRefreshIntervalM?: number
  keepRemoved?: boolean
  keepNames?: string[]
  skipNames?: string[]
}

export interface ProxySourceRefreshRequest {
  sourceId: string
  sourceAutoRefresh?: boolean
  sourceRefreshIntervalM?: number
  keepRemoved?: boolean
  keepNames?: string[]
  skipNames?: string[]
}

export interface ProxyImportReport {
  sourceId: string
  sourceUrl: string
  added: number
  updated: number
  removed: number
  skipped: number
  failed: number
  affectedProfileCount: number
  reboundProfileCount: number
  invalidProfileCount: number
  importedProxies: BrowserProxy[]
  skippedProxyNames: string[]
  unsupportedProxyNames: string[]
  errors: string[]
}

export interface ProxyReconcileReport {
  changedProfileCount: number
  reboundProfileCount: number
  invalidProfileCount: number
  invalidProfileIds: string[]
}

export interface ProxyIPHealthResult {
  proxyId: string
  ok: boolean
  source: string
  error: string
  ip: string
  fraudScore: number
  isResidential: boolean
  isBroadcast: boolean
  country: string
  region: string
  city: string
  asOrganization: string
  rawData: Record<string, any>
  updatedAt: string
}

export interface BrowserCoreExtended {
  coreId: string
  chromeVersion: string
  instanceCount: number
}

export interface CookieInfo {
  name: string
  value: string
  domain: string
  path: string
  expires: number
  httpOnly: boolean
  secure: boolean
  sameSite: string
}

export interface SnapshotInfo {
  snapshotId: string
  profileId: string
  name: string
  sizeMB: number
  createdAt: string
}

export interface BrowserBookmark {
  name: string
  url: string
  openOnStart?: boolean
}

export interface BrowserExtension {
  extensionId: string
  name: string
  description?: string
  version?: string
  manifestVersion?: number
  installPath: string
  sourcePath?: string
  sourceType?: string
  enabled: boolean
  importedAt: string
  updatedAt: string
  pathExists: boolean
  iconDataUrl?: string
}

export interface BookmarkSyncResult {
  total: number
  synced: number
  skipped: number
  failed: number
  skippedList: string[]
  failedList: string[]
}


// 分组相关类型
export interface BrowserGroup {
  groupId: string
  groupName: string
  parentId: string
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface BrowserGroupInput {
  groupName: string
  parentId: string
  sortOrder: number
}

export interface BrowserGroupWithCount extends BrowserGroup {
  instanceCount: number
}

export interface Site {
  siteId: string
  siteName: string
  homeUrl: string
  loginUrl: string
  checkinUrl: string
  readingUrl: string
  checkinButtonRule: string
  status: 'enabled' | 'disabled' | string
  remark: string
  createdAt: string
  updatedAt: string
}

export interface SiteSummary extends Site {
  accountCount: number
  autoCheckinCount: number
  autoReadCount: number
}

export interface SiteInput {
  siteId?: string
  siteName: string
  homeUrl: string
  loginUrl: string
  checkinUrl: string
  readingUrl: string
  checkinButtonRule: string
  status: 'enabled' | 'disabled' | string
  remark: string
}

export interface SiteAccount {
  accountId: string
  siteId: string
  siteName?: string
  profileId: string
  profileName?: string
  username: string
  password?: string
  hasPassword: boolean
  email: string
  emailPassword?: string
  hasEmailPassword: boolean
  accountUrl: string
  autoCheckinEnabled: boolean
  checkinUrl: string
  checkinButtonRule: string
  autoReadEnabled: boolean
  readingUrl: string
  status: 'active' | 'paused' | 'invalid' | 'archived' | string
  remark: string
  createdAt: string
  updatedAt: string
}

export interface SiteAccountInput {
  accountId?: string
  siteId: string
  profileId: string
  username: string
  password: string
  clearPassword?: boolean
  email: string
  emailPassword: string
  clearEmailPassword?: boolean
  accountUrl: string
  autoCheckinEnabled: boolean
  checkinUrl: string
  checkinButtonRule: string
  autoReadEnabled: boolean
  readingUrl: string
  status: 'active' | 'paused' | 'invalid' | 'archived' | string
  remark: string
}

export interface SiteAccountFilter {
  siteId?: string
  profileId?: string
  status?: string
  keyword?: string
  autoCheckin?: string
  autoRead?: string
}

export interface SiteAccountTaskRun {
  runId: string
  taskType: string
  siteId: string
  accountId: string
  profileId: string
  status: string
  summary: string
  error: string
  startedAt: string
  finishedAt: string
  durationMs: number
  artifactPath: string
}

export interface SiteAccountCheckinRequest {
  accountId?: string
  siteId?: string
  profileId?: string
  filter?: SiteAccountFilter
  concurrency?: number
}

export interface SiteAccountCheckinBatchResult {
  total: number
  succeeded: number
  failed: number
  runs: SiteAccountTaskRun[]
}
