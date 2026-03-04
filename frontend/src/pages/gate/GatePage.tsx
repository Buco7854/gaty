import { useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { gatesApi, pinsApi, domainsApi, policiesApi, membersApi } from '@/api'
import type { ActionConfig } from '@/api'
import type { Gate, GatePin, CustomDomain, WorkspaceMembership, MembershipPolicy } from '@/types'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Group, Button, Stack, Paper, Badge, ActionIcon,
  TextInput, PasswordInput, Select, Tooltip, Modal, Code, Alert, Checkbox,
  Table,
} from '@mantine/core'
import { useDisclosure, useClipboard } from '@mantine/hooks'
import {
  ArrowLeft, Zap, Hash, Globe, Plus, Trash2, CheckCircle2, XCircle,
  Clock, Copy, Check, Settings2,
} from 'lucide-react'

const PERMISSIONS = [
  { code: 'gate:read_status', labelKey: 'permissions.viewStatus' },
  { code: 'gate:trigger_open', labelKey: 'permissions.triggerOpen' },
  { code: 'gate:manage', labelKey: 'permissions.manage' },
] as const

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

  // Modal state
  const [pinModalOpened, { open: openPinModal, close: closePinModal }] = useDisclosure(false)
  const [domainModalOpened, { open: openDomainModal, close: closeDomainModal }] = useDisclosure(false)
  const [configModalOpened, { open: openConfigModal, close: closeConfigModal }] = useDisclosure(false)

  // PIN form
  const [pinLabel, setPinLabel] = useState('')
  const [pinValue, setPinValue] = useState('')

  // Domain form
  const [domainValue, setDomainValue] = useState('')
  const [verifyResult, setVerifyResult] = useState<Record<string, { verified: boolean; message?: string }>>({})

  // Config form
  const [editOpenConfig, setEditOpenConfig] = useState<ActionConfig | null>(null)
  const [editCloseConfig, setEditCloseConfig] = useState<ActionConfig | null>(null)
  const [editStatusConfig, setEditStatusConfig] = useState<ActionConfig | null>(null)

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
  })

  const { data: members } = useQuery<WorkspaceMembership[]>({
    queryKey: ['members', wsId],
    queryFn: () => membersApi.list(wsId!),
  })

  const { data: policies } = useQuery<MembershipPolicy[]>({
    queryKey: ['policies', wsId, gateId],
    queryFn: () => policiesApi.list(wsId!, gateId!),
  })

  const trigger = useMutation({
    mutationFn: () => gatesApi.trigger(wsId!, gateId!),
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
    mutationFn: () => pinsApi.create(wsId!, gateId!, { label: pinLabel || undefined, pin: pinValue }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pins', wsId, gateId] })
      closePinModal()
      setPinLabel('')
      setPinValue('')
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

  const grantPerm = useMutation({
    mutationFn: ({ membershipId, permCode }: { membershipId: string; permCode: string }) =>
      policiesApi.grant(wsId!, gateId!, membershipId, permCode),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['policies', wsId, gateId] }),
  })

  const revokePerm = useMutation({
    mutationFn: ({ membershipId, permCode }: { membershipId: string; permCode: string }) =>
      policiesApi.revoke(wsId!, gateId!, membershipId, permCode),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['policies', wsId, gateId] }),
  })

  const regularMembers = members?.filter((m) => m.role === 'MEMBER') ?? []

  function hasPermission(membershipId: string, permCode: string) {
    return policies?.some((p) => p.membership_id === membershipId && p.permission_code === permCode) ?? false
  }

  function togglePermission(membershipId: string, permCode: string) {
    if (hasPermission(membershipId, permCode)) {
      revokePerm.mutate({ membershipId, permCode })
    } else {
      grantPerm.mutate({ membershipId, permCode })
    }
  }

  function openConfig() {
    setEditOpenConfig(gate?.open_config ?? null)
    setEditCloseConfig(gate?.close_config ?? null)
    setEditStatusConfig(gate?.status_config ?? null)
    openConfigModal()
  }

  const statusColor = gate?.status === 'online' ? 'green' : gate?.status === 'offline' ? 'red' : 'gray'

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
          <Tooltip label={t('gates.integration')}>
            <ActionIcon variant="default" size="lg" onClick={openConfig}>
              <Settings2 size={16} />
            </ActionIcon>
          </Tooltip>
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

      {/* PIN codes */}
      <Paper withBorder p="md" radius="md" mb="md">
        <Group justify="space-between" mb="sm">
          <Group gap="xs">
            <Hash size={16} opacity={0.6} />
            <Text fw={600}>{t('pins.title')}</Text>
            <Badge variant="light" size="xs">{pins?.length ?? 0}</Badge>
          </Group>
          <Button size="xs" variant="subtle" leftSection={<Plus size={14} />} onClick={openPinModal}>
            {t('pins.add')}
          </Button>
        </Group>

        <Modal opened={pinModalOpened} onClose={closePinModal} title={t('pins.add')}>
          <form onSubmit={(e) => { e.preventDefault(); createPin.mutate() }}>
            <Stack>
              <TextInput
                label={t('pins.label')}
                value={pinLabel}
                onChange={(e) => setPinLabel(e.target.value)}
                placeholder={t('pins.labelPlaceholder')}
              />
              <PasswordInput
                label={t('pins.pin')}
                value={pinValue}
                onChange={(e) => setPinValue(e.target.value)}
                required
                minLength={4}
                styles={{ input: { fontFamily: 'monospace' } }}
              />
              <Group justify="flex-end">
                <Button variant="default" onClick={closePinModal}>{t('common.cancel')}</Button>
                <Button type="submit" loading={createPin.isPending}>{t('common.add')}</Button>
              </Group>
            </Stack>
          </form>
        </Modal>

        {pins?.length === 0 ? (
          <Text size="sm" c="dimmed">{t('pins.noPins')}</Text>
        ) : (
          <Stack gap={2}>
            {pins?.map((pin) => (
              <Group key={pin.id} justify="space-between" py={4}>
                <Group gap="sm">
                  <Hash size={14} opacity={0.5} />
                  <Text size="sm">
                    {pin.label ?? <Text size="sm" c="dimmed" fs="italic" component="span">{t('pins.unlabeled')}</Text>}
                  </Text>
                  {(pin.metadata as { expires_at?: string }).expires_at && (
                    <Group gap={4}>
                      <Clock size={12} opacity={0.5} />
                      <Text size="xs" c="dimmed">
                        {new Date((pin.metadata as { expires_at: string }).expires_at).toLocaleDateString()}
                      </Text>
                    </Group>
                  )}
                </Group>
                <ActionIcon variant="subtle" color="red" size="sm" onClick={() => deletePin.mutate(pin.id)}>
                  <Trash2 size={14} />
                </ActionIcon>
              </Group>
            ))}
          </Stack>
        )}
      </Paper>

      {/* Custom domains */}
      <Paper withBorder p="md" radius="md" mb="md">
        <Group justify="space-between" mb="sm">
          <Group gap="xs">
            <Globe size={16} opacity={0.6} />
            <Text fw={600}>{t('domains.title')}</Text>
            <Badge variant="light" size="xs">{domains?.length ?? 0}</Badge>
          </Group>
          <Button size="xs" variant="subtle" leftSection={<Plus size={14} />} onClick={openDomainModal}>
            {t('domains.add')}
          </Button>
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
                    {!d.verified_at && (
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
                    <ActionIcon variant="subtle" color="red" size="sm" onClick={() => deleteDomain.mutate(d.id)}>
                      <Trash2 size={14} />
                    </ActionIcon>
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

      {/* Member permissions */}
      <Paper withBorder p="md" radius="md">
        <Group gap="xs" mb="sm">
          <Text fw={600}>{t('permissions.title')}</Text>
          <Badge variant="light" size="xs">{regularMembers.length}</Badge>
        </Group>

        {regularMembers.length === 0 ? (
          <Text size="sm" c="dimmed">No regular members in this workspace</Text>
        ) : (
          <Table>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Member</Table.Th>
                {PERMISSIONS.map((p) => (
                  <Table.Th key={p.code} ta="center">{t(p.labelKey)}</Table.Th>
                ))}
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {regularMembers.map((m) => (
                <Table.Tr key={m.id}>
                  <Table.Td>
                    <Text size="sm" truncate maw={160}>
                      {m.display_name ?? m.local_username ?? `Member ${m.id.slice(0, 8)}`}
                    </Text>
                  </Table.Td>
                  {PERMISSIONS.map((p) => {
                    const checked = hasPermission(m.id, p.code)
                    return (
                      <Table.Td key={p.code} ta="center">
                        <Checkbox
                          checked={checked}
                          onChange={() => togglePermission(m.id, p.code)}
                          disabled={grantPerm.isPending || revokePerm.isPending}
                        />
                      </Table.Td>
                    )
                  })}
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Paper>
    </Container>
  )
}
