import type {
  BrowserProfile,
  Site,
  SiteAccount,
  SiteAccountFilter,
  SiteAccountInput,
  SiteAccountCheckinBatchResult,
  SiteAccountCheckinRequest,
  SiteAccountTaskRun,
  SiteInput,
  SiteSummary,
} from '../types'
import { getBindings, nowISOString } from './runtime'

let mockSites: SiteSummary[] = []
let mockAccounts: SiteAccount[] = []

export async function fetchSites(): Promise<SiteSummary[]> {
  const bindings: any = await getBindings()
  if (bindings?.SiteList) {
    return (await bindings.SiteList()) || []
  }
  return mockSites
}

export async function fetchSite(siteId: string): Promise<Site | null> {
  const bindings: any = await getBindings()
  if (bindings?.SiteGet) {
    return (await bindings.SiteGet(siteId)) || null
  }
  return mockSites.find(item => item.siteId === siteId) || null
}

export async function saveSite(input: SiteInput): Promise<Site | null> {
  const bindings: any = await getBindings()
  if (bindings?.SiteSave) {
    return (await bindings.SiteSave(input)) || null
  }

  const now = nowISOString()
  const siteId = input.siteId || `site-${Date.now()}`
  const existing = mockSites.find(item => item.siteId === siteId)
  const next: SiteSummary = {
    siteId,
    siteName: input.siteName,
    homeUrl: input.homeUrl,
    loginUrl: input.loginUrl,
    checkinUrl: input.checkinUrl,
    readingUrl: input.readingUrl,
    checkinButtonRule: input.checkinButtonRule,
    status: input.status || 'enabled',
    remark: input.remark,
    createdAt: existing?.createdAt || now,
    updatedAt: now,
    accountCount: existing?.accountCount || 0,
    autoCheckinCount: existing?.autoCheckinCount || 0,
    autoReadCount: existing?.autoReadCount || 0,
  }
  mockSites = existing ? mockSites.map(item => item.siteId === siteId ? next : item) : [next, ...mockSites]
  return next
}

export async function deleteSite(siteId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.SiteDelete) {
    await bindings.SiteDelete(siteId)
    return true
  }
  mockSites = mockSites.filter(item => item.siteId !== siteId)
  mockAccounts = mockAccounts.filter(item => item.siteId !== siteId)
  return true
}

export async function fetchSiteAccounts(filter: SiteAccountFilter = {}): Promise<SiteAccount[]> {
  const bindings: any = await getBindings()
  if (bindings?.SiteAccountList) {
    return (await bindings.SiteAccountList(filter)) || []
  }
  const keyword = (filter.keyword || '').toLowerCase().trim()
  return mockAccounts.filter(item => {
    if (filter.siteId && item.siteId !== filter.siteId) return false
    if (filter.profileId && item.profileId !== filter.profileId) return false
    if (filter.status && filter.status !== 'all' && item.status !== filter.status) return false
    if (filter.autoCheckin === 'enabled' && !item.autoCheckinEnabled) return false
    if (filter.autoCheckin === 'disabled' && item.autoCheckinEnabled) return false
    if (filter.autoRead === 'enabled' && !item.autoReadEnabled) return false
    if (filter.autoRead === 'disabled' && item.autoReadEnabled) return false
    if (keyword) {
      const text = [item.username, item.email, item.remark, item.siteName, item.profileName].join(' ').toLowerCase()
      if (!text.includes(keyword)) return false
    }
    return true
  })
}

export async function fetchSiteAccount(accountId: string, revealSensitive = false): Promise<SiteAccount | null> {
  const bindings: any = await getBindings()
  if (bindings?.SiteAccountGet) {
    return (await bindings.SiteAccountGet(accountId, revealSensitive)) || null
  }
  return mockAccounts.find(item => item.accountId === accountId) || null
}

export async function saveSiteAccount(input: SiteAccountInput): Promise<SiteAccount | null> {
  const bindings: any = await getBindings()
  if (bindings?.SiteAccountSave) {
    return (await bindings.SiteAccountSave(input)) || null
  }

  const now = nowISOString()
  const accountId = input.accountId || `account-${Date.now()}`
  const site = mockSites.find(item => item.siteId === input.siteId)
  const existing = mockAccounts.find(item => item.accountId === accountId)
  const next: SiteAccount = {
    accountId,
    siteId: input.siteId,
    siteName: site?.siteName || '',
    profileId: input.profileId,
    username: input.username,
    password: '',
    hasPassword: !!input.password || !!existing?.hasPassword,
    email: input.email,
    emailPassword: '',
    hasEmailPassword: !!input.emailPassword || !!existing?.hasEmailPassword,
    accountUrl: input.accountUrl,
    autoCheckinEnabled: input.autoCheckinEnabled,
    checkinUrl: input.checkinUrl,
    checkinButtonRule: input.checkinButtonRule,
    autoReadEnabled: input.autoReadEnabled,
    readingUrl: input.readingUrl,
    status: input.status || 'active',
    remark: input.remark,
    createdAt: existing?.createdAt || now,
    updatedAt: now,
  }
  mockAccounts = existing ? mockAccounts.map(item => item.accountId === accountId ? next : item) : [next, ...mockAccounts]
  return next
}

export async function deleteSiteAccount(accountId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.SiteAccountDelete) {
    await bindings.SiteAccountDelete(accountId)
    return true
  }
  mockAccounts = mockAccounts.filter(item => item.accountId !== accountId)
  return true
}

export async function quickOpenSiteAccount(accountId: string, target: string): Promise<BrowserProfile | null> {
  const bindings: any = await getBindings()
  if (bindings?.SiteAccountQuickOpen) {
    return (await bindings.SiteAccountQuickOpen(accountId, target)) || null
  }
  return null
}

export async function runSiteAccountCheckin(input: SiteAccountCheckinRequest): Promise<SiteAccountCheckinBatchResult> {
  const bindings: any = await getBindings()
  if (bindings?.SiteAccountRunCheckin) {
    return (await bindings.SiteAccountRunCheckin(input)) || { total: 0, succeeded: 0, failed: 0, runs: [] }
  }
  return {
    total: 0,
    succeeded: 0,
    failed: 0,
    runs: [],
  }
}

export async function runSiteAccountReading(input: SiteAccountCheckinRequest): Promise<SiteAccountCheckinBatchResult> {
  const bindings: any = await getBindings()
  if (bindings?.SiteAccountRunReading) {
    return (await bindings.SiteAccountRunReading(input)) || { total: 0, succeeded: 0, failed: 0, runs: [] }
  }
  return {
    total: 0,
    succeeded: 0,
    failed: 0,
    runs: [],
  }
}

export async function fetchSiteAccountTaskRuns(accountId: string, limit = 50): Promise<SiteAccountTaskRun[]> {
  const bindings: any = await getBindings()
  if (bindings?.SiteAccountTaskRunList) {
    return (await bindings.SiteAccountTaskRunList(accountId, limit)) || []
  }
  return []
}
