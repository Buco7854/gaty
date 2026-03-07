import { useState } from 'react'
import { useParams } from 'react-router'
import { useQuery, useMutation, useQueryClient, useQueries } from '@tanstack/react-query'
import { membersApi, gatesApi, policiesApi, schedulesApi } from '@/api'
import type { WorkspaceMembership, Gate, MembershipPolicy, AccessSchedule } from '@/types'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Group, Button, Modal, Stack, Alert, Tabs,
  TextInput, PasswordInput, Select, Badge, Avatar, ActionIcon, Center, Skeleton,
  Checkbox, Paper, SegmentedControl, Drawer,
} from '@mantine/core'
import { GatePermissionsGrid, useGatePermissions } from '@/components/GatePermissionsGrid'
import { useDisclosure, useMediaQuery } from '@mantine/hooks'
import { UserPlus, Trash2, Users, AlertCircle, Settings2, X, Pencil } from 'lucide-react'
import { notifySuccess, notifyError, extractApiError } from '@/lib/notify'
import { QueryError } from '@/components/QueryError'

const ROLE_COLOR: Record<string, string> = {
  OWNER: 'yellow',
  ADMIN: 'blue',
  MEMBER: 'gray',
}

type PermCode = 'gate:read_status' | 'gate:trigger_open' | 'gate:trigger_close' | 'gate:manage'

const AUTH_METHODS = [
  { key: 'password', labelKey: 'settings.passwordAuth' },
  { key: 'sso', labelKey: 'settings.ssoAuth' },
  { key: 'api_token', labelKey: 'settings.apiTokenAuth' },
] as const

// ---------- Schedule tab for member drawer ----------

