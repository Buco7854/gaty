import { useState, useMemo, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { gatesApi, pinsApi, domainsApi, policiesApi, schedulesApi } from '@/api'
import type { ActionConfig, PinMetadata } from '@/api'
import type { Gate, GatePin, CustomDomain, WorkspaceWithRole, AccessSchedule, MetaField, StatusRule, GateStatus } from '@/types'
import { useAuthStore } from '@/store/auth'
import { findLocalSession } from '@/utils/session'
import { useTranslation } from 'react-i18next'
import { notifySuccess, notifyError } from '@/lib/notify'
import { useGateEvents } from '@/hooks/useGateEvents'
import type { GateEvent } from '@/hooks/useGateEvents'
import {
  Container, Title, Text, Group, Button, Stack, Paper, Badge, ActionIcon,
  TextInput, PasswordInput, Select, Tooltip, Modal, Code, Alert,
  NumberInput, Checkbox, Divider,
} from '@mantine/core'
import { useDisclosure, useClipboard } from '@mantine/hooks'
import {
  ArrowLeft, Zap, Hash, Globe, Plus, Trash2, CheckCircle2, XCircle,
  Clock, Copy, Check, Settings2, Pencil, Info, CalendarClock,
  Key, RefreshCw, Activity, DoorOpen, DoorClosed,
} from 'lucide-react'

// ---------- helpers ----------

function getStatusColor(status: GateStatus | undefined): string {
  switch (status) {
    case 'online':
    case 'open': return 'green'
    case 'offline':
    case 'closed': return 'red'
    case 'unresponsive': return 'orange'
    default: return 'gray'
  }
}

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

/** Inline editor for a list of MetaField entries. */
function MetaConfigEditor({
  value,
  onChange,
}: {
  value: MetaField[]
  onChange: (v: MetaField[]) => void
}) {
  const { t } = useTranslation()

  function updateField(idx: number, patch: Partial<MetaField>) {
    onChange(value.map((f, i) => (i === idx ? { ...f, ...patch } : f)))
  }

  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <div>
          <Text size="sm" fw={500}>{t('gates.metaConfig')}</Text>
          <Text size="xs" c="dimmed">{t('gates.metaConfigDesc')}</Text>
        </div>
        <Button
          size="xs"
          variant="subtle"
          leftSection={<Plus size={12} />}
          onClick={() => onChange([...value, { key: '', label: '', unit: '' }])}
        >
          {t('gates.metaConfigAdd')}
        </Button>
      </Group>
      {value.map((field, idx) => (
        <Group key={idx} gap="xs" align="flex-end">
          <TextInput
            label={idx === 0 ? t('gates.metaConfigKey') : undefined}
            placeholder={t('gates.metaConfigKeyPlaceholder')}
            value={field.key}
            onChange={(e) => updateField(idx, { key: e.target.value })}
            style={{ flex: 2 }}
            styles={{ input: { fontFamily: 'monospace', fontSize: 12 } }}
          />
          <TextInput
            label={idx === 0 ? t('gates.metaConfigLabel') : undefined}
            placeholder={t('gates.metaConfigLabelPlaceholder')}
            value={field.label}
            onChange={(e) => updateField(idx, { label: e.target.value })}
            style={{ flex: 2 }}
          />
          <TextInput
            label={idx === 0 ? t('gates.metaConfigUnit') : undefined}
            placeholder={t('gates.metaConfigUnitPlaceholder')}
            value={field.unit ?? ''}
            onChange={(e) => updateField(idx, { unit: e.target.value })}
            style={{ flex: 1 }}
          />
          <ActionIcon
            variant="subtle"
            color="red"
            mb={idx === 0 ? 0 : undefined}
            onClick={() => onChange(value.filter((_, i) => i !== idx))}
          >
            <Trash2 size={14} />
          </ActionIcon>
        </Group>
      ))}
    </Stack>
  )
}

const STATUS_RULE_OPS = ['eq', 'ne', 'gt', 'gte', 'lt', 'lte'] as const

