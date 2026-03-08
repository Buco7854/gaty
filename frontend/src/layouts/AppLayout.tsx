import { NavLink as RouterNavLink, Outlet, useNavigate, useParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '@/store/auth'
import type { WorkspaceWithRole } from '@/types'
import { workspacesApi, memberCredApi, workspaceCredApi, gatesApi, schedulesApi } from '@/api'
import type { MemberCredential, CreatedToken, MyEffectiveAuthConfig } from '@/api'
import type { Gate, AccessSchedule } from '@/types'
import { GatePermissionsGrid, useGatePermissions } from '@/components/GatePermissionsGrid'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'
import { useTranslation } from 'react-i18next'
import {
  AppShell,
  Burger,
  NavLink,
  Stack,
  Group,
  Text,
  Avatar,
  Menu,
  UnstyledButton,
  Divider,
  Tooltip,
  ActionIcon,
  ScrollArea,
  Modal,
  TextInput,
  Button,
  Code,
  CopyButton,
  Alert,
  Skeleton,
  Drawer,
  Switch,
  Select,
} from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import {
  LayoutGrid,
  Users,
  Settings,
  LogOut,
  ChevronDown,
  Home,
  DoorOpen,
  KeyRound,
  Copy,
  Check,
  Trash2,
  CalendarClock,
  User,
} from 'lucide-react'
import { useEffect, useState } from 'react'

export default function AppLayout() {
  const { wsId } = useParams<{ wsId?: string }>()
  const { t } = useTranslation()
  const tokenPermissions = useGatePermissions()
  const session = useAuthStore((s) => s.session)
  const logout = useAuthStore((s) => s.logout)
  const user = session?.type === 'global' ? session.user : null
  const isGlobalAuth = session?.type === 'global'
  const navigate = useNavigate()
  const [wsMenuOpen, setWsMenuOpen] = useState(false)
  const [navOpened, setNavOpened] = useState(false)
  const [tokenModalOpened, { open: openTokenModal, close: closeTokenModal }] = useDisclosure(false)
  const [tokens, setTokens] = useState<MemberCredential[]>([])
  const [tokensLoading, setTokensLoading] = useState(false)
  const [tokenLabel, setTokenLabel] = useState('')
  const [tokenExpiresAt, setTokenExpiresAt] = useState('')
  const [newToken, setNewToken] = useState<CreatedToken | null>(null)
  const [memberAuthConfig, setMemberAuthConfig] = useState<MyEffectiveAuthConfig | null>(null)
  const [tokenGates, setTokenGates] = useState<Gate[]>([])
  const [tokenSchedules, setTokenSchedules] = useState<AccessSchedule[]>([])
  const [tokenScheduleId, setTokenScheduleId] = useState('')
  const [tokenRestrictPerms, setTokenRestrictPerms] = useState(false)
  const [tokenPolicies, setTokenPolicies] = useState<{ gate_id: string; permission_code: string }[]>([])

  const isAdmin = isGlobalAuth

  const localSession = !isAdmin && wsId && session?.type === 'local' ? session : null

  const { data: workspaces } = useQuery<WorkspaceWithRole[]>({
    queryKey: ['workspaces'],
    queryFn: workspacesApi.list,
    enabled: isAdmin,
  })

  const currentWs = workspaces?.find((w) => w.id === wsId)

  async function handleLogout() {
    await logout()
    navigate('/login')
  }

  function handleMemberLogout() {
    useAuthStore.getState().clearSession()
    navigate(wsId ? `/workspaces/${wsId}/login` : '/login')
  }

  function handleLogoClick() {
    if (isAdmin) {
      navigate('/workspaces')
    } else if (wsId) {
      navigate(`/workspaces/${wsId}`)
    }
  }

  // Load tokens, auth config, gates and schedules when modal opens
  useEffect(() => {
    if (!tokenModalOpened) return
    setTokensLoading(true)
    setMemberAuthConfig(null)
    if (isAdmin && wsId) {
      Promise.all([
        workspaceCredApi.listTokens(wsId),
        workspaceCredApi.getMyAuthConfig(wsId).catch(() => null),
        gatesApi.list(wsId).catch(() => []),
        schedulesApi.listMine(wsId).catch(() => []),
      ]).then(([tks, cfg, gates, schedules]) => {
        setTokens(tks)
        setMemberAuthConfig(cfg)
        setTokenGates(gates as Gate[])
        setTokenSchedules(schedules as AccessSchedule[])
      }).finally(() => setTokensLoading(false))
    } else if (localSession && wsId) {
      Promise.all([
        memberCredApi.listTokens(),
        workspaceCredApi.getMyAuthConfig(wsId).catch(() => null),
        gatesApi.list(wsId).catch(() => []),
        schedulesApi.listMine(wsId).catch(() => []),
      ]).then(([tks, cfg, gates, schedules]) => {
        setTokens(tks)
        setMemberAuthConfig(cfg)
        setTokenGates(gates as Gate[])
        setTokenSchedules(schedules as AccessSchedule[])
      }).finally(() => setTokensLoading(false))
    } else {
      setTokensLoading(false)
    }
  }, [tokenModalOpened, isAdmin, wsId, localSession])

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
    let created: CreatedToken
    if (isAdmin && wsId) {
      const policies = tokenRestrictPerms && tokenPolicies.length > 0 ? tokenPolicies : undefined
      created = await workspaceCredApi.createToken(wsId, tokenLabel, tokenExpiresAt || undefined, policies, tokenScheduleId || undefined)
      const updated = await workspaceCredApi.listTokens(wsId)
      setTokens(updated)
    } else if (localSession) {
      const policies = tokenRestrictPerms && tokenPolicies.length > 0 ? tokenPolicies : undefined
      created = await memberCredApi.createToken(tokenLabel, tokenExpiresAt || undefined, policies, tokenScheduleId || undefined)
      const updated = await memberCredApi.listTokens()
      setTokens(updated)
    } else {
      return
    }
    setNewToken(created)
    resetTokenForm()
  }

  async function handleDeleteToken(credId: string) {
    if (isAdmin && wsId) {
      await workspaceCredApi.deleteToken(wsId, credId)
    } else if (localSession) {
      await memberCredApi.deleteToken(credId)
    } else {
      return
    }
    setTokens((prev) => prev.filter((t) => t.id !== credId))
    if (newToken?.id === credId) setNewToken(null)
  }

  const initials = user?.email?.slice(0, 2).toUpperCase() ?? 'U'

  // Shared nav body: workspace selector + nav links + footer
  const renderNavBody = () => (
    <>
      {/* Workspace section */}
      <div style={{ borderBottom: '1px solid var(--mantine-color-default-border)', flexShrink: 0 }}>
        {isAdmin ? (
          currentWs ? (
            <Menu opened={wsMenuOpen} onChange={setWsMenuOpen} width={220} shadow="md" styles={{ dropdown: { padding: 4 }, item: { borderRadius: 'var(--mantine-radius-sm)', marginBottom: 2 } }}>
              <Menu.Target>
                <UnstyledButton
                  px="md"
                  py="sm"
                  style={{ width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}
                >
                  <Group gap="xs">
                    <Avatar size={22} color="indigo" radius="sm">
                      {currentWs.name[0].toUpperCase()}
                    </Avatar>
                    <Text size="sm" fw={500} style={{ maxWidth: 140, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {currentWs.name}
                    </Text>
                  </Group>
                  <ChevronDown size={14} />
                </UnstyledButton>
              </Menu.Target>
              <Menu.Dropdown>
                {workspaces?.map((w) => (
                  <Menu.Item
                    key={w.id}
                    leftSection={<Avatar size={16} color="indigo" radius="xs">{w.name[0].toUpperCase()}</Avatar>}
                    onClick={() => { navigate(`/workspaces/${w.id}`); setWsMenuOpen(false); setNavOpened(false) }}
                  >
                    <Text size="sm" truncate>{w.name}</Text>
                  </Menu.Item>
                ))}
                <Divider my={4} />
                <Menu.Item
                  leftSection={<Home size={14} />}
                  onClick={() => { navigate('/workspaces'); setWsMenuOpen(false); setNavOpened(false) }}
                >
                  <Text size="sm">{t('workspaces.title')}</Text>
                </Menu.Item>
              </Menu.Dropdown>
            </Menu>
          ) : (
            <NavLink
              component={RouterNavLink as React.FC}
              to="/workspaces"
              label={t('workspaces.title')}
              leftSection={<Home size={16} />}
              px="md"
              py="sm"
              onClick={() => setNavOpened(false)}
            />
          )
        ) : (
          // Local member: static workspace indicator
          <Group px="md" py="sm" gap="xs">
            <Avatar size={22} color="indigo" radius="sm">
              <DoorOpen size={12} />
            </Avatar>
            <Text size="sm" fw={500} c="dimmed" style={{ maxWidth: 160, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {t('portal.myGates')}
            </Text>
          </Group>
        )}
      </div>

      {/* Navigation */}
      <ScrollArea style={{ flex: 1 }}>
        {wsId && (
          <Stack gap={2} p="xs">
            <NavLink
              component={RouterNavLink as React.FC}
              to={`/workspaces/${wsId}`}
              end
              label={t('gates.title')}
              leftSection={<LayoutGrid size={18} />}
              onClick={() => setNavOpened(false)}
              styles={{ root: { borderRadius: 'var(--mantine-radius-md)', paddingTop: 8, paddingBottom: 8 } }}
            />
            <NavLink
              component={RouterNavLink as React.FC}
              to={`/workspaces/${wsId}/schedules`}
              label={t('schedules.title')}
              leftSection={<CalendarClock size={18} />}
              onClick={() => setNavOpened(false)}
              styles={{ root: { borderRadius: 'var(--mantine-radius-md)', paddingTop: 8, paddingBottom: 8 } }}
            />
            {(isAdmin || localSession?.role === 'ADMIN' || localSession?.role === 'OWNER') && (
              <>
                <Divider my={4} label={<Text size="xs" c="dimmed" fw={500}>{t('common.administration')}</Text>} />
                <NavLink
                  component={RouterNavLink as React.FC}
                  to={`/workspaces/${wsId}/members`}
                  label={t('members.title')}
                  leftSection={<Users size={18} />}
                  onClick={() => setNavOpened(false)}
                  styles={{ root: { borderRadius: 'var(--mantine-radius-md)', paddingTop: 8, paddingBottom: 8 } }}
                />
                <NavLink
                  component={RouterNavLink as React.FC}
                  to={`/workspaces/${wsId}/settings`}
                  label={t('settings.title')}
                  leftSection={<Settings size={18} />}
                  onClick={() => setNavOpened(false)}
                  styles={{ root: { borderRadius: 'var(--mantine-radius-md)', paddingTop: 8, paddingBottom: 8 } }}
                />
              </>
            )}
          </Stack>
        )}
      </ScrollArea>

      {/* Footer */}
      <div style={{ borderTop: '1px solid var(--mantine-color-default-border)', padding: '8px 12px', flexShrink: 0 }}>
        {isAdmin ? (
          <Group justify="space-between" wrap="nowrap">
            <Group gap={4} wrap="nowrap">
              <LangToggle />
              <ThemeToggle />
            </Group>
            <Menu position="top-end" width={190} styles={{ dropdown: { padding: 4 }, item: { borderRadius: 'var(--mantine-radius-sm)', marginBottom: 2 } }}>
              <Menu.Target>
                <UnstyledButton style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                  <Avatar size={24} color="indigo" radius="xl">{initials}</Avatar>
                  <ChevronDown size={11} style={{ color: 'var(--mantine-color-dimmed)' }} />
                </UnstyledButton>
              </Menu.Target>
              <Menu.Dropdown>
                <Menu.Label style={{ fontSize: 10 }}>{user?.email}</Menu.Label>
                {wsId && (
                  <Menu.Item leftSection={<KeyRound size={14} />} onClick={openTokenModal}>
                    {t('members.apiTokens')}
                  </Menu.Item>
                )}
                <Divider my={4} />
                <Menu.Item leftSection={<LogOut size={14} />} color="red" onClick={handleLogout}>
                  {t('auth.signOut')}
                </Menu.Item>
              </Menu.Dropdown>
            </Menu>
          </Group>
        ) : (
          <Group justify="space-between" wrap="nowrap">
            <Group gap={4} wrap="nowrap">
              <LangToggle />
              <ThemeToggle />
            </Group>
            <Menu position="top-end" width={160} styles={{ dropdown: { padding: 4 }, item: { borderRadius: 'var(--mantine-radius-sm)', marginBottom: 2 } }}>
              <Menu.Target>
                <ActionIcon variant="subtle" color="gray" size="sm">
                  <User size={14} />
                </ActionIcon>
              </Menu.Target>
              <Menu.Dropdown>
                <Menu.Item leftSection={<KeyRound size={14} />} onClick={openTokenModal}>
                  {t('members.apiTokens')}
                </Menu.Item>
                <Divider my={4} />
                <Menu.Item leftSection={<LogOut size={14} />} color="red" onClick={handleMemberLogout}>
                  {t('auth.signOut')}
                </Menu.Item>
              </Menu.Dropdown>
            </Menu>
          </Group>
        )}
      </div>
    </>
  )

  return (
    <AppShell
      navbar={{ width: 280, breakpoint: 'sm', collapsed: { mobile: true } }}
      header={{ height: { base: 56, sm: 0 } }}
      padding={0}
    >
      {/* Mobile header — burger + logo (hidden on desktop via height:0 + hiddenFrom) */}
      <AppShell.Header hiddenFrom="sm" style={{ borderBottom: '1px solid var(--mantine-color-default-border)' }}>
        <Group h="100%" px="md" justify="space-between">
          <Group gap="xs">
            <Burger opened={navOpened} onClick={() => setNavOpened((o) => !o)} size="sm" />
            <UnstyledButton
              onClick={handleLogoClick}
              style={{ display: 'flex', alignItems: 'center', gap: 8 }}
            >
              <Avatar size={24} color="indigo" radius="md">
                <DoorOpen size={12} />
              </Avatar>
              <Text fw={700} size="md" ff="mono">GATIE</Text>
            </UnstyledButton>
          </Group>
          <Group gap={4}>
            <LangToggle />
            <ThemeToggle />
          </Group>
        </Group>
      </AppShell.Header>

      {/* Mobile nav — Drawer rendered in portal, reliable z-index */}
      <Drawer
        opened={navOpened}
        onClose={() => setNavOpened(false)}
        size={280}
        padding={0}
        withCloseButton={false}
        styles={{ body: { padding: 0, height: '100%', display: 'flex', flexDirection: 'column' } }}
      >
        <Stack gap={0} style={{ flex: 1 }}>
          <Group
            px="md"
            h={56}
            style={{ borderBottom: '1px solid var(--mantine-color-default-border)', flexShrink: 0 }}
          >
            <UnstyledButton
              onClick={() => { handleLogoClick(); setNavOpened(false) }}
              style={{ display: 'flex', alignItems: 'center', gap: 8 }}
            >
              <Avatar size={28} color="indigo" radius="md">
                <DoorOpen size={14} />
              </Avatar>
              <Text fw={700} size="md" ff="mono">GATIE</Text>
            </UnstyledButton>
          </Group>
          {renderNavBody()}
        </Stack>
      </Drawer>

      {/* Desktop sidebar */}
      <AppShell.Navbar>
        <Stack gap={0} h="100%">
          {/* Logo — desktop only */}
          <Group
            px="md"
            h={56}
            visibleFrom="sm"
            style={{ borderBottom: '1px solid var(--mantine-color-default-border)', flexShrink: 0 }}
          >
            <UnstyledButton
              onClick={handleLogoClick}
              style={{ display: 'flex', alignItems: 'center', gap: 8 }}
            >
              <Avatar size={28} color="indigo" radius="md">
                <DoorOpen size={14} />
              </Avatar>
              <Text fw={700} size="md" ff="mono">GATIE</Text>
            </UnstyledButton>
          </Group>
          {renderNavBody()}
        </Stack>
      </AppShell.Navbar>

      <AppShell.Main style={{ overflow: 'auto' }}>
        <Outlet />
      </AppShell.Main>

      {/* API token management modal */}
      <Modal
        opened={tokenModalOpened}
        onClose={() => { closeTokenModal(); setNewToken(null); resetTokenForm(); setMemberAuthConfig(null) }}
        title={t('members.apiTokens')}
        size="lg"
      >
        <Stack gap="xl">
          {memberAuthConfig?.api_token === false && (
            <Alert color="orange" variant="light" title={t('members.tokenDisabled')}>
              <Text size="sm">{t('members.tokenDisabledHint')}</Text>
            </Alert>
          )}

          {newToken && (
            <Alert
              color="green"
              variant="light"
              withCloseButton
              onClose={() => setNewToken(null)}
              title={t('members.tokenCreated')}
            >
              <Text size="sm" mb={4}>{t('members.tokenCreatedHint')}</Text>
              <Group gap={4} align="center">
                <Code style={{ flex: 1, wordBreak: 'break-all', fontSize: 12 }}>{newToken.token}</Code>
                <CopyButton value={newToken.token}>
                  {({ copied, copy }) => (
                    <Tooltip label={copied ? t('common.copied') : t('common.copy')}>
                      <ActionIcon size="sm" variant="subtle" onClick={copy}>
                        {copied ? <Check size={14} /> : <Copy size={14} />}
                      </ActionIcon>
                    </Tooltip>
                  )}
                </CopyButton>
              </Group>
            </Alert>
          )}

          {memberAuthConfig?.api_token !== false && (
            <div>
              <Text fw={600} mb="sm">{t('members.newToken')}</Text>
              <form onSubmit={handleCreateToken}>
                <Stack gap="sm">
                  <TextInput
                    label={t('members.tokenLabel')}
                    placeholder={t('members.tokenLabelPlaceholder')}
                    value={tokenLabel}
                    onChange={(e) => setTokenLabel(e.target.value)}
                    withAsterisk
                  />
                  <TextInput
                    label={`${t('members.tokenExpiresAt')} (${t('common.optional')})`}
                    type="date"
                    value={tokenExpiresAt}
                    onChange={(e) => setTokenExpiresAt(e.target.value)}
                  />

                  {tokenSchedules.length > 0 && (
                    <Select
                      label={t('members.tokenSchedule')}
                      description={t('members.tokenScheduleHint')}
                      value={tokenScheduleId}
                      onChange={(v) => setTokenScheduleId(v ?? '')}
                      data={[
                        { value: '', label: t('common.none') },
                        ...tokenSchedules.map((s) => ({ value: s.id, label: s.name })),
                      ]}
                      clearable
                    />
                  )}

                  {tokenGates.length > 0 && (
                    <Switch
                      label={t('members.tokenRestrictPerms')}
                      description={t('members.tokenRestrictPermsHint')}
                      checked={tokenRestrictPerms}
                      onChange={(e) => {
                        setTokenRestrictPerms(e.currentTarget.checked)
                        if (!e.currentTarget.checked) setTokenPolicies([])
                      }}
                    />
                  )}

                  {tokenRestrictPerms && tokenGates.length > 0 && (
                    <div>
                      <Text size="sm" fw={600} mb={6}>{t('members.gatePermissions')}</Text>
                      <GatePermissionsGrid
                        gates={tokenGates}
                        permissions={tokenPermissions}
                        isChecked={(gateId, code) =>
                          tokenPolicies.some((p) => p.gate_id === gateId && p.permission_code === code)
                        }
                        onToggle={togglePolicy}
                        withColumnSelect
                        maxHeight={200}
                      />
                    </div>
                  )}

                  <Button type="submit" disabled={!tokenLabel.trim()} fullWidth>
                    {t('common.add')}
                  </Button>
                </Stack>
              </form>
            </div>
          )}

          <div>
            <Text fw={600} mb="sm">{t('members.existingTokens')}</Text>
            {tokensLoading ? (
              <Skeleton h={40} />
            ) : tokens.length === 0 ? (
              <Text size="sm" c="dimmed">{t('members.noTokens')}</Text>
            ) : (
              <Stack gap={4}>
                {tokens.map((cred) => (
                  <Group key={cred.id} justify="space-between" wrap="nowrap" p={8}
                    style={{ border: '1px solid var(--mantine-color-default-border)', borderRadius: 6 }}>
                    <Stack gap={0} style={{ minWidth: 0 }}>
                      <Text size="sm" fw={500} truncate>{cred.label || '—'}</Text>
                      <Text size="xs" c="dimmed">
                        {cred.created_at ? new Date(cred.created_at).toLocaleDateString() : '—'}
                        {cred.expires_at && ` → ${new Date(cred.expires_at).toLocaleDateString()}`}
                      </Text>
                    </Stack>
                    <ActionIcon size="sm" color="red" variant="subtle" onClick={() => handleDeleteToken(cred.id)}>
                      <Trash2 size={14} />
                    </ActionIcon>
                  </Group>
                ))}
              </Stack>
            )}
          </div>
        </Stack>
      </Modal>
    </AppShell>
  )
}
