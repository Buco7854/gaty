import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { TextInput, PasswordInput, Button, Stack, Text, Alert, Title } from '@mantine/core'
import { AlertCircle } from 'lucide-react'
import { setupApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import { extractApiError } from '@/lib/notify'

export default function SetupPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const setMemberSession = useAuthStore((s) => s.setMemberSession)
  const qc = useQueryClient()
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
      const data = await setupApi.init(username, password)
      setMemberSession(data.member)
      qc.setQueryData(['setup-status'], { setup_required: false })
    } catch (err: unknown) {
      setError(extractApiError(err, t('setup.failed')))
      setLoading(false)
    }
  }

  return (
    <Stack gap="lg">
      <Stack gap={4} align="center">
        <Title order={2}>{t('setup.title')}</Title>
        <Text size="sm" c="dimmed">{t('setup.subtitle')}</Text>
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
            {t('setup.createAdmin')}
          </Button>
        </Stack>
      </form>
    </Stack>
  )
}
