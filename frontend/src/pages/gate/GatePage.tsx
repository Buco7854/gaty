import { useState, useMemo, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { gatesApi, pinsApi, domainsApi, policiesApi, schedulesApi } from '@/api'
import type { ActionConfig, PinMetadata } from '@/api'
import type { Gate, GatePin, CustomDomain, AccessSchedule, MetaField, StatusRule, StatusTransition, GateStatus } from '@/types'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'
import { notifySuccess, notifyError } from '@/lib/notify'
import { getNestedValue, hasNestedKey } from '@/lib/utils'
import { QueryError } from '@/components/QueryError'
import { useGateEvents } from '@/hooks/useGateEvents'
import type { GateEvent } from '@/hooks/useGateEvents'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { SimpleSelect } from '@/components/ui/select'
import { SimpleTooltip } from '@/components/ui/tooltip'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Separator } from '@/components/ui/separator'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  ArrowLeft, Hash, Globe, Plus, Trash2, CheckCircle2, XCircle,
  Clock, Copy, Check, Settings2, Pencil, Info, CalendarClock,
  Key, RefreshCw, Activity, DoorOpen, DoorClosed,
} from 'lucide-react'

// ---------- helpers ----------

function flattenKeys(obj: Record<string, unknown>, prefix = ''): string[] {
  const keys: string[] = []
  for (const [k, v] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${k}` : k
    if (v != null && typeof v === 'object' && !Array.isArray(v)) {
      keys.push(...flattenKeys(v as Record<string, unknown>, path))
    } else {
      keys.push(path)
    }
  }
  return keys
}

const DEFAULT_STATUSES = ['open', 'closed', 'unavailable']

function getStatusVariant(status: GateStatus | undefined): 'success' | 'destructive' | 'warning' | 'secondary' {
  switch (status) {
    case 'online':
    case 'open': return 'success'
    case 'offline':
    case 'closed': return 'destructive'
    case 'unresponsive':
    case 'unavailable': return 'warning'
    default: return 'secondary'
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
  const VALID_ACTION_TYPES = ['NONE', 'MQTT_GATIE', 'MQTT_CUSTOM', 'HTTP']
  const rawDriver = value?.type
  const driverType = rawDriver && VALID_ACTION_TYPES.includes(rawDriver) ? rawDriver : 'NONE'

  return (
    <div className="space-y-2">
      <SimpleSelect
        label={label}
        value={driverType}
        onValueChange={(v) => {
          const type = (v ?? 'NONE') as ActionConfig['type']
          if (type === 'NONE') {
            onChange(null)
          } else {
            onChange({ type, config: value?.config })
          }
        }}
        data={[
          { value: 'NONE', label: t('gates.noneDriver') },
          { value: 'MQTT_GATIE', label: t('gates.mqttGatieDriver') },
          { value: 'MQTT_CUSTOM', label: t('gates.mqttCustomDriver') },
          { value: 'HTTP', label: t('gates.httpDriver') },
        ]}
      />
      {driverType === 'MQTT_CUSTOM' && (
        <div>
          <label className="text-sm font-medium">{t('gates.mqttCustomPayload')}</label>
          <p className="text-xs text-muted-foreground mb-1">{t('gates.mqttCustomPayloadDesc')}</p>
          <textarea
            className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
            defaultValue={JSON.stringify(value?.config?.payload ?? {}, null, 2)}
            onBlur={(e) => {
              try {
                const parsed = JSON.parse(e.target.value)
                onChange({ type: 'MQTT_CUSTOM', config: { ...value?.config, payload: parsed } })
              } catch { /* ignore invalid JSON */ }
            }}
            placeholder={'{\n  "cmd": 1\n}'}
            rows={3}
          />
        </div>
      )}
      {driverType === 'HTTP' && (
        <>
          <Input
            label={t('gates.httpUrl')}
            value={(value?.config?.url as string) ?? ''}
            onChange={(e) =>
              onChange({ type: 'HTTP', config: { ...value?.config, url: e.target.value } })
            }
            placeholder="https://api.example.com/open"
            required
          />
          <SimpleSelect
            label={t('gates.httpMethod')}
            value={(value?.config?.method as string) ?? 'POST'}
            onValueChange={(v) =>
              onChange({ type: 'HTTP', config: { ...value?.config, method: v ?? 'POST' } })
            }
            data={['POST', 'GET', 'PUT', 'PATCH']}
          />
        </>
      )}
    </div>
  )
}

type StatusDriverType = 'NONE' | 'MQTT_GATIE' | 'MQTT_CUSTOM' | 'HTTP_INBOUND' | 'HTTP_WEBHOOK'

function StatusConfigForm({
  value,
  onChange,
  allStatuses,
}: {
  value: ActionConfig | null
  onChange: (v: ActionConfig | null) => void
  allStatuses: string[]
}) {
  const { t } = useTranslation()
  const VALID_STATUS_TYPES: StatusDriverType[] = ['NONE', 'MQTT_GATIE', 'MQTT_CUSTOM', 'HTTP_INBOUND', 'HTTP_WEBHOOK']
  const rawType = value?.type as StatusDriverType | undefined
  const type: StatusDriverType = rawType && VALID_STATUS_TYPES.includes(rawType) ? rawType : 'NONE'
  const cfg = (value?.config ?? {}) as Record<string, unknown>

  const mapping = (cfg.mapping as Record<string, unknown>) ?? {}
  const statusM = (mapping.status as Record<string, unknown>) ?? {}
  const statusField = (statusM.field as string) ?? ''
  const statusValues = (statusM.values as Record<string, string>) ?? {}

  const url = (cfg.url as string) ?? ''
  const method = (cfg.method as string) ?? 'GET'
  const headersObj = (cfg.headers as Record<string, string>) ?? {}
  const body = (cfg.body as string) ?? ''
  const intervalSeconds = (cfg.interval_seconds as number) ?? 60
  const successStatusCodes = (cfg.success_status_codes as Array<{ from: number; to: number }>) ?? []

  function emit(newType: StatusDriverType, newCfg: Record<string, unknown>) {
    if (newType === 'NONE') { onChange(null); return }
    onChange({ type: newType as ActionConfig['type'], config: newCfg })
  }

  function setCfgField(key: string, val: unknown) {
    emit(type, { ...cfg, [key]: val })
  }

  function setMapping(patch: Record<string, unknown>) {
    emit(type, { ...cfg, mapping: { ...mapping, ...patch } })
  }

  function setStatusM(patch: Record<string, unknown>) {
    setMapping({ status: { ...statusM, ...patch } })
  }

  const svEntries = Object.entries(statusValues)
  const hEntries = Object.entries(headersObj)

  return (
    <div className="space-y-3">
      <SimpleSelect
        label={t('gates.statusMode')}
        value={type}
        onValueChange={(v) => {
          const nt = (v ?? 'NONE') as StatusDriverType
          if (nt === 'NONE') { onChange(null); return }
          onChange({ type: nt as ActionConfig['type'], config: cfg })
        }}
        data={[
          { value: 'NONE', label: t('gates.statusNone') },
          { value: 'MQTT_GATIE', label: t('gates.statusMqttGatie') },
          { value: 'MQTT_CUSTOM', label: t('gates.statusMqttCustom') },
          { value: 'HTTP_INBOUND', label: t('gates.statusHttpInbound') },
          { value: 'HTTP_WEBHOOK', label: t('gates.statusHttpWebhook') },
        ]}
      />

      {type === 'HTTP_WEBHOOK' && (
        <div className="space-y-2">
          <Input
            label={t('gates.httpUrl')}
            value={url}
            onChange={(e) => setCfgField('url', e.target.value)}
            placeholder="http://192.168.1.100/api/status"
          />
          <div className="grid grid-cols-2 gap-2">
            <SimpleSelect
              label={t('gates.httpMethod')}
              value={method}
              onValueChange={(v) => setCfgField('method', v ?? 'GET')}
              data={['GET', 'POST', 'PUT', 'PATCH']}
            />
            <Input
              label={t('gates.webhookInterval')}
              type="number"
              value={String(intervalSeconds)}
              onChange={(e) => setCfgField('interval_seconds', Number(e.target.value) || 60)}
              min={1}
              placeholder={t('gates.webhookIntervalPlaceholder')}
            />
          </div>

          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-sm font-medium">{t('gates.httpHeaders')}</span>
              <Button size="sm" variant="ghost" onClick={() => setCfgField('headers', { ...headersObj, [`Header-${hEntries.length + 1}`]: '' })}>
                <Plus className="h-3 w-3" />
                {t('common.add')}
              </Button>
            </div>
            <div className="space-y-1">
              {hEntries.map(([k, v], idx) => (
                <div key={idx} className="flex items-center gap-1">
                  <input
                    className="flex-1 rounded-md border bg-background px-2 py-1 text-xs font-mono"
                    placeholder="Authorization"
                    defaultValue={k}
                    onBlur={(e) => {
                      if (e.target.value === k) return
                      const newH: Record<string, string> = {}
                      for (const [hk, hv] of Object.entries(headersObj)) newH[hk === k ? e.target.value : hk] = hv
                      setCfgField('headers', newH)
                    }}
                  />
                  <input
                    className="flex-[2] rounded-md border bg-background px-2 py-1 text-xs"
                    placeholder="Bearer …"
                    value={v}
                    onChange={(e) => setCfgField('headers', { ...headersObj, [k]: e.target.value })}
                  />
                  <Button variant="ghost" size="icon-sm" className="text-destructive" onClick={() => {
                    const newH = { ...headersObj }; delete newH[k]; setCfgField('headers', newH)
                  }}>
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          </div>

          <Input
            label={t('gates.httpBody')}
            value={body}
            onChange={(e) => setCfgField('body', e.target.value)}
            placeholder='{"action": "status"}'
            className="font-mono text-xs"
          />

          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-sm font-medium">{t('gates.successStatusCodes')}</span>
              <Button size="sm" variant="ghost" onClick={() => setCfgField('success_status_codes', [...successStatusCodes, { from: 200, to: 299 }])}>
                <Plus className="h-3 w-3" />
                {t('common.add')}
              </Button>
            </div>
            {successStatusCodes.length === 0 && (
              <p className="text-xs text-muted-foreground">{t('gates.successStatusCodesDefault')}</p>
            )}
            <div className="space-y-1">
              {successStatusCodes.map((range, idx) => (
                <div key={idx} className="flex items-center gap-1">
                  <input
                    type="number"
                    className="flex-1 rounded-md border bg-background px-2 py-1 text-xs font-mono"
                    value={range.from}
                    onChange={(e) => {
                      const updated = [...successStatusCodes]
                      updated[idx] = { ...updated[idx], from: Number(e.target.value) || 200 }
                      setCfgField('success_status_codes', updated)
                    }}
                    min={100} max={599}
                    placeholder="200"
                  />
                  <span className="text-sm text-muted-foreground">–</span>
                  <input
                    type="number"
                    className="flex-1 rounded-md border bg-background px-2 py-1 text-xs font-mono"
                    value={range.to}
                    onChange={(e) => {
                      const updated = [...successStatusCodes]
                      updated[idx] = { ...updated[idx], to: Number(e.target.value) || 299 }
                      setCfgField('success_status_codes', updated)
                    }}
                    min={100} max={599}
                    placeholder="299"
                  />
                  <Button variant="ghost" size="icon-sm" className="text-destructive" onClick={() => {
                    setCfgField('success_status_codes', successStatusCodes.filter((_, i) => i !== idx))
                  }}>
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {type === 'MQTT_GATIE' && (
        <p className="text-xs text-muted-foreground">{t('gates.mqttGatieInfo')}</p>
      )}

      {type !== 'NONE' && type !== 'MQTT_GATIE' && (
        <div className="space-y-3">
          <div>
            <Input
              label={t('gates.statusField')}
              description={t('gates.statusFieldHint')}
              value={statusField}
              onChange={(e) => setStatusM({ field: e.target.value })}
              placeholder={t('gates.statusFieldPlaceholder')}
              className="font-mono text-xs"
            />
          </div>

          <div>
            <div className="flex items-center justify-between mb-1">
              <div>
                <p className="text-sm font-medium">{t('gates.statusValues')}</p>
                <p className="text-xs text-muted-foreground">{t('gates.statusValuesDesc')}</p>
              </div>
              <Button size="sm" variant="ghost" onClick={() => setStatusM({ values: { ...statusValues, [`val_${svEntries.length + 1}`]: '' } })}>
                <Plus className="h-3 w-3" />
                {t('gates.statusValuesAdd')}
              </Button>
            </div>
            <div className="space-y-1">
              {svEntries.map(([raw, mapped], idx) => (
                <div key={idx} className="flex items-center gap-1">
                  <input
                    className="flex-1 rounded-md border bg-background px-2 py-1 text-xs font-mono"
                    placeholder={t('gates.statusValuesRawPlaceholder')}
                    defaultValue={raw}
                    onBlur={(e) => {
                      if (e.target.value === raw) return
                      const newV: Record<string, string> = {}
                      for (const [vk, vv] of Object.entries(statusValues)) newV[vk === raw ? e.target.value : vk] = vv
                      setStatusM({ values: newV })
                    }}
                  />
                  <span className="text-muted-foreground">→</span>
                  <select
                    className="flex-[2] rounded-md border bg-background px-2 py-1 text-xs"
                    value={mapped}
                    onChange={(e) => setStatusM({ values: { ...statusValues, [raw]: e.target.value } })}
                  >
                    {allStatuses.map((s) => (
                      <option key={s} value={s}>{s}</option>
                    ))}
                  </select>
                  <Button variant="ghost" size="icon-sm" className="text-destructive"
                    onClick={() => { const nv = { ...statusValues }; delete nv[raw]; setStatusM({ values: nv }) }}>
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

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
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium">{t('gates.metaConfig')}</p>
          <p className="text-xs text-muted-foreground">{t('gates.metaConfigDesc')}</p>
        </div>
        <Button size="sm" variant="ghost" onClick={() => onChange([...value, { key: '', label: '', unit: '' }])}>
          <Plus className="h-3 w-3" />
          {t('gates.metaConfigAdd')}
        </Button>
      </div>
      {value.map((field, idx) => (
        <div key={idx} className="flex items-center gap-1">
          <input
            className="flex-[2] rounded-md border bg-background px-2 py-1 text-xs font-mono"
            placeholder={t('gates.metaConfigKeyPlaceholder')}
            value={field.key}
            onChange={(e) => updateField(idx, { key: e.target.value })}
          />
          <input
            className="flex-[2] rounded-md border bg-background px-2 py-1 text-xs"
            placeholder={t('gates.metaConfigLabelPlaceholder')}
            value={field.label}
            onChange={(e) => updateField(idx, { label: e.target.value })}
          />
          <input
            className="flex-1 rounded-md border bg-background px-2 py-1 text-xs"
            placeholder={t('gates.metaConfigUnitPlaceholder')}
            value={field.unit ?? ''}
            onChange={(e) => updateField(idx, { unit: e.target.value })}
          />
          <Button variant="ghost" size="icon-sm" className="text-destructive"
            onClick={() => onChange(value.filter((_, i) => i !== idx))}>
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      ))}
    </div>
  )
}

function CustomStatusesEditor({
  value,
  onChange,
}: {
  value: string[]
  onChange: (v: string[]) => void
}) {
  const { t } = useTranslation()
  const [newStatus, setNewStatus] = useState('')

  function addStatus() {
    const trimmed = newStatus.trim().toLowerCase().replace(/\s+/g, '_')
    if (!trimmed || DEFAULT_STATUSES.includes(trimmed) || value.includes(trimmed)) return
    onChange([...value, trimmed])
    setNewStatus('')
  }

  return (
    <div className="space-y-3">
      <div>
        <p className="text-sm font-medium">{t('gates.customStatuses')}</p>
        <p className="text-xs text-muted-foreground">{t('gates.customStatusesDesc')}</p>
      </div>
      <div className="flex flex-wrap gap-1.5">
        {DEFAULT_STATUSES.map((s) => (
          <Badge key={s} variant={getStatusVariant(s as GateStatus)}>
            {t(`common.${s}`, { defaultValue: s })}
          </Badge>
        ))}
        {value.map((s, idx) => (
          <Badge key={s} variant="secondary" className="gap-1">
            {s}
            <button
              type="button"
              className="text-destructive hover:text-destructive/80 cursor-pointer"
              onClick={() => onChange(value.filter((_, i) => i !== idx))}
            >
              <Trash2 className="h-2.5 w-2.5" />
            </button>
          </Badge>
        ))}
      </div>
      <div className="flex items-center gap-1">
        <input
          className="flex-1 rounded-md border bg-background px-2 py-1 text-xs font-mono"
          placeholder={t('gates.customStatusPlaceholder')}
          value={newStatus}
          onChange={(e) => setNewStatus(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addStatus() } }}
        />
        <Button size="sm" variant="ghost" onClick={addStatus}>
          <Plus className="h-3 w-3" />
          {t('common.add')}
        </Button>
      </div>
    </div>
  )
}

const STATUS_RULE_OPS = ['eq', 'ne', 'gt', 'gte', 'lt', 'lte'] as const

function StatusRulesEditor({
  value,
  onChange,
  allStatuses,
}: {
  value: StatusRule[]
  onChange: (v: StatusRule[]) => void
  allStatuses: string[]
}) {
  const { t } = useTranslation()

  function updateRule(idx: number, patch: Partial<StatusRule>) {
    onChange(value.map((r, i) => (i === idx ? { ...r, ...patch } : r)))
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium">{t('gates.statusRules')}</p>
          <p className="text-xs text-muted-foreground">{t('gates.statusRulesDesc')}</p>
        </div>
        <Button size="sm" variant="ghost"
          onClick={() => onChange([...value, { key: '', op: 'lt', value: '', set_status: allStatuses[0] ?? '' }])}>
          <Plus className="h-3 w-3" />
          {t('gates.statusRulesAdd')}
        </Button>
      </div>
      {value.map((rule, idx) => (
        <div key={idx} className="flex items-center gap-1">
          <input
            className="flex-[2] rounded-md border bg-background px-2 py-1 text-xs font-mono"
            placeholder={t('gates.statusRulesKeyPlaceholder')}
            value={rule.key}
            onChange={(e) => updateRule(idx, { key: e.target.value })}
          />
          <select
            className="flex-[2] rounded-md border bg-background px-2 py-1 text-xs"
            value={rule.op}
            onChange={(e) => updateRule(idx, { op: e.target.value })}
          >
            {STATUS_RULE_OPS.map((op) => (
              <option key={op} value={op}>
                {t(`gates.statusRulesOp${op.charAt(0).toUpperCase()}${op.slice(1)}`)}
              </option>
            ))}
          </select>
          <input
            className="flex-1 rounded-md border bg-background px-2 py-1 text-xs font-mono"
            placeholder={t('gates.statusRulesValuePlaceholder')}
            value={rule.value}
            onChange={(e) => updateRule(idx, { value: e.target.value })}
          />
          <select
            className="flex-[2] rounded-md border bg-background px-2 py-1 text-xs"
            value={rule.set_status}
            onChange={(e) => updateRule(idx, { set_status: e.target.value })}
          >
            {allStatuses.map((s) => (
              <option key={s} value={s}>{t(`common.${s}`, { defaultValue: s })}</option>
            ))}
          </select>
          <Button variant="ghost" size="icon-sm" className="text-destructive"
            onClick={() => onChange(value.filter((_, i) => i !== idx))}>
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
      ))}
    </div>
  )
}

// ---------- Main page ----------

export default function GatePage() {
  const { gateId } = useParams<{ gateId: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { t } = useTranslation()

  const [copied, setCopied] = useState(false)
  const [tokenCopied, setTokenCopied] = useState(false)

  function copyText(text: string, setFn: (v: boolean) => void) {
    navigator.clipboard.writeText(text)
    setFn(true)
    setTimeout(() => setFn(false), 2000)
  }

  const session = useAuthStore((s) => s.session)
  const isAdmin = session?.type === 'member' && session.member?.role === 'ADMIN'

  const { data: myPolicies } = useQuery({
    queryKey: ['policies-me'],
    queryFn: () => policiesApi.listMine(),
    enabled: !isAdmin && session?.type === 'member',
  })
  const canManageGate =
    isAdmin || myPolicies?.some((p) => p.gate_id === gateId && p.permission_code === 'gate:manage')
  const canViewStatus =
    isAdmin || myPolicies?.some((p) => p.gate_id === gateId && p.permission_code === 'gate:read_status')

  // Dialog state
  const [pinModalOpen, setPinModalOpen] = useState(false)
  const [domainModalOpen, setDomainModalOpen] = useState(false)
  const [configModalOpen, setConfigModalOpen] = useState(false)
  const [tokenWarningOpen, setTokenWarningOpen] = useState(false)

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
  const [editCustomStatuses, setEditCustomStatuses] = useState<string[]>([])
  const [editTTLSeconds, setEditTTLSeconds] = useState<number | null>(null)
  const [editStatusTransitions, setEditStatusTransitions] = useState<StatusTransition[]>([])

  const PIN_SESSION_PRESETS = [
    { value: '__none__', label: t('common.none') },
    { value: '0', label: t('members.sessionInfinite') },
    { value: 'custom', label: t('members.sessionCustom') },
    { value: '3600', label: t('members.session1h') },
    { value: '28800', label: t('members.session8h') },
    { value: '86400', label: t('members.session24h') },
    { value: '604800', label: t('members.session7d') },
    { value: '2592000', label: t('members.session30d') },
  ]

  function resolvePinSessionDurationSeconds(): number | undefined {
    if (pinSessionDuration === '' || pinSessionDuration === '__none__') return undefined
    if (pinSessionDuration === '0') return 0
    if (pinSessionDuration === 'custom') {
      const n = typeof pinCustomValue === 'number' ? pinCustomValue : parseFloat(String(pinCustomValue))
      if (!n || n <= 0) return undefined
      const multipliers: Record<string, number> = { minutes: 60, hours: 3600, days: 86400 }
      return Math.round(n * (multipliers[pinCustomUnit] ?? 3600))
    }
    return parseInt(pinSessionDuration, 10)
  }

  const { data: gate, isError: gateError, error: gateFetchError } = useQuery<Gate>({
    queryKey: ['gate', gateId],
    queryFn: () => gatesApi.get(gateId!),
    refetchInterval: 15_000,
  })

  const handleGateEvent = useCallback(
    (event: GateEvent) => {
      if (event.gate_id !== gateId) return
      const patch = { status: event.status as GateStatus, status_metadata: event.status_metadata }
      qc.setQueryData<Gate>(['gate', gateId], (prev) =>
        prev ? { ...prev, ...patch, status_metadata: patch.status_metadata ?? prev.status_metadata } : prev
      )
      qc.setQueryData<Gate[]>(['gates'], (prev) =>
        prev?.map((g) =>
          g.id === event.gate_id
            ? { ...g, ...patch, status_metadata: patch.status_metadata ?? g.status_metadata }
            : g
        )
      )
    },
    [qc, gateId]
  )
  useGateEvents(handleGateEvent)

  const { data: pins } = useQuery<GatePin[]>({
    queryKey: ['pins', gateId],
    queryFn: () => pinsApi.list(gateId!),
  })

  const { data: domains } = useQuery<CustomDomain[]>({
    queryKey: ['domains', gateId],
    queryFn: () => domainsApi.list(gateId!),
    enabled: canManageGate,
  })

  const { data: schedules = [] } = useQuery<AccessSchedule[]>({
    queryKey: ['schedules'],
    queryFn: () => schedulesApi.list(),
    enabled: canManageGate,
  })

  const { data: tokenData } = useQuery({
    queryKey: ['gate-token', gateId],
    queryFn: () => gatesApi.getToken(gateId!),
    enabled: isAdmin && showToken,
  })
  const gateToken = tokenData?.gate_token

  const rotateToken = useMutation({
    mutationFn: () => gatesApi.rotateToken(gateId!),
    onSuccess: (data) => {
      qc.setQueryData(['gate-token', gateId], data)
      setShowToken(true)
      setTokenWarningOpen(false)
      notifySuccess(t('gates.tokenRotated'))
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const trigger = useMutation({
    mutationFn: (action: 'open' | 'close') => gatesApi.trigger(gateId!, action),
    onMutate: (action) => {
      const optimisticStatus = (action === 'open' ? 'open' : 'closed') as GateStatus
      qc.setQueryData<Gate>(['gate', gateId], (prev) =>
        prev ? { ...prev, status: optimisticStatus } : prev
      )
      qc.setQueryData<Gate[]>(['gates'], (prev) =>
        prev?.map((g) => g.id === gateId ? { ...g, status: optimisticStatus } : g)
      )
    },
    onSuccess: () => notifySuccess(t('pinpad.gateOpened')),
    onError: (err: unknown) => notifyError(err, t('pinpad.unreachable')),
  })

  const updateConfig = useMutation({
    mutationFn: () =>
      gatesApi.update(gateId!, {
        open_config: editOpenConfig,
        close_config: editCloseConfig,
        status_config: editStatusConfig,
        meta_config: editMetaConfig,
        status_rules: editStatusRules,
        custom_statuses: editCustomStatuses,
        ttl_seconds: editTTLSeconds,
        status_transitions: editStatusTransitions,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['gate', gateId] })
      setConfigModalOpen(false)
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
      return pinsApi.create(gateId!, {
        label: pinLabel,
        pin: pinValue,
        code_type: pinCodeType,
        schedule_id: pinScheduleId || undefined,
        metadata,
      })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pins', gateId] })
      setPinModalOpen(false)
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
    setPinSessionDuration(sd === undefined ? '__none__' : sd === 0 ? '0' : String(sd))
    setPinMaxUses(meta.max_uses ?? '')
    setPinScheduleId(pin.schedule_id ?? '')
    setPinModalOpen(true)
  }

  const updatePin = useMutation({
    mutationFn: async () => {
      const metadata: PinMetadata = { permissions: pinPermissions, code_type: pinCodeType }
      metadata.expires_at = pinExpiresAt ? new Date(pinExpiresAt).toISOString() : null
      const dur = resolvePinSessionDurationSeconds()
      metadata.session_duration = dur !== undefined ? dur : null
      const maxUses = typeof pinMaxUses === 'number' ? pinMaxUses : parseInt(String(pinMaxUses), 10)
      metadata.max_uses = maxUses > 0 ? maxUses : null
      await pinsApi.update(gateId!, editingPinId!, { label: pinLabel, metadata })
      if (pinScheduleId) {
        await pinsApi.setSchedule(gateId!, editingPinId!, pinScheduleId)
      } else {
        await pinsApi.clearSchedule(gateId!, editingPinId!).catch(() => {})
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pins', gateId] })
      setPinModalOpen(false)
      resetPinForm()
      notifySuccess(t('common.saved'))
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const deletePin = useMutation({
    mutationFn: (pinId: string) => pinsApi.delete(gateId!, pinId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pins', gateId] }),
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const addDomain = useMutation({
    mutationFn: () => domainsApi.create(gateId!, domainValue),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['domains', gateId] })
      setDomainModalOpen(false)
      setDomainValue('')
      notifySuccess(t('common.created'))
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const deleteDomain = useMutation({
    mutationFn: (domainId: string) => domainsApi.delete(gateId!, domainId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['domains', gateId] }),
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const verifyDomain = useMutation({
    mutationFn: (domainId: string) => domainsApi.verify(gateId!, domainId),
    onSuccess: (data, domainId) => {
      setVerifyResult((prev) => ({ ...prev, [domainId]: data }))
      if (data.verified) {
        qc.invalidateQueries({ queryKey: ['domains', gateId] })
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
    setEditCustomStatuses(gate?.custom_statuses ?? [])
    setEditTTLSeconds(gate?.ttl_seconds ?? null)
    setEditStatusTransitions(gate?.status_transitions ?? [])
    setConfigModalOpen(true)
  }

  const allStatuses = useMemo(
    () => [...DEFAULT_STATUSES, ...editCustomStatuses],
    [editCustomStatuses]
  )

  const metaRows = useMemo(() => {
    if (!gate?.status_metadata) return []
    const meta = gate.status_metadata as Record<string, unknown>
    const cfg = gate.meta_config ?? []
    const mapped = cfg
      .filter((f) => hasNestedKey(meta, f.key))
      .map((f) => ({
        label: f.label,
        value: String(getNestedValue(meta, f.key) ?? ''),
        unit: f.unit,
        raw: false,
      }))
    if (isAdmin) {
      const mappedKeys = new Set(cfg.map((f) => f.key))
      const allKeys = flattenKeys(meta)
      const rawRows = allKeys
        .filter((k) => !mappedKeys.has(k))
        .map((k) => ({ label: k, value: String(getNestedValue(meta, k) ?? ''), unit: undefined, raw: true }))
      return [...mapped, ...rawRows]
    }
    return mapped
  }, [gate, isAdmin])

  const scheduleSelectData = [
    { value: '__none__', label: t('common.none') },
    ...schedules.map((s) => ({ value: s.id, label: s.name })),
  ]
  const scheduleById = useMemo(() => {
    const map: Record<string, AccessSchedule> = {}
    for (const s of schedules) map[s.id] = s
    return map
  }, [schedules])

  return (
    <div className="max-w-xl mx-auto p-6">
      {/* Back button */}
      <Button
        variant="ghost"
        size="sm"
        className="mb-4"
        onClick={() => navigate('/gates')}
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        {t('common.back')}
      </Button>

      {gateError && <QueryError error={gateFetchError} />}

      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-2">
          <h2 className="text-xl font-bold">{gate?.name ?? '…'}</h2>
          {gate && (
            <Badge variant={getStatusVariant(gate.status)}>
              {t(`common.${gate.status}`, { defaultValue: gate.status })}
            </Badge>
          )}
        </div>
        <div className="flex items-center gap-2">
          {canManageGate && (
            <SimpleTooltip label={t('gates.integration')}>
              <Button variant="outline" size="icon" onClick={openConfig}>
                <Settings2 className="h-4 w-4" />
              </Button>
            </SimpleTooltip>
          )}
          {gate?.close_config && (
            <Button
              variant="outline"
              loading={trigger.isPending}
              onClick={() => trigger.mutate('close')}
            >
              <DoorClosed className="h-4 w-4" />
              {t('gates.close')}
            </Button>
          )}
          <Button
            loading={trigger.isPending}
            onClick={() => trigger.mutate('open')}
          >
            <DoorOpen className="h-4 w-4" />
            {t('gates.open')}
          </Button>
        </div>
      </div>

      {/* Live data */}
      {canViewStatus && (
        <div className="border rounded-lg p-4 mb-4">
          <div className="flex items-center gap-1.5 mb-2">
            <Activity className="h-4 w-4 opacity-60" />
            <span className="font-semibold">{t('gates.liveData')}</span>
          </div>
          {metaRows.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('gates.noLiveData')}</p>
          ) : (
            <div className="space-y-1">
              {metaRows.map((row) => (
                <div key={row.label} className="flex items-center justify-between py-0.5">
                  <span className={`text-sm ${row.raw ? 'text-muted-foreground font-mono' : ''}`}>
                    {row.label}
                    {row.raw && <span className="text-xs text-muted-foreground"> ({t('gates.metaConfigRaw')})</span>}
                  </span>
                  <span className="text-sm font-medium font-mono">
                    {row.value}{row.unit ? ` ${row.unit}` : ''}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Gate token (admin only) */}
      {isAdmin && (
        <div className="border rounded-lg p-4 mb-4">
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-1.5">
              <Key className="h-4 w-4 opacity-60" />
              <span className="font-semibold">{t('gates.tokenSection')}</span>
            </div>
            <div className="flex items-center gap-1">
              <Button size="sm" variant="ghost" onClick={() => setShowToken((v) => !v)}>
                {showToken ? t('gates.tokenHide') : t('gates.tokenShow')}
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="text-orange-600"
                loading={rotateToken.isPending}
                onClick={() => setTokenWarningOpen(true)}
              >
                <RefreshCw className="h-3 w-3" />
                {t('gates.tokenRotate')}
              </Button>
            </div>
          </div>
          <p className="text-xs text-muted-foreground mb-2">{t('gates.tokenDesc')}</p>

          {showToken && (
            gateToken ? (
              <div className="flex items-center gap-2">
                <code className="flex-1 text-[11px] break-all bg-muted rounded px-2 py-1">{gateToken}</code>
                <SimpleTooltip label={tokenCopied ? t('common.copied') : t('common.copy')}>
                  <Button variant="ghost" size="icon-sm" onClick={() => copyText(gateToken, setTokenCopied)}>
                    {tokenCopied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </SimpleTooltip>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">…</p>
            )
          )}
        </div>
      )}

      {/* Rotate token warning dialog */}
      <Dialog open={tokenWarningOpen} onOpenChange={setTokenWarningOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('gates.tokenRotate')}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <Alert variant="warning">
              <Info className="h-4 w-4" />
              <AlertDescription>{t('gates.tokenRotateWarning')}</AlertDescription>
            </Alert>
            <div className="flex gap-2">
              <Button variant="outline" className="flex-1" onClick={() => setTokenWarningOpen(false)}>
                {t('common.cancel')}
              </Button>
              <Button className="flex-1 bg-orange-600 hover:bg-orange-700" loading={rotateToken.isPending} onClick={() => rotateToken.mutate()}>
                {t('gates.tokenRotate')}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Integration config dialog */}
      <Dialog open={configModalOpen} onOpenChange={setConfigModalOpen}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>{t('gates.integration')}</DialogTitle>
          </DialogHeader>
          <form onSubmit={(e) => { e.preventDefault(); updateConfig.mutate() }}>
            <div className="space-y-6">
              <div>
                <p className="font-semibold mb-2">{t('gates.actionsSection')}</p>
                <div className="space-y-4">
                  <ActionConfigForm label={t('gates.openAction')} value={editOpenConfig} onChange={setEditOpenConfig} />
                  <ActionConfigForm label={t('gates.closeAction')} value={editCloseConfig} onChange={setEditCloseConfig} />
                </div>
              </div>
              <Separator />
              <div>
                <p className="font-semibold mb-2">{t('gates.statusAction')}</p>
                <StatusConfigForm value={editStatusConfig} onChange={setEditStatusConfig} allStatuses={allStatuses} />
              </div>
              <Separator />
              <MetaConfigEditor value={editMetaConfig} onChange={setEditMetaConfig} />
              <Separator />
              <CustomStatusesEditor value={editCustomStatuses} onChange={setEditCustomStatuses} />
              <Separator />
              <StatusRulesEditor value={editStatusRules} onChange={setEditStatusRules} allStatuses={allStatuses} />
              <Separator />
              <div>
                <p className="font-semibold mb-2">{t('gates.ttlSection')}</p>
                <Input
                  label={t('gates.ttlSeconds')}
                  description={t('gates.ttlSecondsHint')}
                  type="number"
                  value={editTTLSeconds != null ? String(editTTLSeconds) : ''}
                  onChange={(e) => setEditTTLSeconds(e.target.value ? Number(e.target.value) : null)}
                  min={1}
                  max={3600}
                  placeholder="30"
                  className="w-28"
                />
              </div>
              <Separator />
              <div>
                <div className="flex items-center justify-between mb-1">
                  <p className="font-semibold">{t('gates.statusTransitions')}</p>
                  <Button size="sm" variant="ghost" type="button"
                    onClick={() => setEditStatusTransitions([...editStatusTransitions, { from: 'open', to: 'closed', after_seconds: 30 }])}>
                    <Plus className="h-3 w-3" />
                    {t('common.add')}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground mb-2">{t('gates.statusTransitionsHint')}</p>
                <div className="space-y-1">
                  {editStatusTransitions.map((tr, idx) => (
                    <div key={idx} className="flex items-center gap-1">
                      <select
                        className="flex-1 rounded-md border bg-background px-2 py-1 text-xs"
                        value={tr.from}
                        onChange={(e) => {
                          const updated = [...editStatusTransitions]
                          updated[idx] = { ...updated[idx], from: e.target.value }
                          setEditStatusTransitions(updated)
                        }}
                      >
                        {allStatuses.map((s) => <option key={s} value={s}>{s}</option>)}
                      </select>
                      <span className="text-sm text-muted-foreground">→</span>
                      <select
                        className="flex-1 rounded-md border bg-background px-2 py-1 text-xs"
                        value={tr.to}
                        onChange={(e) => {
                          const updated = [...editStatusTransitions]
                          updated[idx] = { ...updated[idx], to: e.target.value }
                          setEditStatusTransitions(updated)
                        }}
                      >
                        {allStatuses.map((s) => <option key={s} value={s}>{s}</option>)}
                      </select>
                      <input
                        type="number"
                        className="flex-1 rounded-md border bg-background px-2 py-1 text-xs"
                        value={tr.after_seconds}
                        onChange={(e) => {
                          const updated = [...editStatusTransitions]
                          updated[idx] = { ...updated[idx], after_seconds: Number(e.target.value) || 30 }
                          setEditStatusTransitions(updated)
                        }}
                        min={1}
                        placeholder="30"
                      />
                      <label className="flex items-center gap-1 text-xs whitespace-nowrap">
                        <input
                          type="checkbox"
                          checked={tr.persist_on_change ?? false}
                          onChange={(e) => {
                            const updated = [...editStatusTransitions]
                            updated[idx] = { ...updated[idx], persist_on_change: e.target.checked }
                            setEditStatusTransitions(updated)
                          }}
                          className="rounded border"
                        />
                        {t('gates.persistOnChange')}
                      </label>
                      <Button variant="ghost" size="icon-sm" type="button" className="text-destructive"
                        onClick={() => setEditStatusTransitions(editStatusTransitions.filter((_, i) => i !== idx))}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
              <div className="flex justify-end gap-2">
                <Button variant="outline" type="button" onClick={() => setConfigModalOpen(false)}>{t('common.cancel')}</Button>
                <Button type="submit" loading={updateConfig.isPending}>{t('common.save')}</Button>
              </div>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      {/* Access codes */}
      <div className="border rounded-lg p-4 mb-4">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-1.5">
            <Hash className="h-4 w-4 opacity-60" />
            <span className="font-semibold">{t('pins.title')}</span>
            <Badge variant="secondary">{pins?.length ?? 0}</Badge>
          </div>
          {canManageGate && (
            <Button size="sm" variant="ghost" onClick={() => { resetPinForm(); setPinModalOpen(true) }}>
              <Plus className="h-3.5 w-3.5" />
              {t('pins.add')}
            </Button>
          )}
        </div>
        {(pins?.length ?? 0) === 0 ? (
          <p className="text-sm text-muted-foreground">{t('pins.noPins')}</p>
        ) : (
          <div className="space-y-0.5">
            {pins?.map((pin) => {
              const codeType = (pin.metadata.code_type as 'pin' | 'password') ?? 'pin'
              const schedule = pin.schedule_id ? scheduleById[pin.schedule_id] : null
              return (
                <div key={pin.id} className="flex items-center justify-between py-1">
                  <div className="flex items-center gap-2">
                    <Hash className="h-3.5 w-3.5 opacity-50" />
                    <span className="text-sm">{pin.label}</span>
                    <Badge variant={codeType === 'pin' ? 'default' : 'secondary'}>
                      {codeType === 'pin' ? 'PIN' : t('pins.passwords')}
                    </Badge>
                    {(pin.metadata as { expires_at?: string }).expires_at && (
                      <div className="flex items-center gap-1">
                        <Clock className="h-3 w-3 opacity-50" />
                        <span className="text-xs text-muted-foreground">
                          {new Date((pin.metadata as { expires_at: string }).expires_at).toLocaleDateString()}
                        </span>
                      </div>
                    )}
                    {schedule && (
                      <SimpleTooltip label={schedule.name}>
                        <Badge variant="warning">
                          <CalendarClock className="h-2.5 w-2.5" />
                          {schedule.name}
                        </Badge>
                      </SimpleTooltip>
                    )}
                  </div>
                  {canManageGate && (
                    <div className="flex items-center gap-1">
                      <Button variant="ghost" size="icon-sm" onClick={() => openEditModal(pin)}>
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button variant="ghost" size="icon-sm" className="text-destructive" onClick={() => deletePin.mutate(pin.id)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Access code create/edit dialog */}
      <Dialog open={pinModalOpen} onOpenChange={(open) => { setPinModalOpen(open); if (!open) resetPinForm() }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{pinModalMode === 'edit' ? t('pins.editCode') : t('pins.add')}</DialogTitle>
          </DialogHeader>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              pinModalMode === 'edit' ? updatePin.mutate() : createPin.mutate()
            }}
          >
            <div className="space-y-6">
              <div>
                <p className="font-semibold mb-2">{t('pins.identification')}</p>
                <div className="space-y-3">
                  <Input
                    label={t('pins.label')}
                    value={pinLabel}
                    onChange={(e) => setPinLabel(e.target.value)}
                    placeholder={t('pins.labelPlaceholder')}
                    required
                  />
                  <SimpleSelect
                    label={t('pins.codeType')}
                    value={pinCodeType}
                    onValueChange={(v) => { setPinCodeType((v as 'pin' | 'password') ?? 'pin'); setPinValue('') }}
                    data={[
                      { value: 'pin', label: t('pins.codeTypePin') },
                      { value: 'password', label: t('pins.codeTypePassword') },
                    ]}
                  />
                  <Alert>
                    <Info className="h-3.5 w-3.5" />
                    <AlertDescription>
                      {pinCodeType === 'pin' ? t('pins.methodWarningPin') : t('pins.methodWarningPassword')}
                    </AlertDescription>
                  </Alert>
                  {pinModalMode === 'create' && (
                    <Input
                      label={t('pins.code')}
                      type="password"
                      value={pinValue}
                      onChange={(e) => {
                        const v = e.target.value
                        setPinValue(pinCodeType === 'pin' ? v.replace(/\D/g, '') : v)
                      }}
                      required
                      minLength={1}
                      inputMode={pinCodeType === 'pin' ? 'numeric' : undefined}
                      className={pinCodeType === 'pin' ? 'font-mono tracking-widest' : ''}
                    />
                  )}
                </div>
              </div>

              <div>
                <p className="font-semibold mb-2">{t('pins.accessRules')}</p>
                <div className="space-y-3">
                  <SimpleSelect
                    label={t('pins.sessionDuration')}
                    description={t('pins.sessionDurationDesc')}
                    value={pinSessionDuration || '__none__'}
                    onValueChange={(v) => setPinSessionDuration(v === '__none__' ? '' : v)}
                    data={PIN_SESSION_PRESETS}
                  />
                  {pinSessionDuration === 'custom' && (
                    <div className="grid grid-cols-2 gap-2">
                      <Input
                        label={t('members.sessionCustomValue')}
                        type="number"
                        value={String(pinCustomValue)}
                        onChange={(e) => setPinCustomValue(Number(e.target.value) || 1)}
                        min={1}
                        step={1}
                      />
                      <SimpleSelect
                        label={t('members.sessionCustomUnit')}
                        value={pinCustomUnit}
                        onValueChange={(v) => setPinCustomUnit(v ?? 'days')}
                        data={[
                          { value: 'minutes', label: t('members.sessionUnitMinutes') },
                          { value: 'hours', label: t('members.sessionUnitHours') },
                          { value: 'days', label: t('members.sessionUnitDays') },
                        ]}
                      />
                    </div>
                  )}
                  <Input
                    label={t('pins.maxUses')}
                    description={t('pins.maxUsesDesc')}
                    type="number"
                    value={String(pinMaxUses)}
                    onChange={(e) => setPinMaxUses(e.target.value === '' ? '' : Number(e.target.value))}
                    min={1}
                    step={1}
                  />
                  <div>
                    <label className="text-sm font-medium">{t('pins.permissions')}</label>
                    <div className="space-y-2 mt-1.5">
                      {[
                        { value: 'gate:trigger_open', label: t('permissions.triggerOpen') },
                        { value: 'gate:trigger_close', label: t('permissions.triggerClose') },
                        { value: 'gate:read_status', label: t('permissions.viewStatus') },
                      ].map((perm) => (
                        <Checkbox
                          key={perm.value}
                          label={perm.label}
                          checked={pinPermissions.includes(perm.value)}
                          onCheckedChange={(checked) => {
                            setPinPermissions((prev) =>
                              checked
                                ? [...prev, perm.value]
                                : prev.filter((p) => p !== perm.value)
                            )
                          }}
                        />
                      ))}
                    </div>
                  </div>
                  <Input
                    label={t('pins.expires')}
                    description={t('common.optional')}
                    type="datetime-local"
                    value={pinExpiresAt}
                    onChange={(e) => setPinExpiresAt(e.target.value)}
                  />
                  <SimpleSelect
                    label={t('pins.schedule')}
                    description={t('pins.scheduleDesc')}
                    value={pinScheduleId || '__none__'}
                    onValueChange={(v) => setPinScheduleId(v === '__none__' ? '' : v)}
                    data={scheduleSelectData}
                  />
                </div>
              </div>

              <div className="flex justify-end gap-2">
                <Button variant="outline" type="button" onClick={() => { setPinModalOpen(false); resetPinForm() }}>
                  {t('common.cancel')}
                </Button>
                <Button type="submit" loading={createPin.isPending || updatePin.isPending}>
                  {pinModalMode === 'edit' ? t('common.save') : t('common.add')}
                </Button>
              </div>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      {/* Custom domains */}
      <div className="border rounded-lg p-4 mb-4">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-1.5">
            <Globe className="h-4 w-4 opacity-60" />
            <span className="font-semibold">{t('domains.title')}</span>
            <Badge variant="secondary">{domains?.length ?? 0}</Badge>
          </div>
          {canManageGate && (
            <Button size="sm" variant="ghost" onClick={() => setDomainModalOpen(true)}>
              <Plus className="h-3.5 w-3.5" />
              {t('domains.add')}
            </Button>
          )}
        </div>

        <Dialog open={domainModalOpen} onOpenChange={setDomainModalOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('domains.add')}</DialogTitle>
            </DialogHeader>
            <form onSubmit={(e) => { e.preventDefault(); addDomain.mutate() }}>
              <div className="space-y-4">
                <Input
                  label={t('domains.domain')}
                  value={domainValue}
                  onChange={(e) => setDomainValue(e.target.value)}
                  required
                  placeholder={t('domains.domainPlaceholder')}
                  className="font-mono"
                />
                <div className="flex justify-end gap-2">
                  <Button variant="outline" type="button" onClick={() => setDomainModalOpen(false)}>{t('common.cancel')}</Button>
                  <Button type="submit" loading={addDomain.isPending}>{t('common.add')}</Button>
                </div>
              </div>
            </form>
          </DialogContent>
        </Dialog>

        {domains?.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('domains.noDomains')}</p>
        ) : (
          <div className="space-y-2">
            {domains?.map((d) => (
              <div key={d.id} className="border rounded-md p-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-1.5">
                    {d.verified_at
                      ? <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                      : <XCircle className="h-4 w-4 text-orange-500" />
                    }
                    <span className="text-sm font-mono">{d.domain}</span>
                  </div>
                  <div className="flex items-center gap-1">
                    {!d.verified_at && canManageGate && (
                      <Button
                        size="sm"
                        variant="outline"
                        className="text-orange-600"
                        loading={verifyDomain.isPending}
                        onClick={() => verifyDomain.mutate(d.id)}
                      >
                        {t('domains.verifyDns')}
                      </Button>
                    )}
                    {canManageGate && (
                      <Button variant="ghost" size="icon-sm" className="text-destructive" onClick={() => deleteDomain.mutate(d.id)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    )}
                  </div>
                </div>

                {!d.verified_at && (
                  <div className="mt-2 bg-muted rounded-md p-2">
                    <p className="text-xs text-muted-foreground mb-1">{t('domains.dnsInstructions')}</p>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 text-[11px] break-all">
                        _gatie.{d.domain} → {d.dns_challenge_token}
                      </code>
                      <SimpleTooltip label={copied ? t('common.copied') : t('common.copy')}>
                        <Button variant="ghost" size="icon-sm" onClick={() => copyText(d.dns_challenge_token, setCopied)}>
                          {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                        </Button>
                      </SimpleTooltip>
                    </div>
                    {verifyResult[d.id] && !verifyResult[d.id].verified && (
                      <p className="text-xs text-destructive mt-1">{verifyResult[d.id].message}</p>
                    )}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
