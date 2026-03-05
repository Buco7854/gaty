import { NavLink as RouterNavLink, Outlet, useNavigate, useParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '@/store/auth'
import type { WorkspaceWithRole } from '@/types'
import { workspacesApi } from '@/api'
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
} from '@mantine/core'
import {
  LayoutGrid,
  Users,
  Settings,
  LogOut,
  ChevronDown,
  Home,
  DoorOpen,
} from 'lucide-react'
import { useMemo, useState } from 'react'
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
    if (localSession?.gateId) {
      localStorage.removeItem(`gaty_session_${localSession.gateId}`)
      navigate(`/unlock/${localSession.gateId}`)
    } else {
      navigate('/unlock')
    }
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
              onClick={() => isAdmin ? navigate('/workspaces') : localSession?.gateId && navigate(`/unlock/${localSession.gateId}`)}
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
              onClick={() => isAdmin ? navigate('/workspaces') : localSession?.gateId && navigate(`/unlock/${localSession.gateId}`)}
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
                <Tooltip label={t('auth.signOut')}>
                  <ActionIcon variant="subtle" color="gray" size="sm" onClick={handleMemberLogout}>
                    <LogOut size={14} />
                  </ActionIcon>
                </Tooltip>
              </Group>
            )}
          </div>
        </Stack>
      </AppShell.Navbar>

      <AppShell.Main style={{ overflow: 'auto' }}>
        <Outlet />
      </AppShell.Main>
    </AppShell>
  )
}
