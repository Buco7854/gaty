import { useState } from 'react'
import { useParams } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { membersApi, gatesApi, policiesApi } from '@/api'
import type { WorkspaceMembership, Gate, MembershipPolicy } from '@/types'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Group, Button, Modal, Stack, Alert, Tabs,
  TextInput, PasswordInput, Select, Badge, Avatar, ActionIcon, Center, Skeleton,
  Collapse, Anchor, NumberInput, Table, Checkbox, Paper, SegmentedControl, Drawer,
} from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { UserPlus, Trash2, Users, AlertCircle, Settings2, X } from 'lucide-react'

const ROLE_COLOR: Record<string, string> = {
  OWNER: 'yellow',
  ADMIN: 'blue',
  MEMBER: 'gray',
}

const SESSION_PRESET_OPTIONS = [
  { value: '', labelKey: 'members.session7d' },
  { value: '0', labelKey: 'members.sessionInfinite' },
  { value: '3600', labelKey: 'members.session1h' },
  { value: '28800', labelKey: 'members.session8h' },
  { value: '86400', labelKey: 'members.session24h' },
  { value: '2592000', labelKey: 'members.session30d' },
  { value: 'custom', labelKey: 'members.sessionCustom' },
] as const

const UNIT_MULTIPLIERS: Record<string, number> = {
  minutes: 60,
  hours: 3600,
  days: 86400,
}

const PERMISSIONS = [
  { code: 'gate:read_status', labelKey: 'permissions.viewStatus' },
  { code: 'gate:trigger_open', labelKey: 'permissions.triggerOpen' },
  { code: 'gate:trigger_close', labelKey: 'permissions.triggerClose' },
  { code: 'gate:manage', labelKey: 'permissions.manage' },
] as const

type PermCode = typeof PERMISSIONS[number]['code']

const PRESETS: Record<string, PermCode[]> = {
  none: [],
  read: ['gate:read_status'],
  operator: ['gate:read_status', 'gate:trigger_open', 'gate:trigger_close'],
  full: ['gate:read_status', 'gate:trigger_open', 'gate:trigger_close', 'gate:manage'],
}