/** Inline editor for a list of StatusRule entries. */
function StatusRulesEditor({
  value,
  onChange,
}: {
  value: StatusRule[]
  onChange: (v: StatusRule[]) => void
}) {
  const { t } = useTranslation()

  function updateRule(idx: number, patch: Partial<StatusRule>) {
    onChange(value.map((r, i) => (i === idx ? { ...r, ...patch } : r)))
  }

  const opData = STATUS_RULE_OPS.map((op) => ({
    value: op,
    label: t(`gates.statusRulesOp${op.charAt(0).toUpperCase()}${op.slice(1)}`),
  }))

  return (
    <Stack gap="sm">
      <Group justify="space-between">
        <div>
          <Text size="sm" fw={500}>{t('gates.statusRules')}</Text>
          <Text size="xs" c="dimmed">{t('gates.statusRulesDesc')}</Text>
        </div>
        <Button
          size="xs"
          variant="subtle"
          leftSection={<Plus size={12} />}
          onClick={() => onChange([...value, { key: '', op: 'lt', value: '', set_status: '' }])}
        >
          {t('gates.statusRulesAdd')}
        </Button>
      </Group>
      {value.map((rule, idx) => (
        <Group key={idx} gap="xs" align="flex-end">
          <TextInput
            label={idx === 0 ? t('gates.statusRulesKey') : undefined}
            placeholder={t('gates.statusRulesKeyPlaceholder')}
            value={rule.key}
            onChange={(e) => updateRule(idx, { key: e.target.value })}
            style={{ flex: 2 }}
            styles={{ input: { fontFamily: 'monospace', fontSize: 12 } }}
          />
          <Select
            label={idx === 0 ? t('gates.statusRulesOp') : undefined}
            value={rule.op}
            onChange={(v) => updateRule(idx, { op: v ?? 'lt' })}
            data={opData}
            style={{ flex: 2 }}
          />
          <TextInput
            label={idx === 0 ? t('gates.statusRulesValue') : undefined}
            placeholder={t('gates.statusRulesValuePlaceholder')}
            value={rule.value}
            onChange={(e) => updateRule(idx, { value: e.target.value })}
            style={{ flex: 1 }}
            styles={{ input: { fontFamily: 'monospace', fontSize: 12 } }}
          />
          <TextInput
            label={idx === 0 ? t('gates.statusRulesSetStatus') : undefined}
            placeholder={t('gates.statusRulesSetStatusPlaceholder')}
            value={rule.set_status}
            onChange={(e) => updateRule(idx, { set_status: e.target.value })}
            style={{ flex: 2 }}
            styles={{ input: { fontFamily: 'monospace', fontSize: 12 } }}
          />
          <ActionIcon
            variant="subtle"
            color="red"
            mb={idx === 0 ? 0 : undefined}
            onClick={() => onChange(value.filter((_, i) => i !== idx))}
          >
            <Trash2 size={14} />
          </ActionIcon>
        </Group>
      ))}
    </Stack>
  )
}

// ---------- Main page ----------

