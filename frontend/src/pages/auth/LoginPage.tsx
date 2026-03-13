import { useState } from 'react'
import { useNavigate } from 'react-router'
import { authApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'
import { extractApiError } from '@/lib/notify'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { DoorOpen, AlertCircle } from 'lucide-react'

export default function LoginPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const setMemberSession = useAuthStore((s) => s.setMemberSession)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      const { member } = await authApi.login(username, password)
      setMemberSession(member)
      navigate('/gates')
    } catch (err) {
      setError(extractApiError(err, t('auth.invalidCredentials')))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex items-center justify-center min-h-screen p-4">
      <div className="w-full max-w-sm space-y-8">
        <div className="flex flex-col items-center gap-3">
          <div className="flex items-center justify-center h-10 w-10 rounded-lg bg-primary/10 text-primary">
            <DoorOpen className="h-5 w-5" />
          </div>
          <div className="text-center">
            <h1 className="text-2xl font-bold">{t('auth.signIn')}</h1>
            <p className="text-sm text-muted-foreground mt-1">{t('auth.enterUsername')}</p>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
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
          />
          <Button type="submit" className="w-full" loading={loading}>
            {t('auth.signIn')}
          </Button>
        </form>
      </div>
    </div>
  )
}
