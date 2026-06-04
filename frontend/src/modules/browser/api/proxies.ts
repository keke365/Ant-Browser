import type {
  BrowserProxy,
  ProxyIPHealthResult,
  ProxyImportClashRequest,
  ProxyImportReport,
  ProxyReconcileReport,
  ProxySourceRefreshRequest,
  ProxySourceSummary,
} from '../types'
import { getBindings, getGoApp, getMockProxies, nowISOString, setMockProxies } from './runtime'

export interface ClashImportURLResult {
  url: string
  content: string
  proxyCount: number
  dnsServers?: string
  suggestedGroup?: string
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export async function fetchBrowserProxies(): Promise<BrowserProxy[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyList) {
    return (await bindings.BrowserProxyList()) || []
  }
  return getMockProxies()
}

export async function fetchBrowserProxyGroups(): Promise<string[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyListGroups) {
    return (await bindings.BrowserProxyListGroups()) || []
  }
  return []
}

export async function fetchBrowserProxiesByGroup(groupName: string): Promise<BrowserProxy[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyListByGroup) {
    return (await bindings.BrowserProxyListByGroup(groupName)) || []
  }
  return getMockProxies().filter((proxy) => proxy.groupName === groupName)
}

export async function fetchBrowserProxySources(): Promise<ProxySourceSummary[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyListSources) {
    return (await bindings.BrowserProxyListSources()) || []
  }
  const sourceMap = new Map<string, ProxySourceSummary>()
  getMockProxies().forEach((proxy) => {
    const sourceId = (proxy.sourceId || '').trim()
    const sourceUrl = (proxy.sourceUrl || '').trim()
    if (!sourceId || !sourceUrl) return
    const existing = sourceMap.get(sourceId)
    if (!existing) {
      sourceMap.set(sourceId, {
        sourceId,
        sourceUrl,
        sourceNamePrefix: proxy.sourceNamePrefix || '',
        groupName: proxy.groupName || '',
        dnsServers: proxy.dnsServers || '',
        sourceAutoRefresh: !!proxy.sourceAutoRefresh,
        sourceRefreshIntervalM: proxy.sourceRefreshIntervalM || 0,
        sourceLastRefreshAt: proxy.sourceLastRefreshAt || '',
        proxyCount: 1,
      })
      return
    }
    existing.proxyCount += 1
    if ((proxy.sourceLastRefreshAt || '') > existing.sourceLastRefreshAt) {
      existing.sourceLastRefreshAt = proxy.sourceLastRefreshAt || ''
    }
  })
  return Array.from(sourceMap.values())
}

export async function fetchClashImportFromURL(targetURL: string): Promise<ClashImportURLResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyFetchClashByURL) {
    return (
      (await bindings.BrowserProxyFetchClashByURL(targetURL)) || {
        url: targetURL,
        content: '',
        proxyCount: 0,
      }
    )
  }

  const goApp = getGoApp()
  if (goApp?.BrowserProxyFetchClashByURL) {
    return (
      (await goApp.BrowserProxyFetchClashByURL(targetURL)) || {
        url: targetURL,
        content: '',
        proxyCount: 0,
      }
    )
  }

  throw new Error('当前环境不支持 URL 导入 Clash 配置')
}

export async function importClashProxies(input: ProxyImportClashRequest): Promise<ProxyImportReport> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyImportClash) {
    return (await bindings.BrowserProxyImportClash(input)) || createEmptyImportReport(input.sourceId || '', input.sourceUrl || '')
  }
  throw new Error('当前环境不支持后端导入 Clash 订阅')
}

export async function refreshClashProxySource(input: ProxySourceRefreshRequest): Promise<ProxyImportReport> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyRefreshSource) {
    return (await bindings.BrowserProxyRefreshSource(input)) || createEmptyImportReport(input.sourceId, '')
  }
  throw new Error('当前环境不支持后端刷新 Clash 订阅')
}

export async function reconcileBrowserProxyBindings(): Promise<ProxyReconcileReport> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyReconcileBindings) {
    return (
      (await bindings.BrowserProxyReconcileBindings()) || {
        changedProfileCount: 0,
        reboundProfileCount: 0,
        invalidProfileCount: 0,
        invalidProfileIds: [],
      }
    )
  }
  return {
    changedProfileCount: 0,
    reboundProfileCount: 0,
    invalidProfileCount: 0,
    invalidProfileIds: [],
  }
}

