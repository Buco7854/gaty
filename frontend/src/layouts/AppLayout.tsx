import { NavLink as RouterNavLink, Outlet, useNavigate, useParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '@/store/auth'
import type { WorkspaceWithRole } from '@/types'
import { workspacesApi, memberCredApi } from '@/api'
import type { MemberCredential, CreatedToken } from '@/api'
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
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { findLocalSession } from '@/utils/session'

export default function AppLayout() {
  const { wsId } = useParams<{ wsId?: string }>()
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.user)
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()
  const [wsMenuOpen, setWsMenuOpen] = useState(false)
  const [navOpened, setNavOpened] = useState(false)
  const [tokenModalOpened, { open: openTokenModal, close: closeTokenModal }] = useDisclosure(false)
  const [tokens, setTokens] = useState<MemberCredential[]>([])
  const [tokensLoading, setTokensLoading] = useState(false)
  const [tokenLabel, setTokenLabel] = useState('')
  const [newToken, setNewToken] = useState<CreatedToken | null>(null)

  const isAdmin = isAuthenticated()

  const localSession = useMemo(
    () => (!isAdmin && wsId ? findLocalSession(wsId) : null),
    [wsId, isAdmin]
  )

  const { data: workspaces } = useQuery<WorkspaceWithRole[]>({
    queryKey: ['workspaces'],
    queryFn: workspacesApi.list,
    enabled: isAdmin,
  })

  const currentWs = workspaces?.find((w) => w.id === wsId)

  function handleLogout() {
    logout()
    navigate('/login')
  }

  function handleMemberLogout() {
    const gateId = localSession?.gateId
    if (gateId) {
      localStorage.removeItem(`gaty_session_${gateId}`)
      navigate(wsId ? `/workspaces/${wsId}/gates/${gateId}/public` : '/')
    } else {
      navigate('/')
    }
  }

  function handleLogoClick() {
    if (isAdmin) {
      navigate('/workspaces')
    } else if (wsId && localSession?.gateId) {
      navigate(`/workspaces/${wsId}/gates/${localSession.gateId}/public`)
    }
  }

  // Load member tokens when modal opens
  useEffect(() => {
    if (!tokenModalOpened || !localSession?.access_token) return
    setTokensLoading(true)
    memberCredApi.listTokens(localSession.access_token)
      .then(setTokens)
      .finally(() => setTokensLoading(false))
  }, [tokenModalOpened, localSession?.access_token])

  async function handleCreateToken(e: React.FormEvent) {
    e.preventDefault()
    if (!localSession?.access_token || !tokenLabel.trim()) return
    const created = await memberCredApi.createToken(localSession.access_token, tokenLabel)
    setNewToken(created)
    setTokenLabel('')
    setTokens((prev) => [...prev, created])
  }

  async function handleDeleteToken(credId: string) {
    if (!localSession?.access_token) return
    await memberCredApi.deleteToken(localSession.access_token, credId)
    setTokens((prev) => prev.filter((t) => t.id !== credId))
    if (newToken?.id === credId) setNewToken(null)
  }

  const initials = user?.email?.slice(0, 2).toUpperCase() ?? 'U'

  return (
    <AppShell
      navbar={{ width: 240, breakpoint: 'sm', collapsed: { mobile: !navOpened } }}
      header={{ height: 56, collapsed: false }}
      padding={0}
    >
      {/* Mobile header — burger + logo, hidden on desktop */}
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
              <Text fw={700} size="md" ff="mono">GATY</Text>
            </UnstyledButton>
          </Group>
          <Group gap={4}>
            <LangToggle />
            <ThemeToggle />
          </Group>
        </Group>
      </AppShell.Header>

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
              <Text fw={700} size="md" ff="mono">GATY</Text>
            </UnstyledButton>
          </Group>

          {/* Workspace section */}
          <div style={{ borderBottom: '1px solid var(--mantine-color-default-border)', flexShrink: 0 }}>
            {isAdmin ? (
              currentWs ? (
                <Menu opened={wsMenuOpen} onChange={setWsMenuOpen} width={220} shadow="md">
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
                    <Divider />
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
                  leftSection={<LayoutGrid size={16} />}
                  onClick={() => setNavOpened(false)}
                />
                {(isAdmin || localSession?.role === 'ADMIN' || localSession?.role === 'OWNER') && (
                  <>
                    <NavLink
                      component={RouterNavLink as React.FC}
                      to={`/workspaces/${wsId}/members`}
                      label={t('members.title')}
                      leftSection={<Users size={16} />}
                      onClick={() => setNavOpened(false)}
                    />
                    <NavLink
                      component={RouterNavLink as React.FC}
                      to={`/workspaces/${wsId}/settings`}
                      label={t('settings.title')}
                      leftSection={<Settings size={16} />}
                      onClick={() => setNavOpened(false)}
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
                <Group gap="xs" style={{ minWidth: 0 }}>
                  <Avatar size={26} color="indigo" radius="xl">{initials}</Avatar>
                  <Text size="xs" c="dimmed" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {user?.email}
                  </Text>
                </Group>
                <Group gap={4} wrap="nowrap">
                  <LangToggle />
                  <ThemeToggle />
                  <Tooltip label={t('auth.signOut')}>
                    <ActionIcon variant="subtle" color="gray" size="sm" onClick={handleLogout}>
                      <LogOut size={14} />
                    </ActionIcon>
                  </Tooltip>
                </Group>
              </Group>
            ) : (
              <Group justify="space-between" wrap="nowrap">
                <Group gap="xs" style={{ minWidth: 0 }}>
                  <LangToggle />
                  <ThemeToggle />
                </Group>
                <Group gap={4} wrap="nowrap">
                  <Tooltip label={t('members.apiTokens')}>
                    <ActionIcon variant="subtle" color="gray" size="sm" onClick={openTokenModal}>
                      <KeyRound size={14} />
                    </ActionIcon>
                  </Tooltip>
                  <Tooltip label={t('auth.signOut')}>
                    <ActionIcon variant="subtle" color="gray" size="sm" onClick={handleMemberLogout}>
                      <LogOut size={14} />
                    </ActionIcon>
                  </Tooltip>
                </Group>
              </Group>
            )}
          </div>
        </Stack>
      </AppShell.Navbar>

      <AppShell.Main style={{ overflow: 'auto' }}>
        <Outlet />
      </AppShell.Main>

      {/* Member API token management modal */}
      <Modal
        opened={tokenModalOpened}
        onClose={() => { closeTokenModal(); setNewToken(null); setTokenLabel('') }}
        title={t('members.apiTokens')}
        size="sm"
      >
        <Stack gap="md">
          {newToken && (
            <Alert
              color="green"
              variant="light"
              withCloseButton
              onClose={() => setNewToken(null)}
              title={t('members.tokenCreated')}
            >
              <Text size="xs" mb={4}>{t('members.tokenCreatedHint')}</Text>
              <Group gap={4} align="center">
                <Code style={{ flex: 1, wordBreak: 'break-all', fontSize: 11 }}>{newToken.token}</Code>
                <CopyButton value={newToken.token}>
                  {({ copied, copy }) => (
                    <Tooltip label={copied ? t('common.copied') : t('common.copy')}>
                      <ActionIcon size="sm" variant="subtle" onClick={copy}>
                        {copied ? <Check size={12} /> : <Copy size={12} />}
                      </ActionIcon>
                    </Tooltip>
                  )}
                </CopyButton>
              </Group>
            </Alert>
          )}

          <form onSubmit={handleCreateToken}>
            <Group gap="xs" align="flex-end">
              <TextInput
                label={t('members.tokenLabel')}
                placeholder={t('members.tokenLabelPlaceholder')}
                value={tokenLabel}
                onChange={(e) => setTokenLabel(e.target.value)}
                size="xs"
                style={{ flex: 1 }}
              />
              <Button type="submit" size="xs" disabled={!tokenLabel.trim()}>
                {t('common.add')}
              </Button>
            </Group>
          </form>

          {tokensLoading ? (
            <Skeleton h={40} />
          ) : tokens.length === 0 ? (
            <Text size="sm" c="dimmed">{t('members.noTokens')}</Text>
          ) : (
            <Stack gap={4}>
              {tokens.map((cred) => (
                <Group key={cred.id} justify="space-between" wrap="nowrap" p={6}
                  style={{ border: '1px solid var(--mantine-color-default-border)', borderRadius: 6 }}>
                  <Stack gap={0} style={{ minWidth: 0 }}>
                    <Text size="xs" fw={500} truncate>{cred.label}</Text>
                    <Text size="xs" c="dimmed">{new Date(cred.created_at).toLocaleDateString()}</Text>
                  </Stack>
                  <ActionIcon size="sm" color="red" variant="subtle" onClick={() => handleDeleteToken(cred.id)}>
                    <Trash2 size={13} />
                  </ActionIcon>
                </Group>
              ))}
            </Stack>
          )}
        </Stack>
      </Modal>
    </AppShell>
  )
}
