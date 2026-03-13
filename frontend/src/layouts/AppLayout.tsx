import { NavLink as RouterNavLink, Outlet, useNavigate, useLocation } from 'react-router'
import { useAuthStore } from '@/store/auth'
import { credentialsApi, gatesApi, schedulesApi } from '@/api'
import type { MemberCredential, CreatedToken } from '@/api'
import type { Gate, AccessSchedule } from '@/types'
import { GatePermissionsGrid, useGatePermissions } from '@/components/GatePermissionsGrid'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { SimpleSelect } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Separator } from '@/components/ui/separator'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { SimpleTooltip } from '@/components/ui/tooltip'
import {
  LayoutGrid,
  Users,
  Settings,
  LogOut,
  ChevronDown,
  DoorOpen,
  KeyRound,
  Copy,
  Check,
  Trash2,
  CalendarClock,
  Menu,
  X,
} from 'lucide-react'
import { useEffect, useState, useCallback } from 'react'

function NavItem({ to, label, icon: Icon, onClick }: { to: string; label: string; icon: React.FC<{ className?: string }>; onClick?: () => void }) {
  const location = useLocation()
  const isActive = location.pathname === to || location.pathname.startsWith(to + '/')

  return (
    <RouterNavLink
      to={to}
      onClick={onClick}
      className={cn(
        'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
        isActive
          ? 'bg-primary/10 text-primary'
          : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
      )}
    >
      <Icon className="h-4 w-4" />
      {label}
    </RouterNavLink>
  )
}