const AUTH_METHODS = [
  { key: 'password', labelKey: 'settings.passwordAuth' },
  { key: 'sso', labelKey: 'settings.ssoAuth' },
  { key: 'api_token', labelKey: 'settings.apiTokenAuth' },
] as const

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
  const qc = useQueryClient()

  const authConfig: Record<string, unknown> = (member.auth_config ?? {}) as Record<string, unknown>

  const updateAuth = useMutation({
    mutationFn: (cfg: Record<string, unknown>) =>
      membersApi.update(wsId, member.id, { auth_config: cfg }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['members', wsId] }),
  })

  function getAuthValue(key: string): string {
    const val = authConfig[key]
    if (val === true) return 'on'
    if (val === false) return 'off'
    return 'inherit'
  }

  function setAuthValue(key: string, value: string) {
    const mapped: boolean | null = value === 'on' ? true : value === 'off' ? false : null
    updateAuth.mutate({ ...authConfig, [key]: mapped })
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
    if (on) await policiesApi.revoke(wsId, gateId, member.id, permCode)
    else await policiesApi.grant(wsId, gateId, member.id, permCode)
    invalidatePolicies()
  }

  async function applyPreset(presetKey: keyof typeof PRESETS) {
    const target = PRESETS[presetKey]
    await Promise.all(
      gates.map(async (gate) => {
        await Promise.all(PERMISSIONS.filter(({ code }) => hasPermission(gate.id, code)).map(({ code }) => policiesApi.revoke(wsId, gate.id, member.id, code)))
        await Promise.all(target.map((code) => policiesApi.grant(wsId, gate.id, member.id, code)))
      })
    )
    invalidatePolicies()
  }

  const memberName = member.display_name ?? member.local_username ?? member.id.slice(0, 8)

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
          <Tabs.Tab value="auth">{t('members.authOverrides')}</Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="permissions" p="md">
          {gates.length === 0 ? (
            <Text size="sm" c="dimmed">{t('gates.noGates')}</Text>
          ) : (
            <Stack gap="md">
              <Group gap="xs">
                <Text size="xs" c="dimmed" fw={500} mr={2}>Preset:</Text>
                <Button size="compact-xs" variant="light" color="gray" onClick={() => applyPreset('none')}>{t('members.bulkNone')}</Button>
                <Button size="compact-xs" variant="light" onClick={() => applyPreset('read')}>{t('members.bulkRead')}</Button>
                <Button size="compact-xs" variant="light" color="teal" onClick={() => applyPreset('operator')}>{t('members.bulkOperator')}</Button>
                <Button size="compact-xs" variant="light" color="indigo" onClick={() => applyPreset('full')}>{t('members.bulkFull')}</Button>
              </Group>
              <Table withColumnBorders withRowBorders={false} horizontalSpacing="xs" verticalSpacing={4} fz="xs">
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th style={{ minWidth: 90 }}>{t('common.name')}</Table.Th>
                    {PERMISSIONS.map(({ code, labelKey }) => (
                      <Table.Th key={code} ta="center" style={{ width: 62 }}>
                        {t(labelKey as Parameters<typeof t>[0])}
                      </Table.Th>
                    ))}
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {gates.map((gate) => (
                    <Table.Tr key={gate.id}>
                      <Table.Td><Text size="xs" truncate maw={110}>{gate.name}</Text></Table.Td>
                      {PERMISSIONS.map(({ code }) => (
                        <Table.Td key={code} ta="center">
                          <Checkbox
                            size="xs"
                            checked={hasPermission(gate.id, code)}
                            onChange={() => toggle(gate.id, code, hasPermission(gate.id, code))}
                          />
                        </Table.Td>
                      ))}
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Stack>
          )}
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

// ---------- Main Page ----------

export default function MembersPage() {
  const { wsId } = useParams<{ wsId: string }>()
  const qc = useQueryClient()
  const { t } = useTranslation()

  const [addOpened, { open: openAdd, close: closeAdd }] = useDisclosure(false)
  const [advancedOpened, setAdvancedOpened] = useState(false)
  const [activeTab, setActiveTab] = useState<string | null>('invite')
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('MEMBER')
  const [sessionDuration, setSessionDuration] = useState<string>('')
  const [customValue, setCustomValue] = useState<number | string>(1)
  const [customUnit, setCustomUnit] = useState<string>('days')
  const [addError, setAddError] = useState<string | null>(null)

  const [drawerMember, setDrawerMember] = useState<WorkspaceMembership | null>(null)
  const [selectedMembers, setSelectedMembers] = useState<Set<string>>(new Set())
  const [bulkLoading, setBulkLoading] = useState(false)
  const [bulkMode, setBulkMode] = useState<'permissions' | 'auth'>('permissions')

  const { data: members, isLoading } = useQuery<WorkspaceMembership[]>({
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
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['members', wsId] }); resetAndClose() },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setAddError(msg ?? t('common.error'))
    },
  })

  function resolveSessionDurationSeconds(): number | undefined {
    if (sessionDuration === '') return undefined
    if (sessionDuration === '0') return 0
    if (sessionDuration === 'custom') {
      const n = typeof customValue === 'number' ? customValue : parseFloat(String(customValue))
      if (!n || n <= 0) return undefined
      return Math.round(n * (UNIT_MULTIPLIERS[customUnit] ?? 3600))
    }
    return parseInt(sessionDuration, 10)
  }

  const createLocal = useMutation({
    mutationFn: () => {
      const dur = resolveSessionDurationSeconds()
      return membersApi.createLocal(wsId!, {
        local_username: username,
        display_name: displayName || undefined,
        password,
        role,
        auth_config: dur !== undefined ? { session_duration: dur } : undefined,
      })
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['members', wsId] }); resetAndClose() },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setAddError(msg ?? t('common.error'))
    },
  })

  const deleteMember = useMutation({
    mutationFn: (memberId: string) => membersApi.delete(wsId!, memberId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['members', wsId] }),
  })

  const updateRole = useMutation({
    mutationFn: ({ memberId, newRole }: { memberId: string; newRole: string }) =>
      membersApi.update(wsId!, memberId, { role: newRole }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['members', wsId] }),
  })

  function resetAndClose() {
    closeAdd()
    setEmail(''); setUsername(''); setDisplayName(''); setPassword('')
    setRole('MEMBER'); setSessionDuration(''); setCustomValue(1); setCustomUnit('days')
    setAdvancedOpened(false); setAddError(null)
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

  async function bulkApplyPreset(presetKey: keyof typeof PRESETS) {
    if (selectedMembers.size === 0 || !wsId) return
    setBulkLoading(true)
    const target = PRESETS[presetKey]
    try {
      await Promise.all(
        Array.from(selectedMembers).map(async (memberId) => {
          await Promise.all(
            gates.map(async (gate) => {
              await Promise.all(PERMISSIONS.map(({ code }) => policiesApi.revoke(wsId, gate.id, memberId, code).catch(() => {})))
              await Promise.all(target.map((code) => policiesApi.grant(wsId, gate.id, memberId, code).catch(() => {})))
            })
          )
          qc.invalidateQueries({ queryKey: ['member-policies', wsId, memberId] })
        })
      )
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
    } finally {
      setBulkLoading(false)
    }
  }

  const isPending = invite.isPending || createLocal.isPending
  const sessionPresetOptions = SESSION_PRESET_OPTIONS.map(({ value, labelKey }) => ({ value, label: t(labelKey) }))
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
                <Anchor component="button" type="button" size="xs" c="dimmed" onClick={() => setAdvancedOpened((o) => !o)}>
                  {t('gates.advancedOptions')} {advancedOpened ? '▲' : '▼'}
                </Anchor>
                <Collapse in={advancedOpened}>
                  <Stack gap="xs">
                    <Select label={t('members.sessionDuration')} value={sessionDuration} onChange={(v) => setSessionDuration(v ?? '')} data={sessionPresetOptions} />
                    {sessionDuration === 'custom' && (
                      <Group gap="xs" grow>
                        <NumberInput label={t('members.sessionCustomValue')} value={customValue} onChange={setCustomValue} min={1} step={1} />
                        <Select label={t('members.sessionCustomUnit')} value={customUnit} onChange={(v) => setCustomUnit(v ?? 'days')}
                          data={[
                            { value: 'minutes', label: t('members.sessionUnitMinutes') },
                            { value: 'hours', label: t('members.sessionUnitHours') },
                            { value: 'days', label: t('members.sessionUnitDays') },
                          ]}
                        />
                      </Group>
                    )}
                  </Stack>
                </Collapse>
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
            const memberName = m.display_name ?? m.local_username ?? `User ${m.id.slice(0, 8)}`

            return (
              <Paper key={m.id} withBorder radius="md" p="sm">
                <Group justify="space-between" wrap="nowrap">
                  <Group gap="sm" wrap="nowrap" style={{ flex: 1, minWidth: 0 }}>
                    {isSelectable ? (
                      <Checkbox size="xs" checked={isSelected} onChange={() => toggleSelected(m.id)} />
                    ) : (
                      <div style={{ width: 16 }} />
                    )}
                    <Avatar color="indigo" radius="xl" size={32}>{memberName[0].toUpperCase()}</Avatar>
                    <div style={{ minWidth: 0 }}>
                      <Text size="sm" fw={500} truncate>{memberName}</Text>
                      {m.local_username && m.display_name && (
                        <Text size="xs" c="dimmed" ff="mono" truncate>{m.local_username}</Text>
                      )}
                    </div>
                  </Group>

                  <Group gap="xs" wrap="nowrap">
                    {m.role !== 'OWNER' ? (
                      <Select
                        size="xs"
                        value={m.role}
                        onChange={(v) => v && updateRole.mutate({ memberId: m.id, newRole: v })}
                        data={[
                          { value: 'MEMBER', label: 'Member' },
                          { value: 'ADMIN', label: 'Admin' },
                        ]}
                        styles={{ input: { minWidth: 82 } }}
                        comboboxProps={{ withinPortal: true }}
                      />
                    ) : (
                      <Badge color={ROLE_COLOR['OWNER']} variant="light" size="sm">OWNER</Badge>
                    )}

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

      {/* Per-member settings drawer */}
      {drawerMember && (
        <MemberSettingsDrawer
          wsId={wsId!}
          member={drawerMember}
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
          style={{
            position: 'fixed',
            bottom: 16,
            left: '50%',
            transform: 'translateX(-50%)',
            zIndex: 100,
            width: 'calc(100vw - 32px)',
            maxWidth: 680,
          }}
        >
          <Stack gap="xs">
            <Group justify="space-between" align="center" wrap="nowrap">
              <Group gap="xs" align="center" wrap="nowrap">
                <Text size="sm" fw={600}>{selectedMembers.size} / {selectableMembers.length}</Text>
                <SegmentedControl
                  size="xs"
                  value={bulkMode}
                  onChange={(v) => setBulkMode(v as 'permissions' | 'auth')}
                  data={[
                    { value: 'permissions', label: t('members.gatePermissions') },
                    { value: 'auth', label: t('members.authOverrides') },
                  ]}
                />
              </Group>
              <ActionIcon variant="subtle" color="gray" size="sm" onClick={() => setSelectedMembers(new Set())}>
                <X size={14} />
              </ActionIcon>
            </Group>

            {bulkMode === 'permissions' ? (
              <Group gap="xs" wrap="wrap">
                <Button size="compact-xs" variant="light" color="gray" loading={bulkLoading} onClick={() => bulkApplyPreset('none')}>{t('members.bulkNone')}</Button>
                <Button size="compact-xs" variant="light" loading={bulkLoading} onClick={() => bulkApplyPreset('read')}>{t('members.bulkRead')}</Button>
                <Button size="compact-xs" variant="light" color="teal" loading={bulkLoading} onClick={() => bulkApplyPreset('operator')}>{t('members.bulkOperator')}</Button>
                <Button size="compact-xs" variant="light" color="indigo" loading={bulkLoading} onClick={() => bulkApplyPreset('full')}>{t('members.bulkFull')}</Button>
              </Group>
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
