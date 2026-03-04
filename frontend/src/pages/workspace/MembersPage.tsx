import { useState } from 'react'
import { useParams } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { membersApi } from '@/api'
import type { WorkspaceMembership } from '@/types'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Group, Button, Modal, Stack, Alert, Tabs,
  TextInput, PasswordInput, Select, Badge, Avatar, ActionIcon, Center, Skeleton,
  Collapse, Anchor, NumberInput,
} from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { UserPlus, Trash2, Users, AlertCircle } from 'lucide-react'

const ROLE_COLOR: Record<string, string> = {
  OWNER: 'yellow',
  ADMIN: 'blue',
  MEMBER: 'gray',
}

// session_duration preset values in seconds; '' = workspace default (7d); 'custom' triggers free input
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

export default function MembersPage() {
  const { wsId } = useParams<{ wsId: string }>()
  const qc = useQueryClient()
  const { t } = useTranslation()
  const [opened, { open, close }] = useDisclosure(false)
  const [advancedOpened, setAdvancedOpened] = useState(false)
  const [activeTab, setActiveTab] = useState<string | null>('invite')
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('MEMBER')
  const [sessionDuration, setSessionDuration] = useState<string>('')  // preset value or 'custom'
  const [customValue, setCustomValue] = useState<number | string>(1)
  const [customUnit, setCustomUnit] = useState<string>('days')
  const [error, setError] = useState<string | null>(null)

  const { data: members, isLoading } = useQuery<WorkspaceMembership[]>({
    queryKey: ['members', wsId],
    queryFn: () => membersApi.list(wsId!),
  })

  const invite = useMutation({
    mutationFn: () => membersApi.invite(wsId!, email, role),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['members', wsId] })
      resetAndClose()
    },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setError(msg ?? t('common.error'))
    },
  })

  function resolveSessionDurationSeconds(): number | undefined {
    if (sessionDuration === '') return undefined // use workspace default
    if (sessionDuration === '0') return 0         // infinite
    if (sessionDuration === 'custom') {
      const n = typeof customValue === 'number' ? customValue : parseFloat(String(customValue))
      if (!n || n <= 0) return undefined
      return Math.round(n * (UNIT_MULTIPLIERS[customUnit] ?? 3600))
    }
    return parseInt(sessionDuration, 10)
  }

  const createLocal = useMutation({
    mutationFn: () => {
      const sessionDurationNum = resolveSessionDurationSeconds()
      return membersApi.createLocal(wsId!, {
        local_username: username,
        display_name: displayName || undefined,
        password,
        role,
        auth_config: sessionDurationNum !== undefined
          ? { session_duration: sessionDurationNum }
          : undefined,
      })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['members', wsId] })
      resetAndClose()
    },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setError(msg ?? t('common.error'))
    },
  })

  const deleteMember = useMutation({
    mutationFn: (memberId: string) => membersApi.delete(wsId!, memberId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['members', wsId] }),
  })

  function resetAndClose() {
    close()
    setEmail('')
    setUsername('')
    setDisplayName('')
    setPassword('')
    setRole('MEMBER')
    setSessionDuration('')
    setCustomValue(1)
    setCustomUnit('days')
    setAdvancedOpened(false)
    setError(null)
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (activeTab === 'invite') {
      invite.mutate()
    } else {
      createLocal.mutate()
    }
  }

  const isPending = invite.isPending || createLocal.isPending

  const sessionPresetOptions = SESSION_PRESET_OPTIONS.map(({ value, labelKey }) => ({
    value,
    label: t(labelKey),
  }))

  return (
    <Container size="sm" py="xl">
      <Group justify="space-between" mb="xl">
        <div>
          <Title order={2}>{t('members.title')}</Title>
          <Text c="dimmed" size="sm">{t('members.subtitle')}</Text>
        </div>
        <Button leftSection={<UserPlus size={16} />} onClick={open}>
          {t('members.add')}
        </Button>
      </Group>

      <Modal opened={opened} onClose={resetAndClose} title={t('members.add')}>
        <Tabs value={activeTab} onChange={setActiveTab}>
          <Tabs.List mb="md">
            <Tabs.Tab value="invite">{t('members.inviteByEmail')}</Tabs.Tab>
            <Tabs.Tab value="create">{t('members.createLocal')}</Tabs.Tab>
          </Tabs.List>
        </Tabs>

        <form onSubmit={handleSubmit}>
          <Stack>
            {activeTab === 'invite' ? (
              <TextInput
                label={t('auth.email')}
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                placeholder="user@example.com"
              />
            ) : (
              <>
                <TextInput
                  label={t('members.username')}
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  placeholder={t('members.usernamePlaceholder')}
                />
                <TextInput
                  label={t('members.displayName')}
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder={t('members.displayNamePlaceholder')}
                />
                <PasswordInput
                  label={t('auth.password')}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  minLength={8}
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
                  <Stack gap="xs">
                    <Select
                      label={t('members.sessionDuration')}
                      value={sessionDuration}
                      onChange={(v) => setSessionDuration(v ?? '')}
                      data={sessionPresetOptions}
                    />
                    {sessionDuration === 'custom' && (
                      <Group gap="xs" grow>
                        <NumberInput
                          label={t('members.sessionCustomValue')}
                          value={customValue}
                          onChange={setCustomValue}
                          min={1}
                          step={1}
                        />
                        <Select
                          label={t('members.sessionCustomUnit')}
                          value={customUnit}
                          onChange={(v) => setCustomUnit(v ?? 'days')}
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
            <Select
              label={t('common.role')}
              value={role}
              onChange={(v) => setRole(v ?? 'MEMBER')}
              data={[
                { value: 'MEMBER', label: 'Member' },
                { value: 'ADMIN', label: 'Admin' },
              ]}
            />
            {error && (
              <Alert icon={<AlertCircle size={16} />} color="red" variant="light">
                {error}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={resetAndClose}>{t('common.cancel')}</Button>
              <Button type="submit" loading={isPending}>{t('common.add')}</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {isLoading ? (
        <Stack>
          {[0, 1, 2].map((i) => <Skeleton key={i} height={56} radius="md" />)}
        </Stack>
      ) : members?.length === 0 ? (
        <Center py={80}>
          <Stack align="center" gap="xs">
            <Users size={36} opacity={0.3} />
            <Text size="sm" c="dimmed">{t('members.noMembers')}</Text>
          </Stack>
        </Center>
      ) : (
        <Stack gap={2}>
          {members?.map((m) => (
            <Group key={m.id} justify="space-between" p="sm" style={{ borderRadius: 8 }}>
              <Group gap="sm">
                <Avatar color="indigo" radius="xl" size={32}>
                  {(m.display_name ?? m.local_username ?? '?')[0].toUpperCase()}
                </Avatar>
                <div>
                  <Text size="sm" fw={500}>
                    {m.display_name ?? m.local_username ?? `User ${m.id.slice(0, 8)}`}
                  </Text>
                  {m.local_username && (
                    <Text size="xs" c="dimmed" ff="mono">{m.local_username}</Text>
                  )}
                </div>
              </Group>
              <Group gap="xs">
                <Badge color={ROLE_COLOR[m.role] ?? 'gray'} variant="light" size="sm">
                  {m.role}
                </Badge>
                {m.role !== 'OWNER' && (
                  <ActionIcon
                    variant="subtle"
                    color="red"
                    size="sm"
                    onClick={() => deleteMember.mutate(m.id)}
                  >
                    <Trash2 size={14} />
                  </ActionIcon>
                )}
              </Group>
            </Group>
          ))}
        </Stack>
      )}
    </Container>
  )
}
