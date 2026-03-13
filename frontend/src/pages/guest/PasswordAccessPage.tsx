import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { publicApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'
import { notifyError } from '@/lib/notify'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

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
    <div className="relative min-h-screen">
      <div className="absolute top-3 right-4 z-10 flex items-center gap-1">
        <LangToggle />
        <ThemeToggle />
      </div>

      <div className="flex items-center justify-center min-h-screen p-4">
        <div className="flex flex-col items-center gap-6 w-full max-w-xs">
          <div className="text-center space-y-1">
            <h2 className="text-xl font-bold">{gateName ?? 'Gate'}</h2>
            <p className="text-sm text-muted-foreground">{t('pinpad.enterPasswordCode')}</p>
          </div>

          <form onSubmit={(e) => { e.preventDefault(); submit() }} className="w-full space-y-3">
            <Input
              type="password"
              placeholder={t('pinpad.enterPasswordCode')}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoFocus
              disabled={submitting}
            />
            {password.length > 0 && (
              <Button type="submit" size="lg" className="w-full rounded-full" loading={submitting}>
                {t('common.confirm')}
              </Button>
            )}
          </form>

          <button
            type="button"
            className="text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
            onClick={() => navigate(portalPath)}
          >
            {t('pinpad.useAnotherMethod')}
          </button>
        </div>
      </div>
    </div>
  )
}
