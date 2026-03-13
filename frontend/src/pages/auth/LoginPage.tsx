import { useState } from 'react'
import { useNavigate } from 'react-router'
import { authApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'
import { TextInput, PasswordInput, Button, Stack, Text, Alert, Title } from '@mantine/core'
import { AlertCircle } from 'lucide-react'
import { extractApiError } from '@/lib/notify'

export default function LoginPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const setMemberSession = useAuthStore((s) => s.setMemberSession)
  const navigate = useNavigate()
  const { t } = useTranslation()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const data = await authApi.login(username, password)
      setMemberSession(data.member)
      navigate('/gates')
    } catch (err: unknown) {
      setError(extractApiError(err, t('auth.invalidCredentials')))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Stack gap="lg">
      <Stack gap={4} align="center">
        <Title order={2}>{t('auth.signIn')}</Title>
        <Text size="sm" c="dimmed">{t('auth.enterUsername')}</Text>
      </Stack>

      <form onSubmit={handleSubmit}>
        <Stack gap="md">
          <TextInput
            label={t('auth.username')}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
            autoComplete="username"
            placeholder={t('auth.usernamePlaceholder')}
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
    </Stack>
  )
}