export default function AppLayout() {
  const { t } = useTranslation()
  const tokenPermissions = useGatePermissions()
  const session = useAuthStore((s) => s.session)
  const logout = useAuthStore((s) => s.logout)
  const member = session?.type === 'member' ? session.member : null
  const isAdmin = member?.role === 'ADMIN'
  const navigate = useNavigate()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const [tokenModalOpen, setTokenModalOpen] = useState(false)
  const [tokens, setTokens] = useState<MemberCredential[]>([])
  const [tokensLoading, setTokensLoading] = useState(false)
  const [tokenLabel, setTokenLabel] = useState('')
  const [tokenExpiresAt, setTokenExpiresAt] = useState('')
  const [newToken, setNewToken] = useState<CreatedToken | null>(null)
  const [tokenGates, setTokenGates] = useState<Gate[]>([])
  const [tokenSchedules, setTokenSchedules] = useState<AccessSchedule[]>([])
  const [tokenScheduleId, setTokenScheduleId] = useState('')
  const [tokenRestrictPerms, setTokenRestrictPerms] = useState(false)
  const [tokenPolicies, setTokenPolicies] = useState<{ gate_id: string; permission_code: string }[]>([])
  const [copied, setCopied] = useState(false)

  async function handleLogout() {
    await logout()
    navigate('/login')
  }

  const closeMobileNav = useCallback(() => setMobileNavOpen(false), [])

  useEffect(() => {
    if (!tokenModalOpen || !member) return
    setTokensLoading(true)
    Promise.all([
      credentialsApi.listTokens(),
      gatesApi.list().catch(() => []),
      schedulesApi.listMine().catch(() => []),
    ]).then(([tks, gates, schedules]) => {
      setTokens(tks)
      setTokenGates(gates as Gate[])
      setTokenSchedules(schedules as AccessSchedule[])
    }).finally(() => setTokensLoading(false))
  }, [tokenModalOpen, member])

  function resetTokenForm() {
    setTokenLabel('')
    setTokenExpiresAt('')
    setTokenScheduleId('')
    setTokenRestrictPerms(false)
    setTokenPolicies([])
  }

  function togglePolicy(gateId: string, permCode: string) {
    setTokenPolicies((prev) => {
      const exists = prev.some((p) => p.gate_id === gateId && p.permission_code === permCode)
      if (exists) return prev.filter((p) => !(p.gate_id === gateId && p.permission_code === permCode))
      return [...prev, { gate_id: gateId, permission_code: permCode }]
    })
  }

  async function handleCreateToken(e: React.FormEvent) {
    e.preventDefault()
    if (!tokenLabel.trim()) return
    const policies = tokenRestrictPerms && tokenPolicies.length > 0 ? tokenPolicies : undefined
    const created = await credentialsApi.createToken(tokenLabel, tokenExpiresAt || undefined, policies, tokenScheduleId || undefined)
    const updated = await credentialsApi.listTokens()
    setTokens(updated)
    setNewToken(created)
    resetTokenForm()
  }

  async function handleDeleteToken(credId: string) {
    await credentialsApi.deleteToken(credId)
    setTokens((prev) => prev.filter((t) => t.id !== credId))
    if (newToken?.id === credId) setNewToken(null)
  }

  function copyToken(text: string) {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const initials = member?.username?.slice(0, 2).toUpperCase() ?? 'U'

  const navContent = (
    <>
      <nav className="flex-1 overflow-y-auto p-2 space-y-1">
        <NavItem to="/gates" label={t('gates.title')} icon={LayoutGrid} onClick={closeMobileNav} />
        <NavItem to="/schedules" label={t('schedules.title')} icon={CalendarClock} onClick={closeMobileNav} />
        {isAdmin && (
          <>
            <div className="px-3 pt-4 pb-1">
              <p className="text-xs font-medium text-muted-foreground">{t('common.administration')}</p>
            </div>
            <NavItem to="/members" label={t('members.title')} icon={Users} onClick={closeMobileNav} />
            <NavItem to="/settings" label={t('settings.title')} icon={Settings} onClick={closeMobileNav} />
          </>
        )}
      </nav>

      {/* Footer */}
      <div className="border-t p-2 flex items-center justify-between">
        <div className="flex items-center gap-1">
          <LangToggle />
          <ThemeToggle />
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button className="flex items-center gap-1.5 rounded-md px-1.5 py-1 hover:bg-accent transition-colors cursor-pointer">
              <Avatar className="h-6 w-6">
                <AvatarFallback className="text-[10px]">{initials}</AvatarFallback>
              </Avatar>
              <ChevronDown className="h-3 w-3 text-muted-foreground" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuLabel className="text-xs">{member?.username}</DropdownMenuLabel>
            <DropdownMenuItem onClick={() => setTokenModalOpen(true)}>
              <KeyRound className="h-4 w-4" />
              {t('members.apiTokens')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={handleLogout} className="text-destructive focus:text-destructive">
              <LogOut className="h-4 w-4" />
              {t('auth.signOut')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </>
  )

  return (
    <div className="flex h-screen">
      {/* Desktop sidebar */}
      <aside className="hidden sm:flex w-64 flex-col border-r bg-sidebar text-sidebar-foreground">
        <div className="flex items-center gap-2 px-4 h-14 border-b">
          <button
            onClick={() => navigate('/gates')}
            className="flex items-center gap-2 cursor-pointer"
          >
            <div className="flex items-center justify-center h-7 w-7 rounded-md bg-primary/10 text-primary">
              <DoorOpen className="h-3.5 w-3.5" />
            </div>
            <span className="font-bold font-mono">GATIE</span>
          </button>
        </div>
        {navContent}
      </aside>

      {/* Mobile header */}
      <div className="flex flex-1 flex-col min-w-0">
        <header className="sm:hidden flex items-center justify-between px-4 h-14 border-b">
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="icon-sm" onClick={() => setMobileNavOpen(true)}>
              <Menu className="h-4 w-4" />
            </Button>
            <button
              onClick={() => navigate('/gates')}
              className="flex items-center gap-2 cursor-pointer"
            >
              <div className="flex items-center justify-center h-6 w-6 rounded-md bg-primary/10 text-primary">
                <DoorOpen className="h-3 w-3" />
              </div>
              <span className="font-bold text-sm font-mono">GATIE</span>
            </button>
          </div>
          <div className="flex items-center gap-1">
            <LangToggle />
            <ThemeToggle />
          </div>
        </header>

        {/* Mobile nav overlay */}
        {mobileNavOpen && (
          <div className="sm:hidden fixed inset-0 z-50 flex">
            <div className="fixed inset-0 bg-black/50" onClick={closeMobileNav} />
            <div className="relative w-64 flex flex-col bg-sidebar border-r z-50">
              <div className="flex items-center justify-between px-4 h-14 border-b">
                <button
                  onClick={() => { navigate('/gates'); closeMobileNav() }}
                  className="flex items-center gap-2 cursor-pointer"
                >
                  <div className="flex items-center justify-center h-7 w-7 rounded-md bg-primary/10 text-primary">
                    <DoorOpen className="h-3.5 w-3.5" />
                  </div>
                  <span className="font-bold font-mono">GATIE</span>
                </button>
                <Button variant="ghost" size="icon-sm" onClick={closeMobileNav}>
                  <X className="h-4 w-4" />
                </Button>
              </div>
              {navContent}
            </div>
          </div>
        )}

        {/* Main content */}
        <main className="flex-1 overflow-auto">
          <Outlet />
        </main>
      </div>

      {/* API token management dialog */}
      <Dialog open={tokenModalOpen} onOpenChange={(open) => { setTokenModalOpen(open); if (!open) { setNewToken(null); resetTokenForm() } }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('members.apiTokens')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-6">
            {newToken && (
              <Alert variant="success">
                <AlertTitle>{t('members.tokenCreated')}</AlertTitle>
                <AlertDescription>
                  <p className="text-sm mb-2">{t('members.tokenCreatedHint')}</p>
                  <div className="flex items-center gap-2">
                    <code className="flex-1 text-xs break-all bg-background/50 rounded px-2 py-1">{newToken.token}</code>
                    <SimpleTooltip label={copied ? t('common.copied') : t('common.copy')}>
                      <Button variant="ghost" size="icon-sm" onClick={() => copyToken(newToken.token)}>
                        {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                      </Button>
                    </SimpleTooltip>
                  </div>
                </AlertDescription>
              </Alert>
            )}

            <div>
              <h4 className="font-semibold mb-3">{t('members.newToken')}</h4>
              <form onSubmit={handleCreateToken} className="space-y-3">
                <Input
                  label={t('members.tokenLabel')}
                  placeholder={t('members.tokenLabelPlaceholder')}
                  value={tokenLabel}
                  onChange={(e) => setTokenLabel(e.target.value)}
                  required
                />
                <Input
                  label={`${t('members.tokenExpiresAt')} (${t('common.optional')})`}
                  type="date"
                  value={tokenExpiresAt}
                  onChange={(e) => setTokenExpiresAt(e.target.value)}
                />

                {tokenSchedules.length > 0 && (
                  <SimpleSelect
                    label={t('members.tokenSchedule')}
                    description={t('members.tokenScheduleHint')}
                    value={tokenScheduleId || '__none__'}
                    onValueChange={(v) => setTokenScheduleId(v === '__none__' ? '' : v)}
                    data={[
                      { value: '__none__', label: t('common.none') },
                      ...tokenSchedules.map((s) => ({ value: s.id, label: s.name })),
                    ]}
                  />
                )}

                {tokenGates.length > 0 && (
                  <Switch
                    label={t('members.tokenRestrictPerms')}
                    description={t('members.tokenRestrictPermsHint')}
                    checked={tokenRestrictPerms}
                    onCheckedChange={(checked) => {
                      setTokenRestrictPerms(!!checked)
                      if (!checked) setTokenPolicies([])
                    }}
                  />
                )}

                {tokenRestrictPerms && tokenGates.length > 0 && (
                  <div>
                    <p className="text-sm font-semibold mb-2">{t('members.gatePermissions')}</p>
                    <GatePermissionsGrid
                      gates={tokenGates}
                      permissions={tokenPermissions}
                      isChecked={(gateId, code) =>
                        tokenPolicies.some((p) => p.gate_id === gateId && p.permission_code === code)
                      }
                      onToggle={togglePolicy}
                      maxHeight={200}
                    />
                  </div>
                )}

                <Button type="submit" disabled={!tokenLabel.trim()} className="w-full">
                  {t('common.add')}
                </Button>
              </form>
            </div>

            <Separator />

            <div>
              <h4 className="font-semibold mb-3">{t('members.existingTokens')}</h4>
              {tokensLoading ? (
                <Skeleton className="h-10 w-full" />
              ) : tokens.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t('members.noTokens')}</p>
              ) : (
                <div className="space-y-2">
                  {tokens.map((cred) => (
                    <div key={cred.id} className="flex items-center justify-between p-2 border rounded-md">
                      <div className="min-w-0">
                        <p className="text-sm font-medium truncate">{cred.label || '—'}</p>
                        <p className="text-xs text-muted-foreground">
                          {cred.created_at ? new Date(cred.created_at).toLocaleDateString() : '—'}
                          {cred.expires_at && ` → ${new Date(cred.expires_at).toLocaleDateString()}`}
                        </p>
                      </div>
                      <Button variant="ghost" size="icon-sm" onClick={() => handleDeleteToken(cred.id)} className="text-destructive hover:text-destructive">
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
