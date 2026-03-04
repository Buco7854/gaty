import { useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { gatesApi } from '@/api'
import type { ActionConfig } from '@/api'
import type { Gate, GateStatus, WorkspaceWithRole } from '@/types'
import { useGateEvents } from '@/hooks/useGateEvents'
import type { GateEvent } from '@/hooks/useGateEvents'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Group, Button, Modal, TextInput, Stack, Badge,
  SimpleGrid, Card, ActionIcon, Select, Center, Tooltip, Loader,
} from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { Plus, DoorOpen, Zap, ChevronRight } from 'lucide-react'

function StatusBadge({ status }: { status: Gate['status'] }) {
  const { t } = useTranslation()
  const color = status === 'online' ? 'green' : status === 'offline' ? 'red' : 'gray'
  return <Badge color={color} variant="dot" size="sm">{t(`common.${status}`)}</Badge>
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

export default function WorkspacePage() {
  const { wsId } = useParams<{ wsId: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { t } = useTranslation()
  const [opened, { open, close }] = useDisclosure(false)
  const [gateName, setGateName] = useState('')
  const [openConfig, setOpenConfig] = useState<ActionConfig | null>({ type: 'MQTT' })
  const [closeConfig, setCloseConfig] = useState<ActionConfig | null>(null)
  const [statusConfig, setStatusConfig] = useState<ActionConfig | null>(null)
  const [triggeringId, setTriggeringId] = useState<string | null>(null)

  const ws = qc.getQueryData<WorkspaceWithRole[]>(['workspaces'])?.find((w) => w.id === wsId)

  const { data: gates, isLoading } = useQuery<Gate[]>({
    queryKey: ['gates', wsId],
    queryFn: () => gatesApi.list(wsId!),
    refetchInterval: 10_000,
  })

  const handleGateEvent = useCallback(
    (event: GateEvent) => {
      qc.setQueryData<Gate[]>(['gates', wsId], (prev) =>
        prev?.map((g) =>
          g.id === event.gate_id ? { ...g, status: event.status as GateStatus } : g
        )
      )
    },
    [qc, wsId]
  )

  useGateEvents(wsId, handleGateEvent)

  const createGate = useMutation({
    mutationFn: () =>
      gatesApi.create(wsId!, {
        name: gateName,
        open_config: openConfig,
        close_config: closeConfig,
        status_config: statusConfig,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['gates', wsId] })
      close()
      setGateName('')
      setOpenConfig({ type: 'MQTT' })
      setCloseConfig(null)
      setStatusConfig(null)
    },
  })

  const triggerGate = useMutation({
    mutationFn: (gateId: string) => gatesApi.trigger(wsId!, gateId),
    onMutate: (gateId) => setTriggeringId(gateId),
    onSettled: () => setTriggeringId(null),
  })

  return (
    <Container size="lg" py="xl">
      <Group justify="space-between" mb="xl">
        <div>
          <Title order={2}>{ws?.name ?? t('gates.title')}</Title>
          <Text c="dimmed" size="sm">{t('gates.subtitle')}</Text>
        </div>
        <Button leftSection={<Plus size={16} />} onClick={open}>
          {t('gates.add')}
        </Button>
      </Group>

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
            <Group justify="flex-end">
              <Button variant="default" onClick={close}>{t('common.cancel')}</Button>
              <Button type="submit" loading={createGate.isPending}>{t('common.add')}</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {isLoading ? (
        <Center py={80}><Loader /></Center>
      ) : gates?.length === 0 ? (
        <Center py={80}>
          <Stack align="center" gap="xs">
            <DoorOpen size={40} opacity={0.3} />
            <Text fw={500}>{t('gates.noGates')}</Text>
            <Text size="sm" c="dimmed">{t('gates.noGatesHint')}</Text>
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
              <Text size="xs" c="dimmed" mb="md">
                {gate.open_config?.type ?? gate.integration_type}
              </Text>
              <Group gap="xs">
                <Button
                  size="xs"
                  leftSection={<Zap size={12} />}
                  loading={triggeringId === gate.id}
                  onClick={() => triggerGate.mutate(gate.id)}
                  style={{ flex: 1 }}
                >
                  {t('gates.open')}
                </Button>
                <Tooltip label={t('common.details')}>
                  <ActionIcon
                    variant="default"
                    size="sm"
                    onClick={() => navigate(`/workspaces/${wsId}/gates/${gate.id}`)}
                  >
                    <ChevronRight size={14} />
                  </ActionIcon>
                </Tooltip>
              </Group>
            </Card>
          ))}
        </SimpleGrid>
      )}
    </Container>
  )
}
