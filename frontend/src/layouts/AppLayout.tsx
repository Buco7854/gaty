import { NavLink as RouterNavLink, Outlet, useNavigate } from 'react-router'
import { useAuthStore } from '@/store/auth'
import { credentialsApi, gatesApi, schedulesApi } from '@/api'
import type { MemberCredential, CreatedToken } from '@/api'
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
  DoorOpen,
  KeyRound,
  Copy,
  Check,
  Trash2,
  CalendarClock,
} from 'lucide-react'
import { useEffect, useState } from 'react'

export default function AppLayout() {
  const { t } = useTranslation()
  const tokenPermissions = useGatePermissions()
  const session = useAuthStore((s) => s.session)
  const logout = useAuthStore((s) => s.logout)
  const member = session?.type === 'member' ? session.member : null
  const isAdmin = member?.role === 'ADMIN'
  const navigate = useNavigate()
  const [navOpened, setNavOpened] = useState(false)
  const [tokenModalOpened, { open: openTokenModal, close: closeTokenModal }] = useDisclosure(false)
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

  async function handleLogout() {
    await logout()
    navigate('/login')
  }

  function handleLogoClick() {
    navigate('/gates')
  }

  // Load tokens, gates and schedules when modal opens
  useEffect(() => {
    if (!tokenModalOpened || !member) return
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
  }, [tokenModalOpened, member])

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

  const initials = member?.username?.slice(0, 2).toUpperCase() ?? 'U'

  // Shared nav body: nav links + footer
  const renderNavBody = () => (
    <>
      {/* Navigation */}
      <ScrollArea style={{ flex: 1 }}>
        <Stack gap={2} p="xs">
          <NavLink
            component={RouterNavLink as React.FC}
            to="/gates"
            end
            label={t('gates.title')}
            leftSection={<LayoutGrid size={18} />}
            onClick={() => setNavOpened(false)}
            styles={{ root: { borderRadius: 'var(--mantine-radius-md)', paddingTop: 8, paddingBottom: 8 } }}
          />
          <NavLink
            component={RouterNavLink as React.FC}
            to="/schedules"
            label={t('schedules.title')}
            leftSection={<CalendarClock size={18} />}
            onClick={() => setNavOpened(false)}
            styles={{ root: { borderRadius: 'var(--mantine-radius-md)', paddingTop: 8, paddingBottom: 8 } }}
          />
          {isAdmin && (
            <>
              <Divider my={4} label={<Text size="xs" c="dimmed" fw={500}>{t('common.administration')}</Text>} />
              <NavLink
                component={RouterNavLink as React.FC}
                to="/members"
                label={t('members.title')}
                leftSection={<Users size={18} />}
                onClick={() => setNavOpened(false)}
                styles={{ root: { borderRadius: 'var(--mantine-radius-md)', paddingTop: 8, paddingBottom: 8 } }}
              />
              <NavLink
                component={RouterNavLink as React.FC}
                to="/settings"
                label={t('settings.title')}
                leftSection={<Settings size={18} />}
                onClick={() => setNavOpened(false)}
                styles={{ root: { borderRadius: 'var(--mantine-radius-md)', paddingTop: 8, paddingBottom: 8 } }}
              />
            </>
          )}
        </Stack>
      </ScrollArea>

      {/* Footer */}
      <div style={{ borderTop: '1px solid var(--mantine-color-default-border)', padding: '8px 12px', flexShrink: 0 }}>
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
              <Menu.Label style={{ fontSize: 10 }}>{member?.username}</Menu.Label>
              <Menu.Item leftSection={<KeyRound size={14} />} onClick={openTokenModal}>
                {t('members.apiTokens')}
              </Menu.Item>
              <Divider my={4} />
              <Menu.Item leftSection={<LogOut size={14} />} color="red" onClick={handleLogout}>
                {t('auth.signOut')}
              </Menu.Item>
            </Menu.Dropdown>
          </Menu>
        </Group>
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
        onClose={() => { closeTokenModal(); setNewToken(null); resetTokenForm() }}
        title={t('members.apiTokens')}
        size="lg"
      >
        <Stack gap="xl">
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
