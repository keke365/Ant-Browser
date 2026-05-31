import type { BrowserExtension } from '../types'
import { getBindings } from './runtime'

export async function fetchBrowserExtensions(): Promise<BrowserExtension[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionList) {
    return (await bindings.BrowserExtensionList()) || []
  }
  return []
}

export async function importBrowserExtensionPackage(): Promise<BrowserExtension | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionImportLocalPackage) {
    return (await bindings.BrowserExtensionImportLocalPackage()) || null
  }
  return null
}

export async function importBrowserExtensionDirectory(): Promise<BrowserExtension | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionImportLocalDirectory) {
    return (await bindings.BrowserExtensionImportLocalDirectory()) || null
  }
  return null
}

export async function importBrowserExtensionChromeWebStoreURL(url: string): Promise<BrowserExtension | null> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionImportChromeWebStoreURL) {
    return (await bindings.BrowserExtensionImportChromeWebStoreURL(url)) || null
  }
  return null
}

export async function setBrowserExtensionEnabled(extensionId: string, enabled: boolean): Promise<BrowserExtension> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionSetEnabled) {
    return await bindings.BrowserExtensionSetEnabled(extensionId, enabled)
  }
  throw new Error('当前运行环境不支持插件管理')
}

export async function deleteBrowserExtension(extensionId: string, removeFiles = true): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionDelete) {
    await bindings.BrowserExtensionDelete(extensionId, removeFiles)
    return true
  }
  return false
}

export async function openBrowserExtensionPath(extensionId: string): Promise<boolean> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserExtensionOpen) {
    await bindings.BrowserExtensionOpen(extensionId)
    return true
  }
  return false
}
