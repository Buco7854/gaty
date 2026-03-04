import { useState } from 'react'
import { useParams } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { workspacesApi } from '@/api'
import type { WorkspaceWithRole } from '@/types'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Stack, Paper, Group, Button, TextInput, PasswordInput, Select, Alert,
} from '@mantine/core'
import { KeyRound, Save, CheckCircle2, AlertCircle } from 'lucide-react'

export default function SettingsPage() {
  const { wsId } = useParams<{ wsId: string }>()
  const qc = useQueryClient()
  const { t } = useTranslation()
  const [saved, setSaved] = useState(false)

  const ws = qc.getQueryData<WorkspaceWithRole[]>(['workspaces'])?.find((w) => w.id === wsId)

  const sso = (ws?.sso_settings ?? {}) as Record<string, string>
  const [issuer, setIssuer] = useState(sso.issuer ?? '')
  const [clientId, setClientId] = useState(sso.client_id ?? '')
  const [clientSecret, setClientSecret] = useState('')
  const [provider, setProvider] = useState(sso.provider ?? 'oidc')

  const updateSSO = useMutation({
    mutationFn: (body: Record<string, string>) =>
      workspacesApi.updateSsoSettings(wsId!, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['workspaces'] })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    },
  })

  function handleSSOSubmit(e: React.FormEvent) {
    e.preventDefault()
    const body: Record<string, string> = { provider, issuer, client_id: clientId }
    if (clientSecret) body.client_secret = clientSecret
    updateSSO.mutate(body)
  }

  return (
    <Container size="sm" py="xl">
      <Stack mb="xl" gap={4}>
        <Title order={2}>{t('settings.title')}</Title>
        <Text c="dimmed" size="sm">{t('settings.subtitle')}</Text>
      </Stack>

      <Paper withBorder p="lg" radius="md">
        <Group gap="xs" mb="md">
          <KeyRound size={18} opacity={0.6} />
          <Text fw={600}>{t('settings.sso')}</Text>
        </Group>

        <form onSubmit={handleSSOSubmit}>
          <Stack>
            <Select
              label={t('settings.provider')}
              value={provider}
              onChange={(v) => setProvider(v ?? 'oidc')}
              data={[{ value: 'oidc', label: 'OIDC' }]}
            />
            <TextInput
              label={t('settings.issuerUrl')}
              value={issuer}
              onChange={(e) => setIssuer(e.target.value)}
              placeholder="https://accounts.google.com"
            />
            <TextInput
              label={t('settings.clientId')}
              value={clientId}
              onChange={(e) => setClientId(e.target.value)}
              placeholder="your-client-id"
            />
            <PasswordInput
              label={t('settings.clientSecret')}
              value={clientSecret}
              onChange={(e) => setClientSecret(e.target.value)}
              placeholder={t('settings.clientSecretPlaceholder')}
            />

            {saved && (
              <Alert icon={<CheckCircle2 size={16} />} color="green" variant="light">
                {t('settings.saved')}
              </Alert>
            )}

            <Group>
              <Button
                type="submit"
                loading={updateSSO.isPending}
                leftSection={<Save size={16} />}
              >
                {t('settings.saveSso')}
              </Button>
            </Group>
          </Stack>
        </form>
      </Paper>
    </Container>
  )
}
