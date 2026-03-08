import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { authApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'
import { TextInput, PasswordInput, Button, Stack, Text, Alert, Title, Anchor } from '@mantine/core'
import { AlertCircle } from 'lucide-react'
import { extractApiError } from '@/lib/notify'

export default function RegisterPage() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const setGlobalSession = useAuthStore((s) => s.setGlobalSession)
  const navigate = useNavigate()
  const { t } = useTranslation()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (password !== confirm) {
      setError(t('auth.passwordMismatch'))
      return
    }
    setError(null)
    setLoading(true)
    try {
      const data = await authApi.register(email, password)
      setGlobalSession(data.user)
      navigate('/workspaces')
    } catch (err: unknown) {
      setError(extractApiError(err, t('auth.registrationFailed')))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Stack gap="lg">
      <Stack gap={4} align="center">
        <Title order={2}>{t('auth.register')}</Title>
        <Text size="sm" c="dimmed">{t('auth.startManaging')}</Text>
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
            minLength={8}
            autoComplete="new-password"
            placeholder={t('auth.passwordPlaceholder')}
          />
          <PasswordInput
            label={t('auth.confirmPassword')}
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            required
            autoComplete="new-password"
          />
          {error && (
            <Alert icon={<AlertCircle size={16} />} color="red" variant="light">
              {error}
            </Alert>
          )}
          <Button type="submit" loading={loading} fullWidth>
            {t('auth.register')}
          </Button>
        </Stack>
      </form>

      <Text size="sm" ta="center" c="dimmed">
        {t('auth.alreadyHaveAccount')}{' '}
        <Anchor component={Link as React.FC} to="/login" size="sm">
          {t('auth.signIn')}
        </Anchor>
      </Text>
    </Stack>
  )
}
