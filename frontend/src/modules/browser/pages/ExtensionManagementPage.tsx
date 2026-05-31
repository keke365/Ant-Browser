import { useCallback, useEffect, useMemo, useState } from 'react'
import clsx from 'clsx'
import {
  Archive,
  CheckCircle2,
  FolderOpen,
  Grid2X2,
  Link2,
  PackageOpen,
  Plus,
  Puzzle,
  Search,
  Trash2,
  XCircle,
} from 'lucide-react'
import { Badge, Button, ConfirmModal, Input, Modal, Switch, toast } from '../../../shared/components'
import type { BrowserExtension } from '../types'
import {
  deleteBrowserExtension,
  fetchBrowserExtensions,
  importBrowserExtensionChromeWebStoreURL,
  importBrowserExtensionDirectory,
  importBrowserExtensionPackage,
  openBrowserExtensionPath,
  setBrowserExtensionEnabled,
} from '../api'

type ExtensionTab = 'all' | 'enabled' | 'disabled'
type ImportMode = 'package' | 'directory' | 'webstore'

const tabs: Array<{ id: ExtensionTab; label: string }> = [
  { id: 'all', label: '团队插件' },
  { id: 'enabled', label: '已启用' },
  { id: 'disabled', label: '已停用' },
]

const sourceTypeLabels: Record<string, string> = {
  directory: '插件目录',
  zip: 'ZIP 安装包',
  crx: 'CRX 安装包',
  package: '安装包',
  chrome_web_store: 'Chrome 应用商店',
}

function sourceTypeLabel(type?: string) {
  if (!type) return '本地导入'
  return sourceTypeLabels[type] || type
}

function formatExtensionDate(value?: string) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

interface ExtensionIconProps {
  item: BrowserExtension
}

function ExtensionIcon({ item }: ExtensionIconProps) {
  return (
    <div className="flex h-[56px] w-[56px] shrink-0 items-center justify-center overflow-hidden rounded-lg bg-[#2f6df6] text-white shadow-[var(--shadow-sm)]">
      {item.iconDataUrl ? (
        <img src={item.iconDataUrl} alt="" className="h-full w-full object-cover" />
      ) : (
        <Puzzle className="h-7 w-7" />
      )}
    </div>
  )
}

interface ExtensionCardProps {
  item: BrowserExtension
  busy: boolean
  onToggle: (item: BrowserExtension, enabled: boolean) => void
  onOpen: (item: BrowserExtension) => void
  onDelete: (item: BrowserExtension) => void
}

function ExtensionCard({ item, busy, onToggle, onOpen, onDelete }: ExtensionCardProps) {
  const statusVariant: 'warning' | 'success' | 'default' = !item.pathExists ? 'warning' : item.enabled ? 'success' : 'default'
  const statusText = !item.pathExists ? '目录缺失' : item.enabled ? '已启用' : '已停用'

  return (
    <article className="flex h-[236px] flex-col rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-5 shadow-[var(--shadow-sm)] transition-colors hover:border-[var(--color-border-strong)]">
      <div className="flex items-start gap-3">
        <ExtensionIcon item={item} />
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <h3 className="truncate text-sm font-semibold text-[var(--color-text-primary)]" title={item.name}>
                {item.name || '未命名插件'}
              </h3>
              <p className="mt-1 truncate text-xs text-[var(--color-text-muted)]">
                {item.version ? `v${item.version}` : '版本未知'} · {sourceTypeLabel(item.sourceType)}
              </p>
            </div>
            <Switch
              checked={item.enabled}
              disabled={busy || !item.pathExists}
              onChange={(checked) => onToggle(item, checked)}
            />
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <Badge variant={statusVariant} size="sm" dot>
              {statusText}
            </Badge>
            {item.manifestVersion ? (
              <Badge variant="info" size="sm">
                MV{item.manifestVersion}
              </Badge>
            ) : null}
          </div>
        </div>
      </div>

      <p className="mt-4 h-[54px] overflow-hidden text-sm leading-6 text-[var(--color-text-secondary)]">
        {item.description || '该插件未提供描述。'}
      </p>

      <div className="mt-auto border-t border-[var(--color-border-muted)] pt-3">
        <div className="mb-3 truncate text-xs text-[var(--color-text-muted)]" title={item.installPath}>
          {item.installPath || '未记录安装目录'}
        </div>
        <div className="flex items-center justify-between gap-3">
          <span className="truncate text-xs text-[var(--color-text-muted)]">
            {formatExtensionDate(item.importedAt)}
          </span>
          <div className="flex shrink-0 items-center gap-1">
            <button
              type="button"
              onClick={() => onOpen(item)}
              className="rounded-md p-1.5 text-[var(--color-text-muted)] transition-colors hover:bg-[var(--color-accent-muted)] hover:text-[var(--color-text-primary)]"
              title="打开目录"
            >
              <FolderOpen className="h-4 w-4" />
            </button>
            <button
              type="button"
              onClick={() => onDelete(item)}
              className="rounded-md p-1.5 text-[var(--color-text-muted)] transition-colors hover:bg-[var(--color-error)]/10 hover:text-[var(--color-error)]"
              title="删除插件"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
    </article>
  )
}

