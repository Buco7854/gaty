import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { publicApi } from '@/api'
import type { GateSession } from '@/api/public'
import { useTranslation } from 'react-i18next'
import { Center, Stack, Group, Text, Title, Button, Anchor, PasswordInput } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'

function sessionKey(gateId: string) {
  return `gaty_session_${gateId}`
}

export default function PasswordAccessPage() {
  const { wsId, gateId } = useParams<{ wsId?: string; gateId: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()

  const [gateName, setGateName] = useState<string | null>(null)
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const portalPath = wsId && gateId
    ? `/workspaces/${wsId}/gates/${gateId}/public`
    : gateId ? `/unlock/${gateId}` : '/'

  useEffect(() => {
    if (!gateId) return
    const raw = localStorage.getItem(sessionKey(gateId))
    if (raw) {
      try {
        const s = JSON.parse(raw) as GateSession
        if (s?.access_token) {
          navigate(portalPath, { replace: true })
          return
        }
      } catch { /* ignore */ }
    }
    publicApi.resolveByGateId(gateId)
      .then((data) => setGateName(data.gate_name))
      .catch(() => {})
  }, [gateId, navigate, portalPath])

  async function submit() {
    if (!gateId || password.length < 1 || submitting) return
    setSubmitting(true)
    try {
      const result = await publicApi.open(gateId, password)
      if (result.session) {
        const session: GateSession = { type: 'pin', ...result.session }
        localStorage.setItem(sessionKey(gateId), JSON.stringify(session))
      }
      navigate(portalPath, { replace: true, state: { justAuthenticated: true } })
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      const msg = status === 429 ? t('pinpad.tooManyAttempts')
        : (status === 401 || status === 403) ? t('pinpad.invalidPin')
        : t('pinpad.unreachable')
      notifications.show({ color: 'red', message: msg, autoClose: 4000 })
      setPassword('')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div style={{ position: 'relative', minHeight: '100vh' }}>
      <Group gap="xs" style={{ position: 'absolute', top: 12, right: 16, zIndex: 10 }}>
        <LangToggle />
        <ThemeToggle />
      </Group>

      <Center mih="100vh" p="md">
        <Stack align="center" gap="xl" w="100%" maw={320}>
          <Stack align="center" gap={4}>
            <Title order={2}>{gateName ?? 'Gate'}</Title>
            <Text size="sm" c="dimmed">{t('pinpad.enterPasswordCode')}</Text>
          </Stack>

          <form onSubmit={(e) => { e.preventDefault(); submit() }} style={{ width: '100%' }}>
            <Stack>
              <PasswordInput
                placeholder={t('pinpad.enterPasswordCode')}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoFocus
                disabled={submitting}
              />
              {password.length > 0 && (
                <Button type="submit" size="md" radius="xl" loading={submitting}>
                  {t('common.confirm')}
                </Button>
              )}
            </Stack>
          </form>

          <Anchor component="button" type="button" size="xs" c="dimmed" onClick={() => navigate(portalPath)}>
            {t('pinpad.useAnotherMethod')}
          </Anchor>
        </Stack>
      </Center>
    </div>
  )
}
