import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { gatesApi, policiesApi } from '@/api'
import type { ActionConfig } from '@/api'
import type { Gate, GateStatus } from '@/types'
import { useGateEvents } from '@/hooks/useGateEvents'
import type { GateEvent } from '@/hooks/useGateEvents'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Group, Button, Modal, TextInput, Textarea, Stack, Badge,
  SimpleGrid, Card, ActionIcon, Select, Center, Tooltip, Loader, Collapse, Anchor,
} from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { Plus, DoorOpen, Zap, ChevronRight } from 'lucide-react'
import { notifySuccess, notifyError } from '@/lib/notify'
import { QueryError } from '@/components/QueryError'
import { useAuthStore } from '@/store/auth'


function getStatusColor(status: GateStatus | undefined): string {
  switch (status) {
    case 'online':
    case 'open': return 'green'
    case 'offline':
    case 'closed': return 'red'
    case 'unresponsive':
    case 'unavailable': return 'orange'
    default: return 'gray'
  }
}

function StatusBadge({ status }: { status: Gate['status'] }) {
  const { t } = useTranslation()
  return (
    <Badge color={getStatusColor(status)} variant="dot" size="sm">
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
          minRows={3}
          styles={{ input: { fontFamily: 'monospace', fontSize: 12 } }}
        />
      )}
      {driverType === 'HTTP' && (
        <TextInput
          label={t('gates.httpUrl')}
          value={(value?.config?.url as string) ?? ''}
          onChange={(e) =>
            onChange({ type: 'HTTP', config: { ...value?.config, url: e.target.value } })
          }
          placeholder="https://api.example.com/open"
        />
      )}
    </Stack>
  )
}

export default function DashboardPage() {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { t } = useTranslation()
  const session = useAuthStore((s) => s.session)
  const member = session?.type === 'member' ? session.member : null
  const isAdmin = member?.role === 'ADMIN'

  const [opened, { open, close }] = useDisclosure(false)
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
      close()
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
    <Container size="lg" py="xl">
      <Group justify="space-between" mb="xl">
        <div>
          <Title order={2}>{t('gates.title')}</Title>
          <Text c="dimmed" size="sm">{t('gates.subtitle')}</Text>
        </div>
        {isAdmin && (
          <Button leftSection={<Plus size={16} />} onClick={open}>
            {t('gates.add')}
          </Button>
        )}
      </Group>

      {isAdmin && (
        <Modal opened={opened} onClose={close} title={t('gates.add')} size="md">
          <form onSubmit={(e) => { e.preventDefault(); createGate.mutate() }}>
            <Stack>
              <TextInput
                label={t('common.name')}
                value={gateName}
                onChange={(e) => setGateName(e.target.value)}
                required
                placeholder="Parking entrance"
              />
              <Anchor
                component="button"
                type="button"
                size="xs"
                c="dimmed"
                onClick={() => setAdvancedOpened((o) => !o)}
              >
                {t('gates.advancedOptions')} {advancedOpened ? '▲' : '▼'}
              </Anchor>
              <Collapse in={advancedOpened}>
                <Stack gap="sm">
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
                </Stack>
              </Collapse>
              <Group justify="flex-end">
                <Button variant="default" onClick={close}>{t('common.cancel')}</Button>
                <Button type="submit" loading={createGate.isPending}>{t('common.add')}</Button>
              </Group>
            </Stack>
          </form>
        </Modal>
      )}

      {isLoading ? (
        <Center py={80}><Loader /></Center>
      ) : isError ? (
        <QueryError error={error} />
      ) : gates?.length === 0 ? (
        <Center py={80}>
          <Stack align="center" gap="xs">
            <DoorOpen size={40} opacity={0.3} />
            <Text fw={500}>{t('gates.noGates')}</Text>
            {isAdmin && <Text size="sm" c="dimmed">{t('gates.noGatesHint')}</Text>}
          </Stack>
        </Center>
      ) : (
        <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }} spacing="md">
          {gates?.map((gate) => (
            <Card key={gate.id} withBorder radius="md" p="md">
              <Group justify="space-between" mb="xs" wrap="nowrap">
                <Text fw={600} truncate style={{ flex: 1 }}>{gate.name}</Text>
                <StatusBadge status={gate.status} />
              </Group>
              {isAdmin && (
                <Text size="xs" c="dimmed" mb="md">
                  {(() => {
                    const types = [gate.open_config, gate.close_config, gate.status_config]
                      .map(c => c?.type)
                      .filter((t): t is string => !!t && t !== 'NONE');
                    const unique = [...new Set(types)];
                    return unique.length > 0 ? unique.join(' / ') : t('gates.noDriver');
                  })()}
                </Text>
              )}
              <Group gap="xs">
                <Button
                  size="xs"
                  leftSection={<Zap size={12} />}
                  loading={triggeringId === gate.id}
                  onClick={() => triggerGate(gate.id)}
                  style={{ flex: 1 }}
                >
                  {t('gates.open')}
                </Button>
                {canManageGate(gate.id) && (
                  <Tooltip label={t('common.details')}>
                    <ActionIcon
                      variant="default"
                      size="sm"
                      onClick={() => navigate(`/gates/${gate.id}`)}
                    >
                      <ChevronRight size={14} />
                    </ActionIcon>
                  </Tooltip>
                )}
              </Group>
            </Card>
          ))}
        </SimpleGrid>
      )}
    </Container>
  )
}