interface AddExtensionModalProps {
  open: boolean
  importing: boolean
  mode: ImportMode
  webStoreUrl: string
  onModeChange: (mode: ImportMode) => void
  onWebStoreUrlChange: (value: string) => void
  onClose: () => void
  onImport: () => void
}

function AddExtensionModal({
  open,
  importing,
  mode,
  webStoreUrl,
  onModeChange,
  onWebStoreUrlChange,
  onClose,
  onImport,
}: AddExtensionModalProps) {
  const packageMode = mode === 'package'
  const directoryMode = mode === 'directory'
  const webStoreMode = mode === 'webstore'

  return (
    <Modal open={open} onClose={onClose} title="添加插件" width="820px">
      <div className="grid gap-6 py-2 md:grid-cols-[160px_1fr]">
        <div className="pt-2 text-sm text-[var(--color-text-secondary)]">类型</div>
        <div className="inline-flex w-fit rounded-lg bg-[var(--color-bg-muted)] p-1">
          <button
            type="button"
            onClick={() => onModeChange('package')}
            className={clsx(
              'inline-flex h-9 items-center gap-2 rounded-md px-4 text-sm transition-colors',
              packageMode
                ? 'bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] shadow-[var(--shadow-sm)]'
                : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]',
            )}
          >
            <Archive className="h-4 w-4" />
            安装包
          </button>
          <button
            type="button"
            onClick={() => onModeChange('directory')}
            className={clsx(
              'inline-flex h-9 items-center gap-2 rounded-md px-4 text-sm transition-colors',
              directoryMode
                ? 'bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] shadow-[var(--shadow-sm)]'
                : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]',
            )}
          >
            <FolderOpen className="h-4 w-4" />
            插件目录
          </button>
          <button
            type="button"
            onClick={() => onModeChange('webstore')}
            className={clsx(
              'inline-flex h-9 items-center gap-2 rounded-md px-4 text-sm transition-colors',
              webStoreMode
                ? 'bg-[var(--color-bg-elevated)] text-[var(--color-text-primary)] shadow-[var(--shadow-sm)]'
                : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]',
            )}
          >
            <Link2 className="h-4 w-4" />
            商店链接
          </button>
        </div>

        <div className="text-sm text-[var(--color-text-secondary)]">来源</div>
        <div className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-5">
          <div className="flex items-start gap-4">
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-[#2f6df6] text-white">
              {packageMode ? (
                <PackageOpen className="h-6 w-6" />
              ) : webStoreMode ? (
                <Link2 className="h-6 w-6" />
              ) : (
                <FolderOpen className="h-6 w-6" />
              )}
            </div>
            <div className="min-w-0 flex-1">
              <h3 className="text-base font-semibold text-[var(--color-text-primary)]">
                {packageMode
                  ? '选择 ZIP / CRX 插件包'
                  : webStoreMode
                    ? '输入 Chrome 应用商店链接'
                    : '选择含 manifest.json 的插件目录'}
              </h3>
              <p className="mt-1 text-sm text-[var(--color-text-muted)]">
                {packageMode
                  ? '导入后会解包到本地插件库。'
                  : webStoreMode
                    ? '支持 chromewebstore.google.com/detail/... 链接。'
                    : '目录内容会复制到本地插件库。'}
              </p>
              {webStoreMode && (
                <Input
                  value={webStoreUrl}
                  onChange={(event) => onWebStoreUrlChange(event.target.value)}
                  disabled={importing}
                  placeholder="https://chromewebstore.google.com/detail/lapnciffpekdengooeolaienkeoilfeo"
                  className="mt-5"
                />
              )}
              <Button className="mt-5 min-w-[148px]" onClick={onImport} loading={importing}>
                <Plus className="h-4 w-4" />
                {packageMode ? '选择安装包' : webStoreMode ? '导入插件' : '选择目录'}
              </Button>
            </div>
          </div>
        </div>

        <div />
        <div className="flex justify-end gap-3">
          <Button variant="secondary" onClick={onClose} disabled={importing}>
            取消
          </Button>
        </div>
      </div>
    </Modal>
  )
}