export default function GatePage() {
  const { wsId, gateId } = useParams<{ wsId: string; gateId: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { t } = useTranslation()
  const clipboard = useClipboard({ timeout: 2000 })
  const tokenClipboard = useClipboard({ timeout: 2000 })

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
  const canManageGate =
    canManage || myPolicies?.some((p) => p.gate_id === gateId && p.permission_code === 'gate:manage')
  const canViewStatus =
    canManage || myPolicies?.some((p) => p.gate_id === gateId && p.permission_code === 'gate:read_status')

  // Modal state
  const [pinModalOpened, { open: openPinModal, close: closePinModal }] = useDisclosure(false)
  const [domainModalOpened, { open: openDomainModal, close: closeDomainModal }] = useDisclosure(false)
  const [configModalOpened, { open: openConfigModal, close: closeConfigModal }] = useDisclosure(false)
  const [tokenWarningOpened, { open: openTokenWarning, close: closeTokenWarning }] = useDisclosure(false)

  // Token visibility
  const [showToken, setShowToken] = useState(false)

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
  const [editMetaConfig, setEditMetaConfig] = useState<MetaField[]>([])
  const [editStatusRules, setEditStatusRules] = useState<StatusRule[]>([])

  const PIN_SESSION_PRESETS = [
    { value: '0', label: t('members.sessionInfinite') },
    { value: 'custom', label: t('members.sessionCustom') },
    { value: '3600', label: t('members.session1h') },
    { value: '28800', label: t('members.session8h') },
    { value: '86400', label: t('members.session24h') },
    { value: '', label: t('members.session7d') },
    { value: '2592000', label: t('members.session30d') },
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
    refetchInterval: 15_000,
  })

  // SSE: update gate data in real-time when a status event arrives
  const handleGateEvent = useCallback(
    (event: GateEvent) => {
      if (event.gate_id !== gateId) return
      qc.setQueryData<Gate>(['gate', wsId, gateId], (prev) =>
        prev
          ? { ...prev, status: event.status as GateStatus, status_metadata: event.status_metadata ?? prev.status_metadata }
          : prev
      )
    },
    [qc, wsId, gateId]
  )
  useGateEvents(globalAuth ? wsId : undefined, handleGateEvent)

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

  // Lazy token fetch: only triggered when admin clicks "Show token"
  const { data: tokenData } = useQuery({
    queryKey: ['gate-token', wsId, gateId],
    queryFn: () => gatesApi.getToken(wsId!, gateId!),
    enabled: canManage && showToken,
  })
  const gateToken = tokenData?.gate_token

  const rotateToken = useMutation({
    mutationFn: () => gatesApi.rotateToken(wsId!, gateId!),
    onSuccess: (data) => {
      qc.setQueryData(['gate-token', wsId, gateId], data)
      setShowToken(true)
      closeTokenWarning()
      notifySuccess(t('gates.tokenRotated'))
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const trigger = useMutation({
    mutationFn: (action: 'open' | 'close') => gatesApi.trigger(wsId!, gateId!, action),
    onSuccess: () => notifySuccess(t('pinpad.gateOpened')),
    onError: (err: unknown) => notifyError(err, t('pinpad.unreachable')),
  })

  const updateConfig = useMutation({
    mutationFn: () =>
      gatesApi.update(wsId!, gateId!, {
        open_config: editOpenConfig,
        close_config: editCloseConfig,
        status_config: editStatusConfig,
        meta_config: editMetaConfig,
        status_rules: editStatusRules,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['gate', wsId, gateId] })
      closeConfigModal()
      notifySuccess(t('common.saved'))
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
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
      notifySuccess(t('common.created'))
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
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
      await pinsApi.update(wsId!, gateId!, editingPinId!, { label: pinLabel, metadata })
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
      notifySuccess(t('common.saved'))
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const deletePin = useMutation({
    mutationFn: (pinId: string) => pinsApi.delete(wsId!, gateId!, pinId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pins', wsId, gateId] }),
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const addDomain = useMutation({
    mutationFn: () => domainsApi.create(wsId!, gateId!, domainValue),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['domains', wsId, gateId] })
      closeDomainModal()
      setDomainValue('')
      notifySuccess(t('common.created'))
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const deleteDomain = useMutation({
    mutationFn: (domainId: string) => domainsApi.delete(wsId!, gateId!, domainId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['domains', wsId, gateId] }),
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const verifyDomain = useMutation({
    mutationFn: (domainId: string) => domainsApi.verify(wsId!, gateId!, domainId),
    onSuccess: (data, domainId) => {
      setVerifyResult((prev) => ({ ...prev, [domainId]: data }))
      if (data.verified) {
        qc.invalidateQueries({ queryKey: ['domains', wsId, gateId] })
        notifySuccess(t('domains.verified'))
      } else {
        notifyError(null, t('domains.notYetVerified'))
      }
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  function openConfig() {
    setEditOpenConfig(gate?.open_config ?? null)
    setEditCloseConfig(gate?.close_config ?? null)
    setEditStatusConfig(gate?.status_config ?? null)
    setEditMetaConfig(gate?.meta_config ?? [])
    setEditStatusRules(gate?.status_rules ?? [])
    openConfigModal()
  }

  // Build metadata display rows: mapped fields + unmapped raw fields (admin only)
  const metaRows = useMemo(() => {
    if (!gate?.status_metadata) return []
    const cfg = gate.meta_config ?? []
    const mapped = cfg
      .filter((f) => f.key in (gate.status_metadata ?? {}))
      .map((f) => ({
        label: f.label,
        value: String((gate.status_metadata ?? {})[f.key] ?? ''),
        unit: f.unit,
        raw: false,
      }))
    if (canManage) {
      const mappedKeys = new Set(cfg.map((f) => f.key))
      const rawRows = Object.entries(gate.status_metadata ?? {})
        .filter(([k]) => !mappedKeys.has(k))
        .map(([k, v]) => ({ label: k, value: String(v ?? ''), unit: undefined, raw: true }))
      return [...mapped, ...rawRows]
    }
    return mapped
  }, [gate, canManage])

  const statusColor = getStatusColor(gate?.status)
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
      {/* Back button */}
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

      {/* Header */}
      <Group justify="space-between" mb="xl">
        <div>
          <Group gap="sm">
            <Title order={2}>{gate?.name ?? '…'}</Title>
            {gate && (
              <Badge color={statusColor} variant="light">
                {t(`common.${gate.status}`, { defaultValue: gate.status })}
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
            leftSection={<DoorClosed size={16} />}
            variant="default"
            loading={trigger.isPending}
            onClick={() => trigger.mutate('close')}
          >
            {t('gates.close')}
          </Button>
          <Button
            leftSection={<DoorOpen size={16} />}
            loading={trigger.isPending}
            onClick={() => trigger.mutate('open')}
          >
            {t('gates.open')}
          </Button>
        </Group>
      </Group>

      {/* Live data (status metadata) */}
      {canViewStatus && (
        <Paper withBorder p="md" radius="md" mb="md">
          <Group gap="xs" mb={metaRows.length > 0 ? 'sm' : 0}>
            <Activity size={16} opacity={0.6} />
            <Text fw={600}>{t('gates.liveData')}</Text>
          </Group>
          {metaRows.length === 0 ? (
            <Text size="sm" c="dimmed">{t('gates.noLiveData')}</Text>
          ) : (
            <Stack gap={4}>
              {metaRows.map((row) => (
                <Group key={row.label} justify="space-between" py={2}>
                  <Text size="sm" c={row.raw ? 'dimmed' : undefined} ff={row.raw ? 'mono' : undefined}>
                    {row.label}
                    {row.raw && <Text component="span" size="xs" c="dimmed"> ({t('gates.metaConfigRaw')})</Text>}
                  </Text>
                  <Text size="sm" fw={500} ff="mono">
                    {row.value}{row.unit ? ` ${row.unit}` : ''}
                  </Text>
                </Group>
              ))}
            </Stack>
          )}
        </Paper>
      )}

      {/* Gate token (admin only) */}
      {canManage && (
        <Paper withBorder p="md" radius="md" mb="md">
          <Group justify="space-between" mb="xs">
            <Group gap="xs">
              <Key size={16} opacity={0.6} />
              <Text fw={600}>{t('gates.tokenSection')}</Text>
            </Group>
            <Group gap="xs">
              <Button size="xs" variant="subtle" onClick={() => setShowToken((v) => !v)}>
                {showToken ? t('gates.tokenHide') : t('gates.tokenShow')}
              </Button>
              <Button
                size="xs"
                variant="light"
                color="orange"
                leftSection={<RefreshCw size={12} />}
                loading={rotateToken.isPending}
                onClick={openTokenWarning}
              >
                {t('gates.tokenRotate')}
              </Button>
            </Group>
          </Group>
          <Text size="xs" c="dimmed" mb="sm">{t('gates.tokenDesc')}</Text>

          {showToken && (
            gateToken ? (
              <Group gap="xs" wrap="nowrap">
                <Code style={{ flex: 1, fontSize: 11, wordBreak: 'break-all' }}>{gateToken}</Code>
                <Tooltip label={tokenClipboard.copied ? t('common.copied') : t('common.copy')}>
                  <ActionIcon
                    variant="subtle"
                    size="sm"
                    onClick={() => tokenClipboard.copy(gateToken)}
                  >
                    {tokenClipboard.copied ? <Check size={12} /> : <Copy size={12} />}
                  </ActionIcon>
                </Tooltip>
              </Group>
            ) : (
              <Text size="sm" c="dimmed">…</Text>
            )
          )}
        </Paper>
      )}

      {/* Rotate token warning modal */}
      <Modal
        opened={tokenWarningOpened}
        onClose={closeTokenWarning}
        title={t('gates.tokenRotate')}
        size="sm"
      >
        <Stack>
          <Alert color="orange" variant="light" icon={<Info size={14} />}>
            <Text size="sm">{t('gates.tokenRotateWarning')}</Text>
          </Alert>
          <Group justify="flex-end">
            <Button variant="default" onClick={closeTokenWarning}>{t('common.cancel')}</Button>
            <Button
              color="orange"
              loading={rotateToken.isPending}
              onClick={() => rotateToken.mutate()}
            >
              {t('gates.tokenRotate')}
            </Button>
          </Group>
        </Stack>
      </Modal>

      {/* Integration config modal */}
      <Modal
        opened={configModalOpened}
        onClose={closeConfigModal}
        title={t('gates.integration')}
        size="lg"
      >
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
            <Divider />
            <MetaConfigEditor value={editMetaConfig} onChange={setEditMetaConfig} />
            <Divider />
            <StatusRulesEditor value={editStatusRules} onChange={setEditStatusRules} />
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
            <Button
              size="xs"
              variant="subtle"
              leftSection={<Plus size={14} />}
              onClick={() => { resetPinForm(); openPinModal() }}
            >
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
                      <ActionIcon
                        variant="subtle"
                        color="red"
                        size="sm"
                        onClick={() => deletePin.mutate(pin.id)}
                      >
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
        <form
          onSubmit={(e) => {
            e.preventDefault()
            pinModalMode === 'edit' ? updatePin.mutate() : createPin.mutate()
          }}
        >
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
              <Text size="xs">
                {pinCodeType === 'pin' ? t('pins.methodWarningPin') : t('pins.methodWarningPassword')}
              </Text>
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
                styles={
                  pinCodeType === 'pin'
                    ? { input: { fontFamily: 'monospace', letterSpacing: '0.2em' } }
                    : undefined
                }
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
              <Button variant="default" onClick={() => { closePinModal(); resetPinForm() }}>
                {t('common.cancel')}
              </Button>
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
                      <ActionIcon
                        variant="subtle"
                        color="red"
                        size="sm"
                        onClick={() => deleteDomain.mutate(d.id)}
                      >
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
                        _gatie.{d.domain} → {d.dns_challenge_token}
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
