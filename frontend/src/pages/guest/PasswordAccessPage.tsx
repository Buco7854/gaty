import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { publicApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'
import { Center, Stack, Group, Text, Title, Button, Anchor, PasswordInput } from '@mantine/core'
import { notifyError } from '@/lib/notify'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'

export default function PasswordAccessPage() {
  const { gateId } = useParams<{ gateId: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()

  const [gateName, setGateName] = useState<string | null>(null)
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const portalPath = gateId ? `/gates/${gateId}/public` : '/'

  useEffect(() => {
    if (!gateId) return
    // If we already have a pin session, redirect to portal
    const session = useAuthStore.getState().session
    if (session?.type === 'pin_session') {
      navigate(portalPath, { replace: true })
      return
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
      if (result.has_session && result.gate_id && result.permissions) {
        useAuthStore.getState().setPinSession(result.gate_id, result.permissions)
      }
      navigate(portalPath, { replace: true, state: { justAuthenticated: true } })
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      const msg = status === 429 ? t('pinpad.tooManyAttempts')
        : (status === 401 || status === 403) ? t('pinpad.invalidPin')
        : t('pinpad.unreachable')
      notifyError(null, msg)
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
