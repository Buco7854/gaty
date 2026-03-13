import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { gatesApi, policiesApi } from '@/api'
import type { ActionConfig } from '@/api'
import type { Gate, GateStatus } from '@/types'
import { useGateEvents } from '@/hooks/useGateEvents'
import type { GateEvent } from '@/hooks/useGateEvents'
import { useTranslation } from 'react-i18next'
import { Plus, DoorOpen, Zap, ChevronRight, Loader2 } from 'lucide-react'
import { notifySuccess, notifyError } from '@/lib/notify'
import { QueryError } from '@/components/QueryError'
import { useAuthStore } from '@/store/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { SimpleSelect } from '@/components/ui/select'
import { SimpleTooltip } from '@/components/ui/tooltip'


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

function StatusBadge({ status }: { status: Gate['status'] }) {
  const { t } = useTranslation()
  return (
    <Badge variant={getStatusVariant(status)}>
      {t(`common.${status}`, { defaultValue: status })}
    </Badge>
  )
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
        <Textarea
          label={t('gates.mqttCustomPayload')}
          description={t('gates.mqttCustomPayloadDesc')}
          defaultValue={JSON.stringify(value?.config?.payload ?? {}, null, 2)}
          onBlur={(e) => {
            try {
              const parsed = JSON.parse(e.target.value)
              onChange({ type: 'MQTT_CUSTOM', config: { ...value?.config, payload: parsed } })
            } catch { /* ignore invalid JSON */ }
          }}
          placeholder={'{\n  "cmd": 1\n}'}
          rows={3}
          className="font-mono text-xs"
        />
      )}
      {driverType === 'HTTP' && (
        <Input
          label={t('gates.httpUrl')}
          value={(value?.config?.url as string) ?? ''}
          onChange={(e) =>
            onChange({ type: 'HTTP', config: { ...value?.config, url: e.target.value } })
          }
          placeholder="https://api.example.com/open"
        />
      )}
    </div>
  )
}

