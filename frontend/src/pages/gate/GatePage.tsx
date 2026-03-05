import { useState, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { gatesApi, pinsApi, domainsApi, policiesApi, schedulesApi } from '@/api'
import type { ActionConfig, PinMetadata } from '@/api'
import type { Gate, GatePin, CustomDomain, WorkspaceWithRole, AccessSchedule } from '@/types'
import { useAuthStore } from '@/store/auth'
import { findLocalSession } from '@/utils/session'
import { useTranslation } from 'react-i18next'
import { notifications } from '@mantine/notifications'
import {
  Container, Title, Text, Group, Button, Stack, Paper, Badge, ActionIcon,
  TextInput, PasswordInput, Select, Tooltip, Modal, Code, Alert,
  NumberInput, Checkbox,
} from '@mantine/core'
import { useDisclosure, useClipboard } from '@mantine/hooks'
import {
  ArrowLeft, Zap, Hash, Globe, Plus, Trash2, CheckCircle2, XCircle,
  Clock, Copy, Check, Settings2, Pencil, Info, CalendarClock,
} from 'lucide-react'

function ActionConfigForm({
  label,
  value,
  onChange,
}: {
  label: string
  value: ActionConfig | null
  onChange: (v: ActionConfig | null) => void
}) {
  const { t } = useTranslation()
  const driverType = value?.type ?? 'NONE'

  return (
    <Stack gap="xs">
      <Select
        label={label}
        value={driverType}
        onChange={(v) => {
          const type = (v ?? 'NONE') as ActionConfig['type']
          if (type === 'NONE') {
            onChange(null)
          } else {
            onChange({ type, config: value?.config })
          }
        }}
        data={[
          { value: 'NONE', label: t('gates.noneDriver') },
          { value: 'MQTT', label: t('gates.mqttDriver') },
          { value: 'HTTP', label: t('gates.httpDriver') },
        ]}
      />
      {driverType === 'HTTP' && (
        <>
          <TextInput
            label={t('gates.httpUrl')}
            value={(value?.config?.url as string) ?? ''}
            onChange={(e) =>
              onChange({ type: 'HTTP', config: { ...value?.config, url: e.target.value } })
            }
            placeholder="https://api.example.com/open"
            required
          />
          <Select
            label={t('gates.httpMethod')}
            value={(value?.config?.method as string) ?? 'POST'}
            onChange={(v) =>
              onChange({ type: 'HTTP', config: { ...value?.config, method: v ?? 'POST' } })
            }
            data={['POST', 'GET', 'PUT', 'PATCH']}
          />
        </>
      )}
    </Stack>
  )
}

export default function GatePage() {
  const { wsId, gateId } = useParams<{ wsId: string; gateId: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { t } = useTranslation()
  const clipboard = useClipboard({ timeout: 2000 })

  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const globalAuth = isAuthenticated()
  const localSession = useMemo(
    () => (!globalAuth && wsId ? findLocalSession(wsId) : null),
    [wsId, globalAuth]
  )
  const ws = qc.getQueryData<WorkspaceWithRole[]>(['workspaces'])?.find((w) => w.id === wsId)
  const effectiveRole = globalAuth ? ws?.role : localSession?.role
  const canManage = effectiveRole === 'ADMIN' || effectiveRole === 'OWNER'

  const { data: myPolicies } = useQuery({
    queryKey: ['policies-me', wsId],
    queryFn: () => policiesApi.listMine(wsId!),
    enabled: !canManage && (globalAuth || !!localSession),
  })
  const canManageGate = canManage ||
    myPolicies?.some((p) => p.gate_id === gateId && p.permission_code === 'gate:manage')

  // Modal state
  const [pinModalOpened, { open: openPinModal, close: closePinModal }] = useDisclosure(false)
  const [domainModalOpened, { open: openDomainModal, close: closeDomainModal }] = useDisclosure(false)
  const [configModalOpened, { open: openConfigModal, close: closeConfigModal }] = useDisclosure(false)

  // PIN form
  const [pinLabel, setPinLabel] = useState('')
  const [pinValue, setPinValue] = useState('')
  const [pinExpiresAt, setPinExpiresAt] = useState('')
  const [pinSessionDuration, setPinSessionDuration] = useState<string>('')
  const [pinCustomValue, setPinCustomValue] = useState<number | string>(1)
  const [pinCustomUnit, setPinCustomUnit] = useState<string>('days')
  const [pinMaxUses, setPinMaxUses] = useState<number | string>('')
  const [pinPermissions, setPinPermissions] = useState<string[]>(['gate:trigger_open'])
  const [pinCodeType, setPinCodeType] = useState<'pin' | 'password'>('pin')
  const [pinModalMode, setPinModalMode] = useState<'create' | 'edit'>('create')
  const [editingPinId, setEditingPinId] = useState<string | null>(null)
  const [pinScheduleId, setPinScheduleId] = useState<string>('')

  // Domain form
  const [domainValue, setDomainValue] = useState('')
  const [verifyResult, setVerifyResult] = useState<Record<string, { verified: boolean; message?: string }>>({})

  // Config form
  const [editOpenConfig, setEditOpenConfig] = useState<ActionConfig | null>(null)
  const [editCloseConfig, setEditCloseConfig] = useState<ActionConfig | null>(null)
  const [editStatusConfig, setEditStatusConfig] = useState<ActionConfig | null>(null)

  const PIN_SESSION_PRESETS = [
    { value: '', label: t('members.session7d') },
    { value: '0', label: t('members.sessionInfinite') },
    { value: '3600', label: t('members.session1h') },
    { value: '28800', label: t('members.session8h') },
    { value: '86400', label: t('members.session24h') },
    { value: '2592000', label: t('members.session30d') },
    { value: 'custom', label: t('members.sessionCustom') },
  ]

  function resolvePinSessionDurationSeconds(): number | undefined {
    if (pinSessionDuration === '') return undefined
    if (pinSessionDuration === '0') return 0
    if (pinSessionDuration === 'custom') {
      const n = typeof pinCustomValue === 'number' ? pinCustomValue : parseFloat(String(pinCustomValue))
      if (!n || n <= 0) return undefined
      const multipliers: Record<string, number> = { minutes: 60, hours: 3600, days: 86400 }
      return Math.round(n * (multipliers[pinCustomUnit] ?? 3600))
    }
    return parseInt(pinSessionDuration, 10)
  }

  const { data: gate } = useQuery<Gate>({
    queryKey: ['gate', wsId, gateId],
    queryFn: () => gatesApi.get(wsId!, gateId!),
    refetchInterval: 10_000,
  })

  const { data: pins } = useQuery<GatePin[]>({
    queryKey: ['pins', wsId, gateId],
    queryFn: () => pinsApi.list(wsId!, gateId!),
  })

  const { data: domains } = useQuery<CustomDomain[]>({
    queryKey: ['domains', wsId, gateId],
    queryFn: () => domainsApi.list(wsId!, gateId!),
    enabled: canManageGate,
  })

  const { data: schedules = [] } = useQuery<AccessSchedule[]>({
    queryKey: ['schedules', wsId],
    queryFn: () => schedulesApi.list(wsId!),
    enabled: canManageGate,
  })

  const trigger = useMutation({
    mutationFn: () => gatesApi.trigger(wsId!, gateId!),
    onSuccess: () => notifications.show({ color: 'green', message: t('pinpad.gateOpened'), autoClose: 3000 }),
    onError: () => notifications.show({ color: 'red', message: t('pinpad.unreachable'), autoClose: 4000 }),
  })

  const updateConfig = useMutation({
    mutationFn: () =>
      gatesApi.update(wsId!, gateId!, {
        open_config: editOpenConfig,
        close_config: editCloseConfig,
        status_config: editStatusConfig,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['gate', wsId, gateId] })
      closeConfigModal()
    },
  })

  const createPin = useMutation({
    mutationFn: async () => {
      const metadata: PinMetadata = { permissions: pinPermissions }
      if (pinExpiresAt) metadata.expires_at = new Date(pinExpiresAt).toISOString()
      const dur = resolvePinSessionDurationSeconds()
      if (dur !== undefined) metadata.session_duration = dur
      const maxUses = typeof pinMaxUses === 'number' ? pinMaxUses : parseInt(String(pinMaxUses), 10)
      if (maxUses > 0) metadata.max_uses = maxUses
      return pinsApi.create(wsId!, gateId!, {
        label: pinLabel,
        pin: pinValue,
        code_type: pinCodeType,
        schedule_id: pinScheduleId || undefined,
        metadata,
      })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pins', wsId, gateId] })
      closePinModal()
      resetPinForm()
    },
  })

  function resetPinForm() {
    setPinLabel(''); setPinValue(''); setPinExpiresAt(''); setPinSessionDuration('')
    setPinCustomValue(1); setPinCustomUnit('days'); setPinMaxUses(''); setPinPermissions(['gate:trigger_open'])
    setPinCodeType('pin'); setPinModalMode('create'); setEditingPinId(null); setPinScheduleId('')
  }

  function openEditModal(pin: GatePin) {
    const meta = pin.metadata as {
      code_type?: 'pin' | 'password'; expires_at?: string
      permissions?: string[]; session_duration?: number; max_uses?: number
    }
    setPinModalMode('edit')
    setEditingPinId(pin.id)
    setPinLabel(pin.label)
    setPinCodeType(meta.code_type ?? 'pin')
    setPinExpiresAt(meta.expires_at ? new Date(meta.expires_at).toISOString().slice(0, 16) : '')
    setPinPermissions(meta.permissions ?? ['gate:trigger_open'])
    const sd = meta.session_duration
    setPinSessionDuration(sd === undefined ? '' : sd === 0 ? '0' : String(sd))
    setPinMaxUses(meta.max_uses ?? '')
    setPinScheduleId(pin.schedule_id ?? '')
    openPinModal()
  }

  const updatePin = useMutation({
    mutationFn: async () => {
      const metadata: PinMetadata = { permissions: pinPermissions, code_type: pinCodeType }
      metadata.expires_at = pinExpiresAt ? new Date(pinExpiresAt).toISOString() : null
      const dur = resolvePinSessionDurationSeconds()
      metadata.session_duration = dur !== undefined ? dur : null
      const maxUses = typeof pinMaxUses === 'number' ? pinMaxUses : parseInt(String(pinMaxUses), 10)
      metadata.max_uses = maxUses > 0 ? maxUses : null
      await pinsApi.update(wsId!, gateId!, editingPinId!, {
        label: pinLabel,
        metadata,
      })
      // Schedule is managed separately via dedicated endpoints
      if (pinScheduleId) {
        await pinsApi.setSchedule(wsId!, gateId!, editingPinId!, pinScheduleId)
      } else {
        await pinsApi.clearSchedule(wsId!, gateId!, editingPinId!).catch(() => {})
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pins', wsId, gateId] })
      closePinModal()
      resetPinForm()
    },
  })

  const deletePin = useMutation({
    mutationFn: (pinId: string) => pinsApi.delete(wsId!, gateId!, pinId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pins', wsId, gateId] }),
  })

  const addDomain = useMutation({
    mutationFn: () => domainsApi.create(wsId!, gateId!, domainValue),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['domains', wsId, gateId] })
      closeDomainModal()
      setDomainValue('')
    },
  })

  const deleteDomain = useMutation({
    mutationFn: (domainId: string) => domainsApi.delete(wsId!, gateId!, domainId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['domains', wsId, gateId] }),
  })

  const verifyDomain = useMutation({
    mutationFn: (domainId: string) => domainsApi.verify(wsId!, gateId!, domainId),
    onSuccess: (data, domainId) => {
      setVerifyResult((prev) => ({ ...prev, [domainId]: data }))
      if (data.verified) qc.invalidateQueries({ queryKey: ['domains', wsId, gateId] })
    },
  })

  function openConfig() {
    setEditOpenConfig(gate?.open_config ?? null)
    setEditCloseConfig(gate?.close_config ?? null)
    setEditStatusConfig(gate?.status_config ?? null)
    openConfigModal()
  }

  const statusColor = gate?.status === 'online' ? 'green' : gate?.status === 'offline' ? 'red' : 'gray'

  const scheduleSelectData = [
    { value: '', label: t('common.none') },
    ...schedules.map((s) => ({ value: s.id, label: s.name })),
  ]

  const scheduleById = useMemo(() => {
    const map: Record<string, AccessSchedule> = {}
    for (const s of schedules) map[s.id] = s
    return map
  }, [schedules])

  return (
    <Container size="sm" py="xl">
      {/* Header */}
      <Button
        variant="subtle"
        color="gray"
        size="xs"
        leftSection={<ArrowLeft size={14} />}
        mb="md"
        onClick={() => navigate(`/workspaces/${wsId}`)}
      >
        {t('common.back')}
      </Button>

      <Group justify="space-between" mb="xl">
        <div>
          <Group gap="sm">
            <Title order={2}>{gate?.name ?? '…'}</Title>
            {gate && (
              <Badge color={statusColor} variant="light">
                {t(`common.${gate.status}`)}
              </Badge>
            )}
          </Group>
        </div>
        <Group>
          {canManageGate && (
            <Tooltip label={t('gates.integration')}>
              <ActionIcon variant="default" size="lg" onClick={openConfig}>
                <Settings2 size={16} />
              </ActionIcon>
            </Tooltip>
          )}
          <Button
            leftSection={<Zap size={16} />}
            loading={trigger.isPending}
            onClick={() => trigger.mutate()}
          >
            {t('gates.openGate')}
          </Button>
        </Group>
      </Group>

      {/* Integration config modal */}
      <Modal opened={configModalOpened} onClose={closeConfigModal} title={t('gates.integration')} size="md">
        <form onSubmit={(e) => { e.preventDefault(); updateConfig.mutate() }}>
          <Stack>
            <ActionConfigForm
              label={t('gates.openAction')}
              value={editOpenConfig}
              onChange={setEditOpenConfig}
            />
            <ActionConfigForm
              label={t('gates.closeAction')}
              value={editCloseConfig}
              onChange={setEditCloseConfig}
            />
            <ActionConfigForm
              label={t('gates.statusAction')}
              value={editStatusConfig}
              onChange={setEditStatusConfig}
            />
            <Group justify="flex-end">
              <Button variant="default" onClick={closeConfigModal}>{t('common.cancel')}</Button>
              <Button type="submit" loading={updateConfig.isPending}>{t('common.save')}</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {/* Access codes */}
      <Paper withBorder p="md" radius="md" mb="md">
        <Group justify="space-between" mb="sm">
          <Group gap="xs">
            <Hash size={16} opacity={0.6} />
            <Text fw={600}>{t('pins.title')}</Text>
            <Badge variant="light" size="xs">{pins?.length ?? 0}</Badge>
          </Group>
          {canManageGate && (
            <Button size="xs" variant="subtle" leftSection={<Plus size={14} />} onClick={() => { resetPinForm(); openPinModal() }}>
              {t('pins.add')}
            </Button>
          )}
        </Group>
        {(pins?.length ?? 0) === 0 ? (
          <Text size="sm" c="dimmed">{t('pins.noPins')}</Text>
        ) : (
          <Stack gap={2}>
            {pins?.map((pin) => {
              const codeType = (pin.metadata.code_type as 'pin' | 'password') ?? 'pin'
              const schedule = pin.schedule_id ? scheduleById[pin.schedule_id] : null
              return (
                <Group key={pin.id} justify="space-between" py={4}>
                  <Group gap="sm">
                    <Hash size={14} opacity={0.5} />
                    <Text size="sm">{pin.label}</Text>
                    <Badge size="xs" variant="dot" color={codeType === 'pin' ? 'indigo' : 'violet'}>
                      {codeType === 'pin' ? 'PIN' : t('pins.passwords')}
                    </Badge>
                    {(pin.metadata as { expires_at?: string }).expires_at && (
                      <Group gap={4}>
                        <Clock size={12} opacity={0.5} />
                        <Text size="xs" c="dimmed">
                          {new Date((pin.metadata as { expires_at: string }).expires_at).toLocaleDateString()}
                        </Text>
                      </Group>
                    )}
                    {schedule && (
                      <Tooltip label={schedule.name}>
                        <Badge size="xs" variant="light" color="orange" leftSection={<CalendarClock size={10} />}>
                          {schedule.name}
                        </Badge>
                      </Tooltip>
                    )}
                  </Group>
                  {canManageGate && (
                    <Group gap={4}>
                      <ActionIcon variant="subtle" size="sm" onClick={() => openEditModal(pin)}>
                        <Pencil size={14} />
                      </ActionIcon>
                      <ActionIcon variant="subtle" color="red" size="sm" onClick={() => deletePin.mutate(pin.id)}>
                        <Trash2 size={14} />
                      </ActionIcon>
                    </Group>
                  )}
                </Group>
              )
            })}
          </Stack>
        )}
      </Paper>

      {/* Access code create/edit modal */}
      <Modal
        opened={pinModalOpened}
        onClose={() => { closePinModal(); resetPinForm() }}
        title={pinModalMode === 'edit' ? t('pins.editCode') : t('pins.add')}
      >
          <form onSubmit={(e) => { e.preventDefault(); pinModalMode === 'edit' ? updatePin.mutate() : createPin.mutate() }}>
            <Stack>
              <TextInput
                label={t('pins.label')}
                value={pinLabel}
                onChange={(e) => setPinLabel(e.target.value)}
                placeholder={t('pins.labelPlaceholder')}
                required
              />
              <Select
                label={t('pins.codeType')}
                value={pinCodeType}
                onChange={(v) => { setPinCodeType((v as 'pin' | 'password') ?? 'pin'); setPinValue('') }}
                data={[
                  { value: 'pin', label: t('pins.codeTypePin') },
                  { value: 'password', label: t('pins.codeTypePassword') },
                ]}
              />
              <Alert color="blue" variant="light" icon={<Info size={14} />} p="xs">
                <Text size="xs">{pinCodeType === 'pin' ? t('pins.methodWarningPin') : t('pins.methodWarningPassword')}</Text>
              </Alert>
              {pinModalMode === 'create' && (
                <PasswordInput
                  label={t('pins.code')}
                  value={pinValue}
                  onChange={(e) => {
                    const v = e.target.value
                    setPinValue(pinCodeType === 'pin' ? v.replace(/\D/g, '') : v)
                  }}
                  required
                  minLength={1}
                  inputMode={pinCodeType === 'pin' ? 'numeric' : undefined}
                  styles={pinCodeType === 'pin' ? { input: { fontFamily: 'monospace', letterSpacing: '0.2em' } } : undefined}
                />
              )}
              <Stack gap="xs">
                <Select
                  label={t('pins.sessionDuration')}
                  description={t('pins.sessionDurationDesc')}
                  value={pinSessionDuration}
                  onChange={(v) => setPinSessionDuration(v ?? '')}
                  data={PIN_SESSION_PRESETS}
                />
                {pinSessionDuration === 'custom' && (
                  <Group gap="xs" grow>
                    <NumberInput
                      label={t('members.sessionCustomValue')}
                      value={pinCustomValue}
                      onChange={setPinCustomValue}
                      min={1}
                      step={1}
                    />
                    <Select
                      label={t('members.sessionCustomUnit')}
                      value={pinCustomUnit}
                      onChange={(v) => setPinCustomUnit(v ?? 'days')}
                      data={[
                        { value: 'minutes', label: t('members.sessionUnitMinutes') },
                        { value: 'hours', label: t('members.sessionUnitHours') },
                        { value: 'days', label: t('members.sessionUnitDays') },
                      ]}
                    />
                  </Group>
                )}
              </Stack>
              <NumberInput
                label={t('pins.maxUses')}
                description={t('pins.maxUsesDesc')}
                value={pinMaxUses}
                onChange={setPinMaxUses}
                min={1}
                step={1}
                allowDecimal={false}
              />
              <Checkbox.Group
                label={t('pins.permissions')}
                value={pinPermissions}
                onChange={setPinPermissions}
              >
                <Stack gap="xs" mt={4}>
                  <Checkbox value="gate:trigger_open" label={t('permissions.triggerOpen')} />
                  <Checkbox value="gate:trigger_close" label={t('permissions.triggerClose')} />
                  <Checkbox value="gate:read_status" label={t('permissions.viewStatus')} />
                </Stack>
              </Checkbox.Group>
              <TextInput
                label={t('pins.expires')}
                description={t('common.optional')}
                type="datetime-local"
                value={pinExpiresAt}
                onChange={(e) => setPinExpiresAt(e.target.value)}
              />
              {schedules.length > 0 && (
                <Select
                  label={t('pins.schedule')}
                  description={t('pins.scheduleDesc')}
                  value={pinScheduleId}
                  onChange={(v) => setPinScheduleId(v ?? '')}
                  data={scheduleSelectData}
                  clearable
                />
              )}
              <Group justify="flex-end">
                <Button variant="default" onClick={() => { closePinModal(); resetPinForm() }}>{t('common.cancel')}</Button>
                <Button type="submit" loading={createPin.isPending || updatePin.isPending}>
                  {pinModalMode === 'edit' ? t('common.save') : t('common.add')}
                </Button>
              </Group>
            </Stack>
          </form>
        </Modal>

      {/* Custom domains */}
      <Paper withBorder p="md" radius="md" mb="md">
        <Group justify="space-between" mb="sm">
          <Group gap="xs">
            <Globe size={16} opacity={0.6} />
            <Text fw={600}>{t('domains.title')}</Text>
            <Badge variant="light" size="xs">{domains?.length ?? 0}</Badge>
          </Group>
          {canManageGate && (
            <Button size="xs" variant="subtle" leftSection={<Plus size={14} />} onClick={openDomainModal}>
              {t('domains.add')}
            </Button>
          )}
        </Group>

        <Modal opened={domainModalOpened} onClose={closeDomainModal} title={t('domains.add')}>
          <form onSubmit={(e) => { e.preventDefault(); addDomain.mutate() }}>
            <Stack>
              <TextInput
                label={t('domains.domain')}
                value={domainValue}
                onChange={(e) => setDomainValue(e.target.value)}
                required
                placeholder={t('domains.domainPlaceholder')}
                styles={{ input: { fontFamily: 'monospace' } }}
              />
              <Group justify="flex-end">
                <Button variant="default" onClick={closeDomainModal}>{t('common.cancel')}</Button>
                <Button type="submit" loading={addDomain.isPending}>{t('common.add')}</Button>
              </Group>
            </Stack>
          </form>
        </Modal>

        {domains?.length === 0 ? (
          <Text size="sm" c="dimmed">{t('domains.noDomains')}</Text>
        ) : (
          <Stack gap="sm">
            {domains?.map((d) => (
              <Paper key={d.id} withBorder p="sm" radius="sm">
                <Group justify="space-between" mb={d.verified_at ? 0 : 'xs'}>
                  <Group gap="xs">
                    {d.verified_at
                      ? <CheckCircle2 size={16} color="var(--mantine-color-green-6)" />
                      : <XCircle size={16} color="var(--mantine-color-orange-6)" />
                    }
                    <Text size="sm" ff="mono">{d.domain}</Text>
                  </Group>
                  <Group gap="xs">
                    {!d.verified_at && canManageGate && (
                      <Button
                        size="xs"
                        variant="light"
                        color="orange"
                        loading={verifyDomain.isPending}
                        onClick={() => verifyDomain.mutate(d.id)}
                      >
                        {t('domains.verifyDns')}
                      </Button>
                    )}
                    {canManageGate && (
                      <ActionIcon variant="subtle" color="red" size="sm" onClick={() => deleteDomain.mutate(d.id)}>
                        <Trash2 size={14} />
                      </ActionIcon>
                    )}
                  </Group>
                </Group>

                {!d.verified_at && (
                  <Alert variant="light" color="gray" mt="xs">
                    <Text size="xs" c="dimmed" mb={4}>{t('domains.dnsInstructions')}</Text>
                    <Group gap="xs" wrap="nowrap">
                      <Code style={{ flex: 1, fontSize: 11 }}>
                        _gaty.{d.domain} → {d.dns_challenge_token}
                      </Code>
                      <Tooltip label={clipboard.copied ? t('common.copied') : t('common.copy')}>
                        <ActionIcon
                          variant="subtle"
                          size="sm"
                          onClick={() => clipboard.copy(d.dns_challenge_token)}
                        >
                          {clipboard.copied ? <Check size={12} /> : <Copy size={12} />}
                        </ActionIcon>
                      </Tooltip>
                    </Group>
                    {verifyResult[d.id] && !verifyResult[d.id].verified && (
                      <Text size="xs" c="red" mt={4}>{verifyResult[d.id].message}</Text>
                    )}
                  </Alert>
                )}
              </Paper>
            ))}
          </Stack>
        )}
      </Paper>

    </Container>
  )
}