export function ExtensionManagementPage() {
  const [items, setItems] = useState<BrowserExtension[]>([])
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState('')
  const [activeTab, setActiveTab] = useState<ExtensionTab>('all')
  const [addOpen, setAddOpen] = useState(false)
  const [importMode, setImportMode] = useState<ImportMode>('package')
  const [webStoreUrl, setWebStoreUrl] = useState('')
  const [importing, setImporting] = useState(false)
  const [busyId, setBusyId] = useState('')
  const [deletingItem, setDeletingItem] = useState<BrowserExtension | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchBrowserExtensions()
      setItems(data)
    } catch (error: any) {
      toast.error(error?.message || '插件列表加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const counts = useMemo(() => {
    return {
      all: items.length,
      enabled: items.filter((item) => item.enabled).length,
      disabled: items.filter((item) => !item.enabled).length,
    }
  }, [items])

  const filteredItems = useMemo(() => {
    const keyword = query.trim().toLowerCase()
    return items.filter((item) => {
      if (activeTab === 'enabled' && !item.enabled) return false
      if (activeTab === 'disabled' && item.enabled) return false
      if (!keyword) return true
      const haystack = [
        item.name,
        item.description,
        item.version,
        item.installPath,
        item.sourcePath,
        sourceTypeLabel(item.sourceType),
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
      return haystack.includes(keyword)
    })
  }, [activeTab, items, query])

  const handleImport = async () => {
    if (importMode === 'webstore' && !webStoreUrl.trim()) {
      toast.error('请输入 Chrome 应用商店链接')
      return
    }
    setImporting(true)
    try {
      const imported =
        importMode === 'package'
          ? await importBrowserExtensionPackage()
          : importMode === 'webstore'
            ? await importBrowserExtensionChromeWebStoreURL(webStoreUrl.trim())
            : await importBrowserExtensionDirectory()
      if (imported) {
        setAddOpen(false)
        if (importMode === 'webstore') setWebStoreUrl('')
        await load()
        toast.success(`已导入 ${imported.name || '插件'}`)
      }
    } catch (error: any) {
      toast.error(error?.message || '导入插件失败')
    } finally {
      setImporting(false)
    }
  }

  const handleToggle = async (item: BrowserExtension, enabled: boolean) => {
    setBusyId(item.extensionId)
    setItems((prev) => prev.map((next) => (next.extensionId === item.extensionId ? { ...next, enabled } : next)))
    try {
      const updated = await setBrowserExtensionEnabled(item.extensionId, enabled)
      setItems((prev) => prev.map((next) => (next.extensionId === item.extensionId ? updated : next)))
      toast.success(enabled ? '插件已启用' : '插件已停用')
    } catch (error: any) {
      await load()
      toast.error(error?.message || '状态更新失败')
    } finally {
      setBusyId('')
    }
  }

  const handleOpen = async (item: BrowserExtension) => {
    try {
      await openBrowserExtensionPath(item.extensionId)
    } catch (error: any) {
      toast.error(error?.message || '打开目录失败')
    }
  }

  const handleDelete = async () => {
    if (!deletingItem) return
    setBusyId(deletingItem.extensionId)
    try {
      await deleteBrowserExtension(deletingItem.extensionId, true)
      setItems((prev) => prev.filter((item) => item.extensionId !== deletingItem.extensionId))
      toast.success('插件已删除')
    } catch (error: any) {
      toast.error(error?.message || '删除失败')
    } finally {
      setBusyId('')
      setDeletingItem(null)
    }
  }

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">插件管理</h1>
          <p className="mt-1 text-sm text-[var(--color-text-muted)]">启用的插件会随每个指纹浏览器实例启动</p>
        </div>
        <Button size="sm" onClick={() => setAddOpen(true)}>
          <Plus className="h-4 w-4" />
          添加插件
        </Button>
      </div>

      <section className="min-h-[620px] rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-subtle)]">
        <div className="flex border-b border-[var(--color-border-default)]">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={clsx(
                'relative px-5 py-4 text-sm font-medium transition-colors',
                activeTab === tab.id
                  ? 'text-[var(--color-text-primary)]'
                  : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)]',
              )}
            >
              {tab.label}
              <span className="ml-2 text-xs text-[var(--color-text-muted)]">{counts[tab.id]}</span>
              {activeTab === tab.id && (
                <span className="absolute inset-x-4 bottom-0 h-0.5 rounded-full bg-[var(--color-accent)]" />
              )}
            </button>
          ))}
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-border-muted)] px-4 py-4">
          <div className="flex items-center gap-3">
            <Button size="sm" onClick={() => setAddOpen(true)}>
              <Plus className="h-4 w-4" />
              添加插件
            </Button>
            <div className="relative w-[260px]">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[var(--color-text-muted)]" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="关键字搜索"
                className="pl-9"
              />
            </div>
          </div>
          <div className="inline-flex h-9 items-center gap-2 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-3 text-sm text-[var(--color-text-secondary)]">
            <Grid2X2 className="h-4 w-4" />
            插件分类
          </div>
        </div>

        <div className="p-4">
          {loading ? (
            <div className="flex h-[320px] items-center justify-center text-sm text-[var(--color-text-muted)]">
              加载中...
            </div>
          ) : filteredItems.length > 0 ? (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(330px,1fr))] gap-4">
              {filteredItems.map((item) => (
                <ExtensionCard
                  key={item.extensionId}
                  item={item}
                  busy={busyId === item.extensionId}
                  onToggle={handleToggle}
                  onOpen={handleOpen}
                  onDelete={setDeletingItem}
                />
              ))}
            </div>
          ) : (
            <div className="flex h-[320px] flex-col items-center justify-center text-center">
              <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-lg bg-[var(--color-bg-surface)] text-[var(--color-text-muted)] shadow-[var(--shadow-sm)]">
                {query ? <XCircle className="h-7 w-7" /> : <CheckCircle2 className="h-7 w-7" />}
              </div>
              <p className="text-sm font-medium text-[var(--color-text-primary)]">
                {query ? '没有匹配的插件' : '暂无插件'}
              </p>
              <Button className="mt-4" size="sm" onClick={() => setAddOpen(true)}>
                <Plus className="h-4 w-4" />
                添加插件
              </Button>
            </div>
          )}
        </div>
      </section>

      <AddExtensionModal
        open={addOpen}
        importing={importing}
        mode={importMode}
        webStoreUrl={webStoreUrl}
        onModeChange={setImportMode}
        onWebStoreUrlChange={setWebStoreUrl}
        onClose={() => setAddOpen(false)}
        onImport={handleImport}
      />

      <ConfirmModal
        open={Boolean(deletingItem)}
        onClose={() => setDeletingItem(null)}
        onConfirm={handleDelete}
        title="删除插件"
        content={`确定删除「${deletingItem?.name || '未命名插件'}」吗？`}
        confirmText="删除"
        danger
      />

    </div>
  )
}