function createEmptyImportReport(sourceId: string, sourceUrl: string): ProxyImportReport {
  return {
    sourceId,
    sourceUrl,
    added: 0,
    updated: 0,
    removed: 0,
    skipped: 0,
    failed: 0,
    affectedProfileCount: 0,
    reboundProfileCount: 0,
    invalidProfileCount: 0,
    importedProxies: [],
    skippedProxyNames: [],
    unsupportedProxyNames: [],
    errors: [],
  }
}

export async function saveBrowserProxies(proxies: BrowserProxy[]): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.SaveBrowserProxies) {
    await bindings.SaveBrowserProxies(proxies)
    return true
  }
  setMockProxies(proxies)
  return true
}

export async function validateProxyConfig(proxyConfig: string, proxyId: string): Promise<{ supported: boolean; errorMsg: string }> {
  const bindings: any = await getBindings()
  if (bindings?.ValidateProxyConfig) {
    return (await bindings.ValidateProxyConfig(proxyConfig, proxyId)) || { supported: true, errorMsg: '' }
  }
  return { supported: true, errorMsg: '' }
}

export async function testProxyConnectivity(proxyId: string, proxyConfig: string): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }> {
  const bindings: any = await getBindings()
  if (bindings?.TestProxyConnectivity) {
    return (await bindings.TestProxyConnectivity(proxyId, proxyConfig)) || { proxyId, ok: false, latencyMs: 0, error: '调用失败' }
  }
  await sleep(300 + Math.random() * 500)
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 200), error: '' }
}

export async function testProxyRealConnectivity(proxyId: string): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }> {
  const bindings: any = await getBindings()
  if (bindings?.TestProxyRealConnectivity) {
    return (await bindings.TestProxyRealConnectivity(proxyId)) || { proxyId, ok: false, latencyMs: 0, error: '调用失败' }
  }
  await sleep(300 + Math.random() * 500)
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), error: '' }
}

export async function browserProxyTestSpeed(proxyId: string): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyTestSpeed) {
    return (await bindings.BrowserProxyTestSpeed(proxyId)) || { proxyId, ok: false, latencyMs: 0, error: '调用失败' }
  }
  await sleep(300 + Math.random() * 500)
  return { proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), error: '' }
}

export async function browserProxyBatchTestSpeed(proxyIds: string[], concurrency: number = 20): Promise<{ proxyId: string; ok: boolean; latencyMs: number; error: string }[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyBatchTestSpeed) {
    return (await bindings.BrowserProxyBatchTestSpeed(proxyIds, concurrency)) || []
  }
  await sleep(1000)
  return proxyIds.map((proxyId) => ({ proxyId, ok: true, latencyMs: Math.floor(100 + Math.random() * 400), error: '' }))
}

export async function browserProxyCheckIPHealth(proxyId: string): Promise<ProxyIPHealthResult> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyCheckIPHealth) {
    return (
      (await bindings.BrowserProxyCheckIPHealth(proxyId)) || {
        proxyId,
        ok: false,
        source: 'ip_health',
        error: '调用失败',
        ip: '',
        fraudScore: 0,
        isResidential: false,
        isBroadcast: false,
        country: '',
        region: '',
        city: '',
        asOrganization: '',
        rawData: {},
        updatedAt: nowISOString(),
      }
    )
  }

  await sleep(600)
  return {
    proxyId,
    ok: true,
    source: 'ip_health',
    error: '',
    ip: '127.0.0.1',
    fraudScore: Math.floor(Math.random() * 100),
    isResidential: Math.random() > 0.5,
    isBroadcast: false,
    country: 'Mock',
    region: 'Mock',
    city: 'Mock',
    asOrganization: 'Mock ISP',
    rawData: {},
    updatedAt: nowISOString(),
  }
}

export async function browserProxyBatchCheckIPHealth(proxyIds: string[], concurrency: number = 10): Promise<ProxyIPHealthResult[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserProxyBatchCheckIPHealth) {
    return (await bindings.BrowserProxyBatchCheckIPHealth(proxyIds, concurrency)) || []
  }

  await sleep(1200)
  return proxyIds.map((proxyId) => ({
    proxyId,
    ok: true,
    source: 'ip_health',
    error: '',
    ip: '127.0.0.1',
    fraudScore: Math.floor(Math.random() * 100),
    isResidential: Math.random() > 0.5,
    isBroadcast: false,
    country: 'Mock',
    region: 'Mock',
    city: 'Mock',
    asOrganization: 'Mock ISP',
    rawData: {},
    updatedAt: nowISOString(),
  }))
}
