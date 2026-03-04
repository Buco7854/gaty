import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { authApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'
import { TextInput, PasswordInput, Button, Stack, Text, Alert, Title, Anchor } from '@mantine/core'
import { AlertCircle } from 'lucide-react'

export default function LoginPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const setAuth = useAuthStore((s) => s.setAuth)
  const navigate = useNavigate()
  const { t } = useTranslation()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const data = await authApi.login(email, password)
      setAuth(data.user, data.access_token, data.refresh_token)
      navigate('/workspaces')
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setError(msg ?? t('auth.invalidCredentials'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Stack gap="lg">
      <Stack gap={4} align="center">
        <Title order={2}>{t('auth.signIn')}</Title>
        <Text size="sm" c="dimmed">{t('auth.enterEmail')}</Text>
      </Stack>

      <form onSubmit={handleSubmit}>
        <Stack gap="md">
          <TextInput
            label={t('auth.email')}
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoComplete="email"
            placeholder={t('auth.emailPlaceholder')}
          />
          <PasswordInput
            label={t('auth.password')}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="current-password"
          />
          {error && (
            <Alert icon={<AlertCircle size={16} />} color="red" variant="light">
              {error}
            </Alert>
          )}
          <Button type="submit" loading={loading} fullWidth>
            {t('auth.signIn')}
          </Button>
        </Stack>
      </form>

      <Text size="sm" ta="center" c="dimmed">
        {t('auth.noAccount')}{' '}
        <Anchor component={Link as React.FC} to="/register" size="sm">
          {t('auth.register')}
        </Anchor>
      </Text>
    </Stack>
  )
}
