import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { AlertCircle } from 'lucide-react'
import { setupApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import { extractApiError } from '@/lib/notify'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert, AlertDescription } from '@/components/ui/alert'

export default function SetupPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const setMemberSession = useAuthStore((s) => s.setMemberSession)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (password !== confirm) {
      setError(t('auth.passwordMismatch'))
      return
    }
    setLoading(true)
    setError('')
    try {
      const { member } = await setupApi.init(username, password)
      setMemberSession(member)
      qc.setQueryData(['setup-status'], { setup_required: false })
    } catch (err) {
      setError(extractApiError(err, t('setup.failed')))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="text-center">
        <h2 className="text-xl font-bold">{t('setup.title')}</h2>
        <p className="text-sm text-muted-foreground mt-1">{t('setup.subtitle')}</p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-3">
        {error && (
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <Input
          label={t('auth.username')}
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder={t('auth.usernamePlaceholder')}
          required
          autoFocus
        />
        <Input
          label={t('auth.password')}
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder={t('auth.passwordPlaceholder')}
          required
          minLength={8}
        />
        <Input
          label={t('auth.confirmPassword')}
          type="password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          placeholder={t('auth.passwordPlaceholder')}
          required
          minLength={8}
        />
        <Button type="submit" className="w-full" loading={loading}>
          {t('setup.createAdmin')}
        </Button>
      </form>
    </div>
  )
}
