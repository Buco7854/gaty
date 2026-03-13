import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { gatesApi, publicApi } from '@/api'
import type { Gate } from '@/types'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Stack, Paper, Group, Button, NumberInput, Loader, Center,
  Switch, Alert, Badge,
} from '@mantine/core'
import { GatePermissionsGrid, useGatePermissions } from '@/components/GatePermissionsGrid'
import { KeyRound, Save, CheckCircle2, Info } from 'lucide-react'
import { QueryError } from '@/components/QueryError'
import { api } from '@/lib/api'

export default function SettingsPage() {
  const qc = useQueryClient()
  const { t } = useTranslation()
  const gatePermissions = useGatePermissions()
  const [macSaved, setMacSaved] = useState(false)

  // SSO providers — read-only (configured via environment)
  const { data: ssoProviders = [], isLoading: ssoLoading } = useQuery({
    queryKey: ['sso-providers'],
    queryFn: () => publicApi.ssoProviders().catch(() => []),
  })

  // Member auth config
  const { data: macData, isLoading: macLoading, isError: macError, error: macFetchError } = useQuery({
    queryKey: ['member-auth-config'],
    queryFn: () => api.get<Record<string, unknown>>('/auth/sso/settings').then((r) => r.data),
  })

  const { data: gatesData } = useQuery({
    queryKey: ['gates'],
    queryFn: () => gatesApi.list(),
  })
  const gates = (gatesData ?? []) as Gate[]

  const [passwordAuth, setPasswordAuth] = useState<boolean>(true)
  const [ssoAuth, setSsoAuth] = useState<boolean>(false)
  const [apiTokenAuth, setApiTokenAuth] = useState<boolean>(false)
  const [apiTokenMax, setApiTokenMax] = useState<number>(5)
  const [sessionDuration, setSessionDuration] = useState<number | string>('')
  const [defaultGatePerms, setDefaultGatePerms] = useState<Record<string, Set<string>>>({})

  // Sync MAC state once data is loaded
  const [macInitialized, setMacInitialized] = useState(false)
  if (macData && !macInitialized) {
    setPasswordAuth((macData.password as boolean) ?? true)
    setSsoAuth((macData.sso as boolean) ?? false)
    setApiTokenAuth((macData.api_token as boolean) ?? false)
    setApiTokenMax((macData.api_token_max as number) ?? 5)
    if (macData.session_duration != null) setSessionDuration(macData.session_duration as number)
    const dgp = macData.default_gate_permissions
    if (Array.isArray(dgp)) {
      const map: Record<string, Set<string>> = {}
      for (const entry of dgp as { gate_id: string; permissions: string[] }[]) {
        if (entry.gate_id) map[entry.gate_id] = new Set(entry.permissions ?? [])
      }
      setDefaultGatePerms(map)
    }
    setMacInitialized(true)
  }

  const updateMAC = useMutation({
    mutationFn: (body: Record<string, unknown>) => api.put('/auth/sso/settings', body).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['member-auth-config'] })
      setMacSaved(true)
      setTimeout(() => setMacSaved(false), 2000)
    },
  })

  function handleSaveMemberAuth() {
    const dur = typeof sessionDuration === 'number' ? sessionDuration : (sessionDuration === '' ? null : parseInt(String(sessionDuration), 10))
    const defaultGatePermsList = Object.entries(defaultGatePerms)
      .filter(([, perms]) => perms.size > 0)
      .map(([gate_id, perms]) => ({ gate_id, permissions: Array.from(perms) }))
    updateMAC.mutate({
      password: passwordAuth,
      sso: ssoAuth,
      api_token: apiTokenAuth,
      api_token_max: apiTokenMax,
      session_duration: dur,
      default_gate_permissions: defaultGatePermsList,
    })
  }

  if (macLoading || ssoLoading) {
    return (
      <Center py="xl">
        <Loader />
      </Center>
    )
  }

  if (macError) {
    return (
      <Container size="sm" py="xl">
        <QueryError error={macFetchError} />
      </Container>
    )
  }

  return (
    <Container size="sm" py="xl">
      <Stack mb="xl" gap={4}>
        <Title order={2}>{t('settings.title')}</Title>
        <Text c="dimmed" size="sm">{t('settings.subtitle')}</Text>
      </Stack>

      {/* SSO Providers — read-only */}
      <Paper withBorder p="lg" radius="md" mb="md">
        <Group justify="space-between" mb="md">
          <Group gap="xs">
            <KeyRound size={18} opacity={0.6} />
            <Text fw={600}>{t('settings.sso')}</Text>
          </Group>
        </Group>

        <Alert icon={<Info size={16} />} color="blue" variant="light" mb="md">
          <Text size="sm">{t('settings.ssoEnvConfigured')}</Text>
        </Alert>

        {ssoProviders.length === 0 ? (
          <Text size="sm" c="dimmed">{t('settings.noProviders')}</Text>
        ) : (
          <Stack gap="xs">
            {ssoProviders.map((p) => (
              <Paper key={p.id} withBorder p="sm" radius="sm">
                <Group gap="sm">
                  <Text size="sm" fw={500}>{p.name}</Text>
                  <Badge size="xs" variant="light">{p.type.toUpperCase()}</Badge>
                </Group>
              </Paper>
            ))}
          </Stack>
        )}
      </Paper>

      {/* Member auth defaults */}
      <Paper withBorder p="lg" radius="md" mb="md">
        <Text fw={600} mb="xs">{t('settings.memberAuthDefaults')}</Text>
        <Text size="xs" c="dimmed" mb="lg">{t('settings.memberAuthDefaultsHint')}</Text>

        <Stack gap="lg">
          <Switch
            label={t('settings.passwordAuth')}
            checked={passwordAuth}
            onChange={(e) => setPasswordAuth(e.currentTarget.checked)}
          />
          <Switch
            label={t('settings.ssoAuth')}
            checked={ssoAuth}
            onChange={(e) => setSsoAuth(e.currentTarget.checked)}
          />
          <Switch
            label={t('settings.apiTokenAuth')}
            checked={apiTokenAuth}
            onChange={(e) => setApiTokenAuth(e.currentTarget.checked)}
          />
          {apiTokenAuth && (
            <NumberInput
              label={t('settings.apiTokenMax')}
              value={apiTokenMax}
              onChange={(v) => setApiTokenMax(Number(v) || 5)}
              min={1}
              max={100}
              w={120}
            />
          )}
          <NumberInput
            label={t('settings.memberSessionDuration')}
            description={t('settings.memberSessionDurationHint')}
            value={sessionDuration}
            onChange={setSessionDuration}
            min={0}
            step={3600}
            placeholder={t('settings.memberSessionDurationPlaceholder')}
            w={200}
          />
        </Stack>
      </Paper>

      {/* Default member permissions */}
      <Paper withBorder p="lg" radius="md">
        <Text fw={600} mb="xs">{t('settings.defaultMemberPermissions')}</Text>
        <Text size="xs" c="dimmed" mb="md">{t('settings.defaultMemberPermissionsHint')}</Text>

        {gates.length === 0 ? (
          <Text size="xs" c="dimmed" fs="italic">{t('settings.noGatesForDefaults')}</Text>
        ) : (
          <GatePermissionsGrid
            gates={gates}
            permissions={gatePermissions}
            isChecked={(gateId, code) => (defaultGatePerms[gateId] ?? new Set()).has(code)}
            onToggle={(gateId, code) => {
              setDefaultGatePerms((prev) => {
                const next = { ...prev }
                const set = new Set(next[gateId] ?? [])
                if (set.has(code)) set.delete(code)
                else set.add(code)
                next[gateId] = set
                return next
              })
            }}
            withColumnSelect
          />
        )}

        {macSaved && (
          <Alert icon={<CheckCircle2 size={16} />} color="green" variant="light" mt="md">
            {t('settings.saved')}
          </Alert>
        )}

        <Group mt="md">
          <Button
            onClick={handleSaveMemberAuth}
            loading={updateMAC.isPending}
            leftSection={<Save size={16} />}
          >
            {t('settings.saveSso')}
          </Button>
        </Group>
      </Paper>
    </Container>
  )
}