export default function DashboardPage() {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { t } = useTranslation()
  const session = useAuthStore((s) => s.session)
  const member = session?.type === 'member' ? session.member : null
  const isAdmin = member?.role === 'ADMIN'

  const [opened, setOpened] = useState(false)
  const [advancedOpened, setAdvancedOpened] = useState(false)
  const [gateName, setGateName] = useState('')
  const [openConfig, setOpenConfig] = useState<ActionConfig | null>({ type: 'MQTT_GATIE' })
  const [closeConfig, setCloseConfig] = useState<ActionConfig | null>(null)
  const [statusConfig, setStatusConfig] = useState<ActionConfig | null>(null)
  const [triggeringId, setTriggeringId] = useState<string | null>(null)

  const { data: gates, isLoading, isError, error } = useQuery<Gate[]>({
    queryKey: ['gates'],
    queryFn: () => gatesApi.list(),
    refetchInterval: 15_000,
    enabled: !!member,
  })

  const { data: myPolicies } = useQuery({
    queryKey: ['policies-me'],
    queryFn: () => policiesApi.listMine(),
    enabled: !isAdmin && !!member,
  })

  const canManageGate = (gateId: string) =>
    isAdmin || myPolicies?.some((p) => p.gate_id === gateId && p.permission_code === 'gate:manage')

  // SSE: update gate status + metadata in real-time
  const handleGateEvent = useCallback(
    (event: GateEvent) => {
      const patch = { status: event.status as GateStatus, status_metadata: event.status_metadata }
      qc.setQueryData<Gate[]>(['gates'], (prev) =>
        prev?.map((g) =>
          g.id === event.gate_id
            ? { ...g, ...patch, status_metadata: patch.status_metadata ?? g.status_metadata }
            : g
        )
      )
      qc.setQueryData<Gate>(['gate', event.gate_id], (prev) =>
        prev ? { ...prev, ...patch, status_metadata: patch.status_metadata ?? prev.status_metadata } : prev
      )
    },
    [qc]
  )
  useGateEvents(handleGateEvent)

  const createGate = useMutation({
    mutationFn: () =>
      gatesApi.create({
        name: gateName,
        open_config: openConfig,
        close_config: closeConfig,
        status_config: statusConfig,
      }),
    onSuccess: (gate) => {
      qc.invalidateQueries({ queryKey: ['gates'] })
      setOpened(false)
      setGateName('')
      setOpenConfig({ type: 'MQTT_GATIE' })
      setCloseConfig(null)
      setStatusConfig(null)
      setAdvancedOpened(false)
      if (gate.gate_token) {
        notifySuccess(`${t('common.created')} — token: ${gate.gate_token.slice(0, 8)}…`)
      } else {
        notifySuccess(t('common.created'))
      }
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  async function triggerGate(gateId: string, action: 'open' | 'close' = 'open') {
    if (triggeringId) return
    setTriggeringId(gateId)
    const optimisticStatus = (action === 'open' ? 'open' : 'closed') as GateStatus
    qc.setQueryData<Gate[]>(['gates'], (prev) =>
      prev?.map((g) => g.id === gateId ? { ...g, status: optimisticStatus } : g)
    )
    qc.setQueryData<Gate>(['gate', gateId], (prev) =>
      prev ? { ...prev, status: optimisticStatus } : prev
    )
    try {
      await gatesApi.trigger(gateId, action)
    } catch { /* fire-and-forget */ }
    setTriggeringId(null)
  }

  return (
    <div className="max-w-5xl mx-auto py-8 px-4">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h2 className="text-xl font-bold">{t('gates.title')}</h2>
          <p className="text-sm text-muted-foreground">{t('gates.subtitle')}</p>
        </div>
        {isAdmin && (
          <Button onClick={() => setOpened(true)}>
            <Plus size={16} />
            {t('gates.add')}
          </Button>
        )}
      </div>

      {isAdmin && (
        <Dialog open={opened} onOpenChange={setOpened}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('gates.add')}</DialogTitle>
            </DialogHeader>
            <form onSubmit={(e) => { e.preventDefault(); createGate.mutate() }}>
              <div className="space-y-4">
                <Input
                  label={t('common.name')}
                  value={gateName}
                  onChange={(e) => setGateName(e.target.value)}
                  required
                  placeholder="Parking entrance"
                />
                <button
                  type="button"
                  className="text-xs text-muted-foreground hover:underline cursor-pointer"
                  onClick={() => setAdvancedOpened((o) => !o)}
                >
                  {t('gates.advancedOptions')} {advancedOpened ? '\u25B2' : '\u25BC'}
                </button>
                {advancedOpened && (
                  <div className="space-y-3">
                    <ActionConfigForm
                      label={t('gates.openAction')}
                      value={openConfig}
                      onChange={setOpenConfig}
                    />
                    <ActionConfigForm
                      label={t('gates.closeAction')}
                      value={closeConfig}
                      onChange={setCloseConfig}
                    />
                    <ActionConfigForm
                      label={t('gates.statusAction')}
                      value={statusConfig}
                      onChange={setStatusConfig}
                    />
                  </div>
                )}
                <div className="flex items-center justify-end gap-2">
                  <Button variant="outline" type="button" onClick={() => setOpened(false)}>{t('common.cancel')}</Button>
                  <Button type="submit" loading={createGate.isPending}>{t('common.add')}</Button>
                </div>
              </div>
            </form>
          </DialogContent>
        </Dialog>
      )}

      {isLoading ? (
        <div className="flex justify-center py-20">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : isError ? (
        <QueryError error={error} />
      ) : gates?.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 gap-2">
          <DoorOpen size={40} className="opacity-30" />
          <p className="font-medium">{t('gates.noGates')}</p>
          {isAdmin && <p className="text-sm text-muted-foreground">{t('gates.noGatesHint')}</p>}
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {gates?.map((gate) => (
            <div key={gate.id} className="border rounded-lg p-4">
              <div className="flex items-center justify-between gap-2 mb-2">
                <p className="font-semibold truncate flex-1">{gate.name}</p>
                <StatusBadge status={gate.status} />
              </div>
              {isAdmin && (
                <p className="text-xs text-muted-foreground mb-4">
                  {(() => {
                    const types = [gate.open_config, gate.close_config, gate.status_config]
                      .map(c => c?.type)
                      .filter((t): t is NonNullable<typeof t> => !!t && t !== 'NONE');
                    const unique = [...new Set(types)];
                    return unique.length > 0 ? unique.join(' / ') : t('gates.noDriver');
                  })()}
                </p>
              )}
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  loading={triggeringId === gate.id}
                  onClick={() => triggerGate(gate.id)}
                  className="flex-1"
                >
                  <Zap size={12} />
                  {t('gates.open')}
                </Button>
                {canManageGate(gate.id) && (
                  <SimpleTooltip label={t('common.details')}>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      onClick={() => navigate(`/gates/${gate.id}`)}
                    >
                      <ChevronRight size={14} />
                    </Button>
                  </SimpleTooltip>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