function MemberSchedulesTab({
  wsId,
  member,
  gates,
  schedules,
}: {
  wsId: string
  member: WorkspaceMembership
  gates: Gate[]
  schedules: AccessSchedule[]
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()

  const scheduleQueries = useQueries({
    queries: gates.map((gate) => ({
      queryKey: ['member-gate-schedule', wsId, gate.id, member.id],
      queryFn: async () => {
        try {
          return await policiesApi.getMemberGateSchedule(wsId, gate.id, member.id)
        } catch (e: unknown) {
          if ((e as { response?: { status?: number } })?.response?.status === 404) return null
          throw e
        }
      },
    })),
  })

  const scheduleSelectData = [
    { value: '', label: t('common.none') },
    ...schedules.map((s) => ({ value: s.id, label: s.name })),
  ]

  async function handleScheduleChange(gate: Gate, scheduleId: string) {
    try {
      if (scheduleId === '') {
        await policiesApi.removeMemberGateSchedule(wsId, gate.id, member.id)
      } else {
        await policiesApi.setMemberGateSchedule(wsId, gate.id, member.id, scheduleId)
      }
      qc.invalidateQueries({ queryKey: ['member-gate-schedule', wsId, gate.id, member.id] })
      notifySuccess(t('common.saved'))
    } catch (err) {
      notifyError(err, t('common.error'))
    }
  }

  if (gates.length === 0) {
    return <Text size="sm" c="dimmed">{t('gates.noGates')}</Text>
  }

  return (
    <Stack gap="sm">
      <Text size="xs" c="dimmed">{t('members.schedulesHint')}</Text>
      {gates.map((gate, i) => {
        const query = scheduleQueries[i]
        const currentSchedule = query.data as AccessSchedule | null | undefined
        const currentValue = currentSchedule?.id ?? ''
        return (
          <Group key={gate.id} justify="space-between" align="center">
            <Text size="sm" truncate maw={140}>{gate.name}</Text>
            <Select
              size="xs"
              value={query.isLoading ? null : currentValue}
              onChange={(v) => handleScheduleChange(gate, v ?? '')}
              data={scheduleSelectData}
              disabled={query.isLoading}
              placeholder={query.isLoading ? t('common.loading') : undefined}
              style={{ width: 180 }}
              comboboxProps={{ withinPortal: true }}
            />
          </Group>
        )
      })}
    </Stack>
  )
}

// ---------- Member settings Drawer ----------

function MemberSettingsDrawer({
  wsId,
  member,
  gates,
  opened,
  onClose,
}: {
  wsId: string
  member: WorkspaceMembership
  gates: Gate[]
  opened: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const gatePermissions = useGatePermissions()
  const qc = useQueryClient()

  const { data: schedules = [] } = useQuery<AccessSchedule[]>({
    queryKey: ['schedules', wsId],
    queryFn: () => schedulesApi.list(wsId),
    enabled: opened,
  })

  const updateAuth = useMutation({
    mutationFn: (cfg: Record<string, unknown>) =>
      membersApi.update(wsId, member.id, { auth_config: cfg }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['members', wsId] }); notifySuccess(t('common.saved')) },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  function getAuthValue(key: string): string {
    const val = (member.auth_config as Record<string, unknown> | null)?.[key]
    if (val === true) return 'on'
    if (val === false) return 'off'
    return 'inherit'
  }

  function setAuthValue(key: string, value: string) {
    const mapped: boolean | null = value === 'on' ? true : value === 'off' ? false : null
    const current = (member.auth_config ?? {}) as Record<string, unknown>
    updateAuth.mutate({ ...current, [key]: mapped })
  }

  const { data: policies = [] } = useQuery<MembershipPolicy[]>({
    queryKey: ['member-policies', wsId, member.id],
    queryFn: () => policiesApi.listByMembership(wsId, member.id),
    enabled: opened,
  })

  const policySet = new Set(policies.map((p) => `${p.gate_id}:${p.permission_code}`))
  const hasPermission = (gateId: string, permCode: string) => policySet.has(`${gateId}:${permCode}`)

  function invalidatePolicies() {
    qc.invalidateQueries({ queryKey: ['member-policies', wsId, member.id] })
  }

  async function toggle(gateId: string, permCode: string, on: boolean) {
    try {
      if (on) await policiesApi.revoke(wsId, gateId, member.id, permCode)
      else await policiesApi.grant(wsId, gateId, member.id, permCode)
      invalidatePolicies()
    } catch (err) {
      notifyError(err, t('common.error'))
    }
  }

  const memberName = member.display_name ?? member.local_username ?? member.user_email ?? member.id.slice(0, 8)

  return (
    <Drawer
      opened={opened}
      onClose={onClose}
      title={
        <Group gap="sm">
          <Avatar color="indigo" radius="xl" size={28}>{memberName[0].toUpperCase()}</Avatar>
          <Text fw={600} size="sm">{memberName}</Text>
        </Group>
      }
      position="right"
      size="md"
      styles={{ body: { padding: 0 } }}
    >
      <Tabs defaultValue="permissions">
        <Tabs.List px="md" pt="xs">
          <Tabs.Tab value="permissions">{t('members.gatePermissions')}</Tabs.Tab>
          <Tabs.Tab value="schedules">{t('members.schedules')}</Tabs.Tab>
          <Tabs.Tab value="auth">{t('members.authOverrides')}</Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="permissions" p="md">
          <Text size="xs" c="dimmed" mb="sm">{t('members.gatePermissionsHint')}</Text>
          {gates.length === 0 ? (
            <Text size="sm" c="dimmed">{t('gates.noGates')}</Text>
          ) : (
            <GatePermissionsGrid
              gates={gates}
              permissions={gatePermissions}
              isChecked={hasPermission}
              onToggle={(gateId, code) => toggle(gateId, code, hasPermission(gateId, code))}
              withColumnSelect
            />
          )}
        </Tabs.Panel>

        <Tabs.Panel value="schedules" p="md">
          <MemberSchedulesTab wsId={wsId} member={member} gates={gates} schedules={schedules} />
        </Tabs.Panel>

        <Tabs.Panel value="auth" p="md">
          <Stack gap="md">
            <Text size="xs" c="dimmed">{t('members.authOverridesHint')}</Text>
            {AUTH_METHODS.map(({ key, labelKey }) => (
              <Stack key={key} gap={4}>
                <Text size="sm" fw={500}>{t(labelKey as Parameters<typeof t>[0])}</Text>
                <SegmentedControl
                  size="xs"
                  value={getAuthValue(key)}
                  onChange={(v) => setAuthValue(key, v)}
                  data={[
                    { value: 'inherit', label: t('members.authInherit') },
                    { value: 'on', label: 'On' },
                    { value: 'off', label: 'Off' },
                  ]}
                />
              </Stack>
            ))}
          </Stack>
        </Tabs.Panel>

      </Tabs>
    </Drawer>
  )
}

// ---------- Edit member modal ----------

function EditMemberModal({
  wsId,
  member,
  opened,
  onClose,
}: {
  wsId: string
  member: WorkspaceMembership
  opened: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [displayName, setDisplayName] = useState(member.display_name ?? '')
  const [localUsername, setLocalUsername] = useState(member.local_username ?? '')
  const [role, setRole] = useState(member.role)
  const [error, setError] = useState<string | null>(null)

  const update = useMutation({
    mutationFn: () =>
      membersApi.update(wsId, member.id, {
        display_name: displayName || undefined,
        local_username: member.local_username != null ? (localUsername || undefined) : undefined,
        role: role !== member.role ? role : undefined,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['members', wsId] })
      onClose()
    },
    onError: (err: unknown) => {
      setError(extractApiError(err, t('common.error')))
    },
  })

  return (
    <Modal opened={opened} onClose={onClose} title={t('members.editMemberInfo')}>
      <form onSubmit={(e) => { e.preventDefault(); setError(null); update.mutate() }}>
        <Stack>
          <TextInput
            label={t('members.displayName')}
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder={t('members.displayNamePlaceholder')}
          />
          {member.local_username != null && (
            <TextInput
              label={t('members.username')}
              value={localUsername}
              onChange={(e) => setLocalUsername(e.target.value)}
              placeholder={t('members.usernamePlaceholder')}
            />
          )}
          {member.role !== 'OWNER' && (
            <Select
              label={t('common.role')}
              value={role}
              onChange={(v) => v && setRole(v as WorkspaceMembership['role'])}
              data={[
                { value: 'MEMBER', label: 'Member' },
                { value: 'ADMIN', label: 'Admin' },
              ]}
            />
          )}
          {error && <Alert icon={<AlertCircle size={16} />} color="red" variant="light">{error}</Alert>}
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose}>{t('common.cancel')}</Button>
            <Button type="submit" loading={update.isPending}>{t('common.save')}</Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}

// ---------- Main Page ----------

export default function MembersPage() {
  const { wsId } = useParams<{ wsId: string }>()
  const qc = useQueryClient()
  const { t } = useTranslation()

  const [addOpened, { open: openAdd, close: closeAdd }] = useDisclosure(false)
  const [activeTab, setActiveTab] = useState<string | null>('invite')
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('MEMBER')
  const [addError, setAddError] = useState<string | null>(null)

  const [drawerMember, setDrawerMember] = useState<WorkspaceMembership | null>(null)
  const [editMember, setEditMember] = useState<WorkspaceMembership | null>(null)
  const [selectedMembers, setSelectedMembers] = useState<Set<string>>(new Set())
  const [bulkLoading, setBulkLoading] = useState(false)
  const [bulkMode, setBulkMode] = useState<'permissions' | 'auth'>('permissions')

  const { data: members, isLoading, isError, error } = useQuery<WorkspaceMembership[]>({
    queryKey: ['members', wsId],
    queryFn: () => membersApi.list(wsId!),
  })

  const { data: gates = [] } = useQuery<Gate[]>({
    queryKey: ['gates', wsId],
    queryFn: () => gatesApi.list(wsId!),
    enabled: !!wsId,
  })

  const invite = useMutation({
    mutationFn: () => membersApi.invite(wsId!, email, role),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['members', wsId] }); resetAndClose(); notifySuccess(t('common.saved')) },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setAddError(msg ?? t('common.error'))
    },
  })

  const createLocal = useMutation({
    mutationFn: () =>
      membersApi.createLocal(wsId!, {
        local_username: username,
        display_name: displayName || undefined,
        password,
        role,
      }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['members', wsId] }); resetAndClose(); notifySuccess(t('common.saved')) },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setAddError(msg ?? t('common.error'))
    },
  })

  const deleteMember = useMutation({
    mutationFn: (memberId: string) => membersApi.delete(wsId!, memberId),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['members', wsId] }); notifySuccess(t('common.saved')) },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })


  function resetAndClose() {
    closeAdd()
    setEmail(''); setUsername(''); setDisplayName(''); setPassword('')
    setRole('MEMBER'); setAddError(null)
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setAddError(null)
    if (activeTab === 'invite') invite.mutate()
    else createLocal.mutate()
  }

  function toggleSelected(memberId: string) {
    setSelectedMembers((prev) => {
      const next = new Set(prev)
      if (next.has(memberId)) next.delete(memberId)
      else next.add(memberId)
      return next
    })
  }

  async function bulkTogglePermission(permCode: PermCode, grant: boolean) {
    if (selectedMembers.size === 0 || !wsId) return
    setBulkLoading(true)
    try {
      await Promise.all(
        Array.from(selectedMembers).flatMap((memberId) =>
          gates.map((gate) =>
            grant
              ? policiesApi.grant(wsId, gate.id, memberId, permCode).catch(() => {})
              : policiesApi.revoke(wsId, gate.id, memberId, permCode).catch(() => {})
          )
        )
      )
      Array.from(selectedMembers).forEach((memberId) =>
        qc.invalidateQueries({ queryKey: ['member-policies', wsId, memberId] })
      )
      notifySuccess(t('common.saved'))
    } catch (err) {
      notifyError(err, t('common.error'))
    } finally {
      setBulkLoading(false)
    }
  }

  async function bulkSetAuth(key: string, value: boolean | null) {
    if (selectedMembers.size === 0 || !wsId) return
    setBulkLoading(true)
    try {
      await Promise.all(Array.from(selectedMembers).map((memberId) =>
        membersApi.update(wsId, memberId, { auth_config: { [key]: value } })
      ))
      qc.invalidateQueries({ queryKey: ['members', wsId] })
      notifySuccess(t('common.saved'))
    } catch (err) {
      notifyError(err, t('common.error'))
    } finally {
      setBulkLoading(false)
    }
  }

  const isMobile = useMediaQuery('(max-width: 768px)') ?? false
  const isPending = invite.isPending || createLocal.isPending
  const selectableMembers = members?.filter((m) => m.role === 'MEMBER') ?? []

  return (
    <Container size="sm" py="xl" style={{ paddingBottom: selectedMembers.size > 0 ? 96 : undefined }}>
      <Group justify="space-between" mb="xl">
        <div>
          <Title order={2}>{t('members.title')}</Title>
          <Text c="dimmed" size="sm">{t('members.subtitle')}</Text>
        </div>
        <Button leftSection={<UserPlus size={16} />} onClick={openAdd}>{t('members.add')}</Button>
      </Group>

      {/* Add member modal */}
      <Modal opened={addOpened} onClose={resetAndClose} title={t('members.add')}>
        <Tabs value={activeTab} onChange={setActiveTab}>
          <Tabs.List mb="md">
            <Tabs.Tab value="invite">{t('members.inviteByEmail')}</Tabs.Tab>
            <Tabs.Tab value="create">{t('members.createLocal')}</Tabs.Tab>
          </Tabs.List>
        </Tabs>
        <form onSubmit={handleSubmit}>
          <Stack>
            {activeTab === 'invite' ? (
              <TextInput label={t('auth.email')} type="email" value={email} onChange={(e) => setEmail(e.target.value)} required placeholder="user@example.com" />
            ) : (
              <>
                <TextInput label={t('members.username')} value={username} onChange={(e) => setUsername(e.target.value)} required placeholder={t('members.usernamePlaceholder')} />
                <TextInput label={t('members.displayName')} value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder={t('members.displayNamePlaceholder')} />
                <PasswordInput label={t('auth.password')} value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} />
              </>
            )}
            <Select label={t('common.role')} value={role} onChange={(v) => setRole(v ?? 'MEMBER')}
              data={[{ value: 'MEMBER', label: 'Member' }, { value: 'ADMIN', label: 'Admin' }]}
            />
            {addError && <Alert icon={<AlertCircle size={16} />} color="red" variant="light">{addError}</Alert>}
            <Group justify="flex-end">
              <Button variant="default" onClick={resetAndClose}>{t('common.cancel')}</Button>
              <Button type="submit" loading={isPending}>{t('common.add')}</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Member list */}
      {isLoading ? (
        <Stack>{[0, 1, 2].map((i) => <Skeleton key={i} height={60} radius="md" />)}</Stack>
      ) : isError ? (
        <QueryError error={error} />
      ) : members?.length === 0 ? (
        <Center py={80}>
          <Stack align="center" gap="xs">
            <Users size={36} opacity={0.3} />
            <Text size="sm" c="dimmed">{t('members.noMembers')}</Text>
          </Stack>
        </Center>
      ) : (
        <Stack gap="xs">
          {/* Select all header */}
          {selectableMembers.length > 0 && (
            <Group gap="xs" px={4} mb={2}>
              <Checkbox
                size="xs"
                checked={selectedMembers.size === selectableMembers.length && selectableMembers.length > 0}
                indeterminate={selectedMembers.size > 0 && selectedMembers.size < selectableMembers.length}
                onChange={() => {
                  if (selectedMembers.size === selectableMembers.length) setSelectedMembers(new Set())
                  else setSelectedMembers(new Set(selectableMembers.map((m) => m.id)))
                }}
              />
              <Text size="xs" c="dimmed">
                {selectedMembers.size > 0
                  ? `${selectedMembers.size} / ${selectableMembers.length} selected`
                  : `${selectableMembers.length} member${selectableMembers.length !== 1 ? 's' : ''}`}
              </Text>
            </Group>
          )}

          {members?.map((m) => {
            const isSelectable = m.role === 'MEMBER'
            const isSelected = selectedMembers.has(m.id)
            const memberName = m.display_name ?? m.local_username ?? m.user_email ?? m.id.slice(0, 8)

            return (
              <Paper key={m.id} withBorder radius="md" p="sm">
                <Group wrap="nowrap" gap="xs" align="center">
                  {isSelectable && (
                    <Checkbox size="xs" checked={isSelected} onChange={() => toggleSelected(m.id)} style={{ flexShrink: 0 }} />
                  )}
                  <Avatar color="indigo" radius="xl" size={32} style={{ flexShrink: 0 }}>{memberName[0].toUpperCase()}</Avatar>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <Text size="sm" fw={500} truncate>{memberName}</Text>
                    {m.local_username && m.display_name && (
                      <Text size="xs" c="dimmed" ff="mono" truncate>{m.local_username}</Text>
                    )}
                  </div>
                  <Badge visibleFrom="xs" color={ROLE_COLOR[m.role]} variant="light" size="sm" style={{ flexShrink: 0 }}>{m.role}</Badge>
                  <Badge hiddenFrom="xs" color={ROLE_COLOR[m.role]} variant="light" size="sm" style={{ flexShrink: 0 }}>{m.role[0]}</Badge>
                  <Group gap={4} wrap="nowrap" style={{ flexShrink: 0 }}>
                    <ActionIcon
                      variant="subtle"
                      size="sm"
                      title={t('members.editMemberInfo')}
                      onClick={() => setEditMember(m)}
                    >
                      <Pencil size={14} />
                    </ActionIcon>
                    {m.role === 'MEMBER' && (
                      <ActionIcon
                        variant="subtle"
                        size="sm"
                        title={t('members.editMember')}
                        onClick={() => setDrawerMember(m)}
                      >
                        <Settings2 size={14} />
                      </ActionIcon>
                    )}
                    {m.role !== 'OWNER' && (
                      <ActionIcon variant="subtle" color="red" size="sm" onClick={() => deleteMember.mutate(m.id)}>
                        <Trash2 size={14} />
                      </ActionIcon>
                    )}
                  </Group>
                </Group>
              </Paper>
            )
          })}
        </Stack>
      )}

      {/* Edit member modal */}
      {editMember && (
        <EditMemberModal
          wsId={wsId!}
          member={editMember}
          opened={!!editMember}
          onClose={() => setEditMember(null)}
        />
      )}

      {/* Per-member settings drawer */}
      {drawerMember && (
        <MemberSettingsDrawer
          wsId={wsId!}
          member={members?.find((m) => m.id === drawerMember.id) ?? drawerMember}
          gates={gates}
          opened={!!drawerMember}
          onClose={() => setDrawerMember(null)}
        />
      )}

      {/* Bulk action bar */}
      {selectedMembers.size > 0 && (
        <Paper
          withBorder
          shadow="md"
          p="sm"
          style={isMobile ? {
            position: 'fixed',
            bottom: 16,
            left: 16,
            right: 16,
            zIndex: 100,
          } : {
            position: 'fixed',
            bottom: 16,
            left: 'calc(50% + 140px)',
            transform: 'translateX(-50%)',
            zIndex: 100,
            width: 'calc(100vw - 280px - 32px)',
            maxWidth: 680,
          }}
        >
          <Stack gap="xs">
            <Group justify="space-between" align="center" wrap="nowrap">
              <Text size="sm" fw={600}>{selectedMembers.size} / {selectableMembers.length}</Text>
              <ActionIcon variant="subtle" color="gray" size="sm" onClick={() => setSelectedMembers(new Set())}>
                <X size={14} />
              </ActionIcon>
            </Group>
            <SegmentedControl
              size="xs"
              fullWidth
              value={bulkMode}
              onChange={(v) => setBulkMode(v as 'permissions' | 'auth')}
              data={[
                { value: 'permissions', label: t('members.expandPermissions') },
                { value: 'auth', label: t('members.bulkAuth') },
              ]}
            />

            {bulkMode === 'permissions' ? (
              <Stack gap={4}>
                {(
                  [
                    { code: 'gate:read_status' as PermCode, labelKey: 'permissions.viewStatus' },
                    { code: 'gate:trigger_open' as PermCode, labelKey: 'permissions.triggerOpen' },
                    { code: 'gate:trigger_close' as PermCode, labelKey: 'permissions.triggerClose' },
                    { code: 'gate:manage' as PermCode, labelKey: 'permissions.manage' },
                  ] as const
                ).map(({ code, labelKey }) => (
                  <Group key={code} justify="space-between" align="center" wrap="nowrap">
                    <Text size="xs" fw={500}>{t(labelKey as Parameters<typeof t>[0])}</Text>
                    <Group gap={4} wrap="nowrap">
                      <Button size="compact-xs" variant="light" color="green" loading={bulkLoading} onClick={() => bulkTogglePermission(code, true)}>{t('common.grant')}</Button>
                      <Button size="compact-xs" variant="light" color="red" loading={bulkLoading} onClick={() => bulkTogglePermission(code, false)}>{t('common.revoke')}</Button>
                    </Group>
                  </Group>
                ))}
              </Stack>
            ) : (
              <Stack gap={6}>
                {AUTH_METHODS.map(({ key, labelKey }) => (
                  <Group key={key} justify="space-between" align="center">
                    <Text size="xs" fw={500}>{t(labelKey as Parameters<typeof t>[0])}</Text>
                    <Group gap={4}>
                      <Button size="compact-xs" variant="light" color="green" loading={bulkLoading} onClick={() => bulkSetAuth(key, true)}>On</Button>
                      <Button size="compact-xs" variant="light" color="red" loading={bulkLoading} onClick={() => bulkSetAuth(key, false)}>Off</Button>
                      <Button size="compact-xs" variant="subtle" color="gray" loading={bulkLoading} onClick={() => bulkSetAuth(key, null)}>{t('members.authInherit')}</Button>
                    </Group>
                  </Group>
                ))}
              </Stack>
            )}
          </Stack>
        </Paper>
      )}
    </Container>
  )
}
