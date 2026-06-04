import { useEffect, useMemo, useState } from 'react'
import {
  BookOpen,
  CheckCircle2,
  Edit,
  ExternalLink,
  Globe2,
  KeyRound,
  ListChecks,
  Mail,
  Plus,
  Search,
  Trash2,
  User,
} from 'lucide-react'
import { Badge, Button, ConfirmModal, FormItem, Input, Modal, Select, Switch, Textarea, toast } from '../../../shared/components'
import type { BrowserProfile, BrowserProfileInput, SiteAccount, SiteAccountInput, SiteAccountTaskRun, SiteInput, SiteSummary } from '../types'
import {
  createBrowserProfile,
  deleteSite,
  deleteSiteAccount,
  fetchBrowserProfiles,
  fetchBrowserSettings,
  fetchSiteAccount,
  fetchSiteAccounts,
  fetchSites,
  quickOpenSiteAccount,
  runSiteAccountCheckin,
  runSiteAccountReading,
  saveSite,
  saveSiteAccount,
  fetchSiteAccountTaskRuns,
} from '../api'

const ALL_SITES = '__all__'
const ALL_PROFILES = '__all__'
const CREATE_PROFILE = '__create_profile__'
const DIRECT_PROXY_ID = '__direct__'
const fallbackLaunchArgs = ['--disable-sync', '--no-first-run']

const emptySiteForm: SiteInput = {
  siteName: '',
  homeUrl: '',
  loginUrl: '',
  checkinUrl: '',
  readingUrl: '',
  checkinButtonRule: '',
  status: 'enabled',
  remark: '',
}

const emptyAccountForm: SiteAccountInput = {
  siteId: '',
  profileId: '',
  username: '',
  password: '',
  email: '',
  emailPassword: '',
  accountUrl: '',
  autoCheckinEnabled: false,
  checkinUrl: '',
  checkinButtonRule: '',
  autoReadEnabled: false,
  readingUrl: '',
  status: 'active',
  remark: '',
}

const emptyNewProfileForm = {
  profileName: '',
  userDataDir: '',
  tags: '站点账号',
  keywords: '',
}

function profileLabel(profile: BrowserProfile | undefined, fallbackId: string) {
  if (!profile) return fallbackId || '-'
  return profile.launchCode ? `${profile.profileName} (${profile.launchCode})` : profile.profileName
}

function statusBadge(status: string) {
  switch (status) {
    case 'enabled':
    case 'active':
      return <Badge variant="success" size="sm" dot>启用</Badge>
    case 'disabled':
    case 'paused':
      return <Badge variant="warning" size="sm" dot>暂停</Badge>
    case 'invalid':
      return <Badge variant="error" size="sm" dot>异常</Badge>
    case 'archived':
      return <Badge variant="default" size="sm">归档</Badge>
    default:
      return <Badge variant="default" size="sm">{status || '-'}</Badge>
  }
}

function maskSecret(hasValue: boolean) {
  return hasValue ? '已保存' : '未保存'
}

function splitWords(value: string) {
  return value
    .split(/[\n,，]/)
    .map(item => item.trim())
    .filter(Boolean)
}

export function SiteAccountPage() {
  const [sites, setSites] = useState<SiteSummary[]>([])
  const [accounts, setAccounts] = useState<SiteAccount[]>([])
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedSiteId, setSelectedSiteId] = useState(ALL_SITES)
  const [profileFilter, setProfileFilter] = useState(ALL_PROFILES)
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const [checkinFilter, setCheckinFilter] = useState('all')

  const [siteModalOpen, setSiteModalOpen] = useState(false)
  const [siteForm, setSiteForm] = useState<SiteInput>(emptySiteForm)
  const [savingSite, setSavingSite] = useState(false)
  const [deleteSiteTarget, setDeleteSiteTarget] = useState<SiteSummary | null>(null)

  const [accountModalOpen, setAccountModalOpen] = useState(false)
  const [accountForm, setAccountForm] = useState<SiteAccountInput>(emptyAccountForm)
  const [newProfileForm, setNewProfileForm] = useState(emptyNewProfileForm)
  const [editingAccount, setEditingAccount] = useState<SiteAccount | null>(null)
  const [savingAccount, setSavingAccount] = useState(false)
  const [deleteAccountTarget, setDeleteAccountTarget] = useState<SiteAccount | null>(null)
  const [openingAccountId, setOpeningAccountId] = useState('')
  const [checkinRunningKey, setCheckinRunningKey] = useState('')
  const [readingRunningKey, setReadingRunningKey] = useState('')
  const [runHistoryOpen, setRunHistoryOpen] = useState(false)
  const [runHistoryAccount, setRunHistoryAccount] = useState<SiteAccount | null>(null)
  const [runHistory, setRunHistory] = useState<SiteAccountTaskRun[]>([])
  const [loadingHistory, setLoadingHistory] = useState(false)

  const profileMap = useMemo(() => new Map(profiles.map(profile => [profile.profileId, profile])), [profiles])
  const selectedSite = sites.find(site => site.siteId === selectedSiteId) || null

  const buildDefaultProfileName = (form: SiteAccountInput = accountForm) => {
    const site = sites.find(item => item.siteId === form.siteId)
    const siteName = site?.siteName?.trim() || '站点账号'
    const username = form.username.trim() || '新账号'
    return `${siteName}-${username}`
  }

  const loadAll = async () => {
    setLoading(true)
    try {
      const [siteItems, profileItems] = await Promise.all([
        fetchSites(),
        fetchBrowserProfiles(),
      ])
      setSites(siteItems)
      setProfiles(profileItems)
      await loadAccounts(siteItems)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '站点账号加载失败')
    } finally {
      setLoading(false)
    }
  }

  const loadAccounts = async (_siteItems = sites) => {
    const items = await fetchSiteAccounts({
      siteId: selectedSiteId === ALL_SITES ? '' : selectedSiteId,
      profileId: profileFilter === ALL_PROFILES ? '' : profileFilter,
      status: statusFilter === 'all' ? '' : statusFilter,
      keyword,
      autoCheckin: checkinFilter === 'all' ? '' : checkinFilter,
    })
    setAccounts(items)
  }

  useEffect(() => {
    void loadAll()
  }, [])

  useEffect(() => {
    if (!loading) {
      void loadAccounts()
    }
  }, [selectedSiteId, profileFilter, statusFilter, checkinFilter])

  const visibleSites = sites
  const totalAccounts = sites.reduce((sum, site) => sum + (site.accountCount || 0), 0)
  const autoCheckinCount = sites.reduce((sum, site) => sum + (site.autoCheckinCount || 0), 0)

  const openCreateSite = () => {
    setSiteForm(emptySiteForm)
    setSiteModalOpen(true)
  }

  const openEditSite = (site: SiteSummary) => {
    setSiteForm({
      siteId: site.siteId,
      siteName: site.siteName,
      homeUrl: site.homeUrl,
      loginUrl: site.loginUrl,
      checkinUrl: site.checkinUrl,
      readingUrl: site.readingUrl,
      checkinButtonRule: site.checkinButtonRule,
      status: site.status || 'enabled',
      remark: site.remark,
    })
    setSiteModalOpen(true)
  }

  const submitSite = async () => {
    if (!siteForm.siteName.trim()) {
      toast.warning('请填写站点名称')
      return
    }
    setSavingSite(true)
    try {
      await saveSite(siteForm)
      toast.success('站点已保存')
      setSiteModalOpen(false)
      const nextSites = await fetchSites()
      setSites(nextSites)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '站点保存失败')
    } finally {
      setSavingSite(false)
    }
  }

  const confirmDeleteSite = async () => {
    if (!deleteSiteTarget) return
    try {
      await deleteSite(deleteSiteTarget.siteId)
      toast.success('站点已删除')
      if (selectedSiteId === deleteSiteTarget.siteId) {
        setSelectedSiteId(ALL_SITES)
      }
      setSites(await fetchSites())
      await loadAccounts()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '站点删除失败')
    }
  }

  const openCreateAccount = () => {
    if (sites.length === 0) {
      toast.warning('请先创建站点')
      return
    }
    setEditingAccount(null)
    setAccountForm({
      ...emptyAccountForm,
      siteId: selectedSiteId === ALL_SITES ? sites[0]?.siteId || '' : selectedSiteId,
    })
    setNewProfileForm(emptyNewProfileForm)
    setAccountModalOpen(true)
  }

  const openEditAccount = async (account: SiteAccount) => {
    setEditingAccount(account)
    try {
      const detail = await fetchSiteAccount(account.accountId, true)
      const source = detail || account
      setAccountForm({
        accountId: source.accountId,
        siteId: source.siteId,
        profileId: source.profileId,
        username: source.username,
        password: source.password || '',
        email: source.email,
        emailPassword: source.emailPassword || '',
        accountUrl: source.accountUrl,
        autoCheckinEnabled: source.autoCheckinEnabled,
        checkinUrl: source.checkinUrl,
        checkinButtonRule: source.checkinButtonRule,
        autoReadEnabled: source.autoReadEnabled,
        readingUrl: source.readingUrl,
        status: source.status || 'active',
        remark: source.remark,
      })
      setNewProfileForm(emptyNewProfileForm)
      setAccountModalOpen(true)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '账号详情加载失败')
    }
  }

  const submitAccount = async () => {
    if (!accountForm.siteId) {
      toast.warning('请选择站点')
      return
    }
    if (!accountForm.username.trim()) {
      toast.warning('请填写用户名')
      return
    }
    const shouldCreateProfile = accountForm.profileId === CREATE_PROFILE
    if (!shouldCreateProfile && !accountForm.profileId) {
      toast.warning('请选择指纹浏览器')
      return
    }
    setSavingAccount(true)
    try {
      let profileId = accountForm.profileId
      if (shouldCreateProfile) {
        const settings = await fetchBrowserSettings()
        const profileName = newProfileForm.profileName.trim() || buildDefaultProfileName()
        const launchArgs = settings.defaultLaunchArgs?.length ? settings.defaultLaunchArgs : fallbackLaunchArgs
        const profileInput: BrowserProfileInput = {
          profileName,
          userDataDir: newProfileForm.userDataDir.trim(),
          coreId: '',
          fingerprintArgs: settings.defaultFingerprintArgs || [],
          proxyId: DIRECT_PROXY_ID,
          proxyConfig: '',
          launchArgs,
          tags: splitWords(newProfileForm.tags),
          keywords: splitWords(newProfileForm.keywords),
          groupId: '',
        }
        const createdProfile = await createBrowserProfile(profileInput)
        if (!createdProfile?.profileId) {
          throw new Error('指纹浏览器创建失败')
        }
        profileId = createdProfile.profileId
      }
      await saveSiteAccount({ ...accountForm, profileId })
      toast.success('账号已保存')
      setAccountModalOpen(false)
      setSites(await fetchSites())
      setProfiles(await fetchBrowserProfiles())
      await loadAccounts()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '账号保存失败')
    } finally {
      setSavingAccount(false)
    }
  }

  const confirmDeleteAccount = async () => {
    if (!deleteAccountTarget) return
    try {
      await deleteSiteAccount(deleteAccountTarget.accountId)
      toast.success('账号已删除')
      setSites(await fetchSites())
      await loadAccounts()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '账号删除失败')
    }
  }

  const quickOpen = async (account: SiteAccount, target: string) => {
    setOpeningAccountId(`${account.accountId}:${target}`)
    try {
      const profile = await quickOpenSiteAccount(account.accountId, target)
      toast.success(profile?.running ? '已打开账号网址' : '已启动指纹浏览器')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '快速打开失败')
    } finally {
      setOpeningAccountId('')
    }
  }

  const runCheckinForAccount = async (account: SiteAccount) => {
    setCheckinRunningKey(account.accountId)
    try {
      const result = await runSiteAccountCheckin({ accountId: account.accountId, concurrency: 1 })
      if (result.failed > 0) {
        const firstError = result.runs.find(run => run.status !== 'success')?.error
        toast.warning(firstError || `签到完成，失败 ${result.failed} 个`, 6000)
      } else {
        toast.success('签到执行完成')
      }
      await loadHistory(account)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '签到执行失败', 7000)
    } finally {
      setCheckinRunningKey('')
    }
  }

  const runCheckinForCurrentFilter = async () => {
    setCheckinRunningKey('__batch__')
    try {
      const result = await runSiteAccountCheckin({
        siteId: selectedSiteId === ALL_SITES ? '' : selectedSiteId,
        profileId: profileFilter === ALL_PROFILES ? '' : profileFilter,
        filter: {
          status: statusFilter === 'all' ? '' : statusFilter,
          keyword,
          autoCheckin: 'enabled',
        },
        concurrency: 2,
      })
      if (result.total === 0) {
        toast.info('当前筛选下没有开启自动签到的账号')
      } else if (result.failed > 0) {
        toast.warning(`签到完成：成功 ${result.succeeded}，失败 ${result.failed}`, 7000)
      } else {
        toast.success(`签到完成：成功 ${result.succeeded}`)
      }
      if (runHistoryAccount) {
        await loadHistory(runHistoryAccount)
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '批量签到失败', 7000)
    } finally {
      setCheckinRunningKey('')
    }
  }

  const runReadingForAccount = async (account: SiteAccount) => {
    setReadingRunningKey(account.accountId)
    try {
      const result = await runSiteAccountReading({ accountId: account.accountId, concurrency: 1 })
      if (result.failed > 0) {
        const firstError = result.runs.find(run => run.status !== 'success')?.error
        toast.warning(firstError || `阅读完成，失败 ${result.failed} 个`, 6000)
      } else {
        toast.success('阅读执行完成')
      }
      await loadHistory(account)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '阅读执行失败', 7000)
    } finally {
      setReadingRunningKey('')
    }
  }

  const runReadingForCurrentFilter = async () => {
    setReadingRunningKey('__batch__')
    try {
      const result = await runSiteAccountReading({
        siteId: selectedSiteId === ALL_SITES ? '' : selectedSiteId,
        profileId: profileFilter === ALL_PROFILES ? '' : profileFilter,
        filter: {
          status: statusFilter === 'all' ? '' : statusFilter,
          keyword,
          autoRead: 'enabled',
        },
        concurrency: 2,
      })
      if (result.total === 0) {
        toast.info('当前筛选下没有开启自动阅读的账号')
      } else if (result.failed > 0) {
        toast.warning(`阅读完成：成功 ${result.succeeded}，失败 ${result.failed}`, 7000)
      } else {
        toast.success(`阅读完成：成功 ${result.succeeded}`)
      }
      if (runHistoryAccount) {
        await loadHistory(runHistoryAccount)
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '批量阅读失败', 7000)
    } finally {
      setReadingRunningKey('')
    }
  }

  const loadHistory = async (account: SiteAccount) => {
    setRunHistoryAccount(account)
    setLoadingHistory(true)
    try {
      setRunHistory(await fetchSiteAccountTaskRuns(account.accountId, 50))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '执行记录加载失败')
    } finally {
      setLoadingHistory(false)
    }
  }

  const openHistory = async (account: SiteAccount) => {
    setRunHistoryOpen(true)
    await loadHistory(account)
  }

  const applyKeywordSearch = () => {
    void loadAccounts()
  }

  const handleAccountProfileChange = (profileId: string) => {
    const nextForm = { ...accountForm, profileId }
    setAccountForm(nextForm)
    if (profileId === CREATE_PROFILE && !newProfileForm.profileName.trim()) {
      setNewProfileForm(prev => ({ ...prev, profileName: buildDefaultProfileName(nextForm) }))
    }
  }

  return (
    <div className="h-full min-h-0 bg-[var(--color-bg-page)]">
      <div className="border-b border-[var(--color-border-default)] bg-[var(--color-bg-surface)] px-6 py-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">站点账号</h1>
            <div className="mt-1 flex flex-wrap gap-3 text-xs text-[var(--color-text-muted)]">
              <span>站点 {sites.length}</span>
              <span>账号 {totalAccounts}</span>
              <span>自动签到 {autoCheckinCount}</span>
            </div>
          </div>
          <div className="flex gap-2">
            <Button variant="secondary" onClick={openCreateSite}>
              <Globe2 className="h-4 w-4" />
              新建站点
            </Button>
            <Button onClick={openCreateAccount}>
              <Plus className="h-4 w-4" />
              新建账号
            </Button>
          </div>
        </div>
      </div>

      <div className="grid h-[calc(100vh-137px)] grid-cols-[280px_minmax(0,1fr)] overflow-hidden">
        <aside className="border-r border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-4">
          <button
            className={`mb-2 flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-sm transition-colors ${
              selectedSiteId === ALL_SITES
                ? 'bg-[var(--color-accent)] text-[var(--color-text-inverse)]'
                : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-muted)]'
            }`}
            onClick={() => setSelectedSiteId(ALL_SITES)}
          >
            <span>全部站点</span>
            <span>{totalAccounts}</span>
          </button>
          <div className="space-y-2 overflow-y-auto pr-1" style={{ maxHeight: 'calc(100vh - 215px)' }}>
            {visibleSites.map(site => (
              <div
                key={site.siteId}
                className={`rounded-lg border p-3 transition-colors ${
                  selectedSiteId === site.siteId
                    ? 'border-[var(--color-accent)] bg-[var(--color-accent-muted)]'
                    : 'border-[var(--color-border-muted)] hover:border-[var(--color-border-strong)]'
                }`}
              >
                <button className="w-full text-left" onClick={() => setSelectedSiteId(site.siteId)}>
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate text-sm font-medium text-[var(--color-text-primary)]">{site.siteName}</span>
                    {statusBadge(site.status)}
                  </div>
                  <div className="mt-2 flex gap-2 text-xs text-[var(--color-text-muted)]">
                    <span>{site.accountCount || 0} 账号</span>
                    <span>{site.autoCheckinCount || 0} 签到</span>
                  </div>
                  {site.homeUrl && <div className="mt-2 truncate text-xs text-[var(--color-text-muted)]">{site.homeUrl}</div>}
                </button>
                <div className="mt-3 flex gap-1">
                  <button
                    className="rounded-md p-1.5 text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
                    onClick={() => openEditSite(site)}
                    title="编辑站点"
                  >
                    <Edit className="h-4 w-4" />
                  </button>
                  <button
                    className="rounded-md p-1.5 text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-error)]"
                    onClick={() => setDeleteSiteTarget(site)}
                    title="删除站点"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>
            ))}
            {!loading && visibleSites.length === 0 && (
              <div className="rounded-lg border border-dashed border-[var(--color-border-default)] p-4 text-center text-sm text-[var(--color-text-muted)]">
                暂无站点
              </div>
            )}
          </div>
        </aside>

        <main className="min-w-0 overflow-hidden p-5">
          <div className="mb-4 flex flex-wrap items-center gap-2">
            <div className="relative w-72">
              <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-[var(--color-text-muted)]" />
              <Input
                className="pl-9"
                placeholder="搜索用户名、邮箱、备注"
                value={keyword}
                onChange={event => setKeyword(event.target.value)}
                onKeyDown={event => {
                  if (event.key === 'Enter') applyKeywordSearch()
                }}
              />
            </div>
            <Select
              className="w-52"
              value={profileFilter}
              onChange={event => setProfileFilter(event.target.value)}
              options={[
                { value: ALL_PROFILES, label: '全部指纹浏览器' },
                ...profiles.map(profile => ({ value: profile.profileId, label: profileLabel(profile, profile.profileId) })),
              ]}
            />
            <Select
              className="w-32"
              value={statusFilter}
              onChange={event => setStatusFilter(event.target.value)}
              options={[
                { value: 'all', label: '全部状态' },
                { value: 'active', label: '启用' },
                { value: 'paused', label: '暂停' },
                { value: 'invalid', label: '异常' },
                { value: 'archived', label: '归档' },
              ]}
            />
            <Select
              className="w-36"
              value={checkinFilter}
              onChange={event => setCheckinFilter(event.target.value)}
              options={[
                { value: 'all', label: '全部签到' },
                { value: 'enabled', label: '已开启签到' },
                { value: 'disabled', label: '未开启签到' },
              ]}
            />
            <Button variant="secondary" onClick={applyKeywordSearch}>
              查询
            </Button>
            <Button
              variant="secondary"
              loading={checkinRunningKey === '__batch__'}
              onClick={runCheckinForCurrentFilter}
            >
              <CheckCircle2 className="h-4 w-4" />
              执行当前筛选签到
            </Button>
            <Button
              variant="secondary"
              loading={readingRunningKey === '__batch__'}
              onClick={runReadingForCurrentFilter}
            >
              <BookOpen className="h-4 w-4" />
              执行当前筛选阅读
            </Button>
          </div>

          <div className="overflow-hidden rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)]">
            <div className="flex items-center justify-between border-b border-[var(--color-border-muted)] px-4 py-3">
              <div>
                <div className="text-sm font-medium text-[var(--color-text-primary)]">
                  {selectedSite ? selectedSite.siteName : '全部账号'}
                </div>
                <div className="text-xs text-[var(--color-text-muted)]">{accounts.length} 条记录</div>
              </div>
              <Button size="sm" onClick={openCreateAccount}>
                <Plus className="h-4 w-4" />
                添加账号
              </Button>
            </div>

            <div className="overflow-auto" style={{ maxHeight: 'calc(100vh - 265px)' }}>
              <table className="min-w-[1120px] w-full">
                <thead className="sticky top-0 z-10 bg-[var(--color-bg-muted)]">
                  <tr className="text-left text-xs font-semibold uppercase tracking-wider text-[var(--color-text-muted)]">
                    <th className="px-4 py-3">账号</th>
                    <th className="px-4 py-3">站点</th>
                    <th className="px-4 py-3">指纹浏览器</th>
                    <th className="px-4 py-3">安全</th>
                    <th className="px-4 py-3">自动化</th>
                    <th className="px-4 py-3">状态</th>
                    <th className="px-4 py-3 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--color-border-muted)]">
                  {loading ? (
                    <tr>
                      <td className="px-4 py-12 text-center text-sm text-[var(--color-text-muted)]" colSpan={7}>加载中...</td>
                    </tr>
                  ) : accounts.length === 0 ? (
                    <tr>
                      <td className="px-4 py-12 text-center text-sm text-[var(--color-text-muted)]" colSpan={7}>暂无账号</td>
                    </tr>
                  ) : accounts.map(account => {
                    const profile = profileMap.get(account.profileId)
                    return (
                      <tr key={account.accountId} className="hover:bg-[var(--color-bg-muted)]/50">
                        <td className="px-4 py-3">
                          <div className="flex items-start gap-3">
                            <div className="mt-0.5 flex h-8 w-8 items-center justify-center rounded-lg bg-[var(--color-accent-muted)] text-[var(--color-accent)]">
                              <User className="h-4 w-4" />
                            </div>
                            <div className="min-w-0">
                              <div className="truncate text-sm font-medium text-[var(--color-text-primary)]">{account.username}</div>
                              <div className="mt-1 flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
                                <Mail className="h-3.5 w-3.5" />
                                <span className="truncate">{account.email || '-'}</span>
                              </div>
                              {account.remark && <div className="mt-1 max-w-64 truncate text-xs text-[var(--color-text-muted)]">{account.remark}</div>}
                            </div>
                          </div>
                        </td>
                        <td className="px-4 py-3 text-sm text-[var(--color-text-secondary)]">{account.siteName || '-'}</td>
                        <td className="px-4 py-3">
                          <div className="text-sm text-[var(--color-text-primary)]">{profileLabel(profile, account.profileId)}</div>
                          <div className="text-xs text-[var(--color-text-muted)]">{account.profileId}</div>
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex flex-col gap-1 text-xs text-[var(--color-text-muted)]">
                            <span className="inline-flex items-center gap-1"><KeyRound className="h-3.5 w-3.5" />账号密码：{maskSecret(account.hasPassword)}</span>
                            <span className="inline-flex items-center gap-1"><KeyRound className="h-3.5 w-3.5" />邮箱密码：{maskSecret(account.hasEmailPassword)}</span>
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex flex-col gap-1">
                            <Badge variant={account.autoCheckinEnabled ? 'info' : 'default'} size="sm">
                              签到 {account.autoCheckinEnabled ? '开启' : '关闭'}
                            </Badge>
                            <Badge variant={account.autoReadEnabled ? 'info' : 'default'} size="sm">
                              阅读 {account.autoReadEnabled ? '开启' : '关闭'}
                            </Badge>
                          </div>
                        </td>
                        <td className="px-4 py-3">{statusBadge(account.status)}</td>
                        <td className="px-4 py-3">
                          <div className="flex justify-end gap-1">
                            <Button
                              size="sm"
                              variant="secondary"
                              loading={openingAccountId === `${account.accountId}:default`}
                              onClick={() => quickOpen(account, 'default')}
                            >
                              <ExternalLink className="h-4 w-4" />
                              打开
                            </Button>
                            <button
                              className="rounded-md p-2 text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-success)] disabled:cursor-not-allowed disabled:opacity-50"
                              onClick={() => runCheckinForAccount(account)}
                              disabled={!!checkinRunningKey || !!readingRunningKey}
                              title="执行签到"
                            >
                              <CheckCircle2 className={`h-4 w-4 ${checkinRunningKey === account.accountId ? 'animate-pulse' : ''}`} />
                            </button>
                            <button
                              className="rounded-md p-2 text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
                              onClick={() => runReadingForAccount(account)}
                              disabled={!!checkinRunningKey || !!readingRunningKey}
                              title="执行阅读"
                            >
                              <BookOpen className={`h-4 w-4 ${readingRunningKey === account.accountId ? 'animate-pulse' : ''}`} />
                            </button>
                            <button
                              className="rounded-md p-2 text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
                              onClick={() => openHistory(account)}
                              title="执行记录"
                            >
                              <ListChecks className="h-4 w-4" />
                            </button>
                            <button
                              className="rounded-md p-2 text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-text-primary)]"
                              onClick={() => openEditAccount(account)}
                              title="编辑账号"
                            >
                              <Edit className="h-4 w-4" />
                            </button>
                            <button
                              className="rounded-md p-2 text-[var(--color-text-muted)] hover:bg-[var(--color-bg-muted)] hover:text-[var(--color-error)]"
                              onClick={() => setDeleteAccountTarget(account)}
                              title="删除账号"
                            >
                              <Trash2 className="h-4 w-4" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </div>
        </main>
      </div>

      <Modal
        open={siteModalOpen}
        onClose={() => setSiteModalOpen(false)}
        title={siteForm.siteId ? '编辑站点' : '新建站点'}
        width="720px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setSiteModalOpen(false)}>取消</Button>
            <Button onClick={submitSite} loading={savingSite}>保存</Button>
          </>
        }
      >
        <div className="grid grid-cols-2 gap-4">
          <FormItem label="站点名称" required>
            <Input value={siteForm.siteName} onChange={event => setSiteForm(prev => ({ ...prev, siteName: event.target.value }))} />
          </FormItem>
          <FormItem label="状态">
            <Select
              value={siteForm.status}
              onChange={event => setSiteForm(prev => ({ ...prev, status: event.target.value }))}
              options={[
                { value: 'enabled', label: '启用' },
                { value: 'disabled', label: '停用' },
              ]}
            />
          </FormItem>
          <FormItem label="主页 URL">
            <Input value={siteForm.homeUrl} onChange={event => setSiteForm(prev => ({ ...prev, homeUrl: event.target.value }))} placeholder="https://example.com" />
          </FormItem>
          <FormItem label="登录 URL">
            <Input value={siteForm.loginUrl} onChange={event => setSiteForm(prev => ({ ...prev, loginUrl: event.target.value }))} />
          </FormItem>
          <FormItem label="签到 URL">
            <Input value={siteForm.checkinUrl} onChange={event => setSiteForm(prev => ({ ...prev, checkinUrl: event.target.value }))} />
          </FormItem>
          <FormItem label="阅读 URL">
            <Input value={siteForm.readingUrl} onChange={event => setSiteForm(prev => ({ ...prev, readingUrl: event.target.value }))} />
          </FormItem>
          <FormItem label="签到按钮规则" className="col-span-2">
            <Input value={siteForm.checkinButtonRule} onChange={event => setSiteForm(prev => ({ ...prev, checkinButtonRule: event.target.value }))} placeholder="CSS selector / 按钮文本 / XPath" />
          </FormItem>
          <FormItem label="备注" className="col-span-2">
            <Textarea rows={3} value={siteForm.remark} onChange={event => setSiteForm(prev => ({ ...prev, remark: event.target.value }))} />
          </FormItem>
        </div>
      </Modal>

      <Modal
        open={accountModalOpen}
        onClose={() => setAccountModalOpen(false)}
        title={editingAccount ? '编辑账号' : '新建账号'}
        width="820px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setAccountModalOpen(false)}>取消</Button>
            <Button onClick={submitAccount} loading={savingAccount}>保存</Button>
          </>
        }
      >
        <div className="grid grid-cols-2 gap-4">
          <FormItem label="站点" required>
            <Select
              value={accountForm.siteId}
              onChange={event => setAccountForm(prev => ({ ...prev, siteId: event.target.value }))}
              options={sites.map(site => ({ value: site.siteId, label: site.siteName }))}
            />
          </FormItem>
          <FormItem label="指纹浏览器" required>
            <Select
              value={accountForm.profileId}
              onChange={event => handleAccountProfileChange(event.target.value)}
              options={[
                { value: '', label: '请选择指纹浏览器' },
                ...(!editingAccount ? [{ value: CREATE_PROFILE, label: '新建指纹浏览器并绑定' }] : []),
                ...profiles.map(profile => ({ value: profile.profileId, label: profileLabel(profile, profile.profileId) })),
              ]}
            />
          </FormItem>
          {accountForm.profileId === CREATE_PROFILE && (
            <div className="col-span-2 grid grid-cols-2 gap-4 rounded-lg border border-[var(--color-border-muted)] bg-[var(--color-bg-muted)]/30 p-4">
              <FormItem label="新实例名称" required>
                <Input
                  value={newProfileForm.profileName}
                  onChange={event => setNewProfileForm(prev => ({ ...prev, profileName: event.target.value }))}
                  placeholder={buildDefaultProfileName()}
                />
              </FormItem>
              <FormItem label="用户数据目录" hint="留空自动生成">
                <Input
                  value={newProfileForm.userDataDir}
                  onChange={event => setNewProfileForm(prev => ({ ...prev, userDataDir: event.target.value }))}
                  placeholder="留空则使用新实例 ID"
                />
              </FormItem>
              <FormItem label="标签" hint="逗号或换行分隔">
                <Input
                  value={newProfileForm.tags}
                  onChange={event => setNewProfileForm(prev => ({ ...prev, tags: event.target.value }))}
                />
              </FormItem>
              <FormItem label="关键字" hint="逗号或换行分隔">
                <Input
                  value={newProfileForm.keywords}
                  onChange={event => setNewProfileForm(prev => ({ ...prev, keywords: event.target.value }))}
                />
              </FormItem>
            </div>
          )}
          <FormItem label="用户名" required>
            <Input value={accountForm.username} onChange={event => setAccountForm(prev => ({ ...prev, username: event.target.value }))} />
          </FormItem>
          <FormItem label="账号密码">
            <Input
              type="password"
              value={accountForm.password}
              onChange={event => setAccountForm(prev => ({ ...prev, password: event.target.value }))}
              placeholder={editingAccount?.hasPassword ? '留空则保持原密码' : ''}
            />
          </FormItem>
          <FormItem label="邮箱">
            <Input value={accountForm.email} onChange={event => setAccountForm(prev => ({ ...prev, email: event.target.value }))} />
          </FormItem>
          <FormItem label="邮箱密码">
            <Input
              type="password"
              value={accountForm.emailPassword}
              onChange={event => setAccountForm(prev => ({ ...prev, emailPassword: event.target.value }))}
              placeholder={editingAccount?.hasEmailPassword ? '留空则保持原密码' : ''}
            />
          </FormItem>
          <FormItem label="账号 URL">
            <Input value={accountForm.accountUrl} onChange={event => setAccountForm(prev => ({ ...prev, accountUrl: event.target.value }))} />
          </FormItem>
          <FormItem label="状态">
            <Select
              value={accountForm.status}
              onChange={event => setAccountForm(prev => ({ ...prev, status: event.target.value }))}
              options={[
                { value: 'active', label: '启用' },
                { value: 'paused', label: '暂停' },
                { value: 'invalid', label: '异常' },
                { value: 'archived', label: '归档' },
              ]}
            />
          </FormItem>
          <div className="col-span-2 grid grid-cols-2 gap-4 rounded-lg border border-[var(--color-border-muted)] p-4">
            <div className="flex items-center justify-between">
              <div>
                <div className="text-sm font-medium text-[var(--color-text-primary)]">自动签到</div>
                <div className="text-xs text-[var(--color-text-muted)]">执行器后续接入，当前保存配置</div>
              </div>
              <Switch checked={accountForm.autoCheckinEnabled} onChange={checked => setAccountForm(prev => ({ ...prev, autoCheckinEnabled: checked }))} />
            </div>
            <div className="flex items-center justify-between">
              <div>
                <div className="text-sm font-medium text-[var(--color-text-primary)]">自动阅读</div>
                <div className="text-xs text-[var(--color-text-muted)]">执行器后续接入，当前保存配置</div>
              </div>
              <Switch checked={accountForm.autoReadEnabled} onChange={checked => setAccountForm(prev => ({ ...prev, autoReadEnabled: checked }))} />
            </div>
            <FormItem label="签到 URL">
              <Input value={accountForm.checkinUrl} onChange={event => setAccountForm(prev => ({ ...prev, checkinUrl: event.target.value }))} />
            </FormItem>
            <FormItem label="阅读 URL">
              <Input value={accountForm.readingUrl} onChange={event => setAccountForm(prev => ({ ...prev, readingUrl: event.target.value }))} />
            </FormItem>
            <FormItem label="签到按钮规则" className="col-span-2">
              <Input value={accountForm.checkinButtonRule} onChange={event => setAccountForm(prev => ({ ...prev, checkinButtonRule: event.target.value }))} placeholder="优先使用账号规则，留空则使用站点规则" />
            </FormItem>
          </div>
          <FormItem label="备注" className="col-span-2">
            <Textarea rows={3} value={accountForm.remark} onChange={event => setAccountForm(prev => ({ ...prev, remark: event.target.value }))} />
          </FormItem>
        </div>
      </Modal>

      <ConfirmModal
        open={!!deleteSiteTarget}
        onClose={() => setDeleteSiteTarget(null)}
        onConfirm={confirmDeleteSite}
        title="删除站点"
        content={`确认删除站点「${deleteSiteTarget?.siteName || ''}」？该站点下账号也会一并删除。`}
        danger
      />

      <ConfirmModal
        open={!!deleteAccountTarget}
        onClose={() => setDeleteAccountTarget(null)}
        onConfirm={confirmDeleteAccount}
        title="删除账号"
        content={`确认删除账号「${deleteAccountTarget?.username || ''}」？`}
        danger
      />

      <Modal
        open={runHistoryOpen}
        onClose={() => setRunHistoryOpen(false)}
        title={runHistoryAccount ? `执行记录 - ${runHistoryAccount.username}` : '执行记录'}
        width="760px"
      >
        <div className="overflow-hidden rounded-lg border border-[var(--color-border-default)]">
          <table className="w-full">
            <thead className="bg-[var(--color-bg-muted)] text-left text-xs font-semibold text-[var(--color-text-muted)]">
              <tr>
                <th className="px-4 py-3">任务</th>
                <th className="px-4 py-3">状态</th>
                <th className="px-4 py-3">摘要</th>
                <th className="px-4 py-3">时间</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border-muted)]">
              {loadingHistory ? (
                <tr>
                  <td colSpan={4} className="px-4 py-10 text-center text-sm text-[var(--color-text-muted)]">加载中...</td>
                </tr>
              ) : runHistory.length === 0 ? (
                <tr>
                  <td colSpan={4} className="px-4 py-10 text-center text-sm text-[var(--color-text-muted)]">暂无执行记录</td>
                </tr>
              ) : runHistory.map(run => (
                <tr key={run.runId}>
                  <td className="px-4 py-3 text-sm text-[var(--color-text-primary)]">{run.taskType === 'checkin' ? '签到' : run.taskType === 'reading' ? '阅读' : run.taskType}</td>
                  <td className="px-4 py-3">
                    <Badge variant={run.status === 'success' ? 'success' : run.status === 'failed' ? 'error' : 'info'} size="sm" dot>
                      {run.status === 'success' ? '成功' : run.status === 'failed' ? '失败' : run.status}
                    </Badge>
                  </td>
                  <td className="px-4 py-3">
                    <div className="max-w-80 text-sm text-[var(--color-text-secondary)]">{run.summary || '-'}</div>
                    {run.error && <div className="mt-1 max-w-80 text-xs text-[var(--color-error)]">{run.error}</div>}
                    {run.artifactPath && <div className="mt-1 max-w-80 truncate text-xs text-[var(--color-text-muted)]">{run.artifactPath}</div>}
                  </td>
                  <td className="px-4 py-3 text-xs text-[var(--color-text-muted)]">
                    <div>{run.startedAt || '-'}</div>
                    <div>{run.durationMs ? `${run.durationMs}ms` : ''}</div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Modal>
    </div>
  )
}
