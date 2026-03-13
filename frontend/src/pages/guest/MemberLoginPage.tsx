import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { publicApi, authApi } from '@/api'
import type { DomainResolveResult } from '@/types'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'
import { CheckCircle2, XCircle, Loader2 } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
import { isSafeRedirect } from '@/lib/utils'

type PageState = 'idle' | 'loading' | 'success' | 'error'

export default function MemberLoginPage() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { t } = useTranslation()

  const gateId = searchParams.get('gate_id')
  const redirectParam = searchParams.get('redirect')
  const errorParam = searchParams.get('error')

  const [resolving, setResolving] = useState(!!gateId)
  const [resolved, setResolved] = useState<DomainResolveResult | null>(null)
  const [ssoProviders, setSsoProviders] = useState<{ id: string; name: string; type: string }[]>([])

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [state, setState] = useState<PageState>('idle')
  const [errorMsg, setErrorMsg] = useState(errorParam ? t('pinpad.ssoError') : '')

  useEffect(() => {
    publicApi.ssoProviders()
      .then((providers) => setSsoProviders(providers))
      .catch(() => {})

    if (!gateId) {
      setResolving(false)
      return
    }
    publicApi.resolveByGateId(gateId)
      .then((data) => setResolved(data))
      .catch(() => {})
      .finally(() => setResolving(false))
  }, [gateId])

  function redirectAfterLogin(role: string) {
    const authState = { state: { justAuthenticated: true } }
    if (isSafeRedirect(redirectParam)) {
      navigate(redirectParam, authState)
      return
    }
    if (role === 'ADMIN') {
      navigate('/gates', authState)
    } else {
      navigate(gateId ? `/gates/${gateId}/public` : '/gates', authState)
    }
  }

  function showFeedback(result: 'success' | 'error', msg = '', role?: string) {
    setState(result)
    setErrorMsg(msg)
    if (result === 'success' && role) {
      setTimeout(() => redirectAfterLogin(role), 1500)
    } else if (result === 'error') {
      setTimeout(() => {
        setState('idle')
        setErrorMsg('')
      }, 4500)
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (state !== 'idle') return
    setState('loading')
    try {
      const data = await authApi.login(username, password)
      useAuthStore.getState().setMemberSession(data.member)
      showFeedback('success', '', data.member.role)
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status === 401 || status === 403) showFeedback('error', t('pinpad.invalidCredentials'))
      else showFeedback('error', t('pinpad.unreachable'))
    }
  }

  function handleSSOLogin(providerId: string) {
    const url = `/api/auth/sso/${encodeURIComponent(providerId)}/authorize`
    window.location.href = gateId ? `${url}?gate_id=${encodeURIComponent(gateId)}` : url
  }

  if (resolving) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
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
            <h2 className="text-xl font-bold">{resolved?.gate_name ?? 'GATIE'}</h2>
            <p className="text-sm text-muted-foreground">{t('pinpad.memberAccess')}</p>
          </div>

          {state === 'success' ? (
            <div className="flex flex-col items-center gap-2">
              <CheckCircle2 className="h-10 w-10 text-emerald-500" />
              <p className="text-sm font-medium text-emerald-600 text-center">{t('pinpad.gateOpened')}</p>
            </div>
          ) : (
            <>
              {(state === 'error' || errorMsg) && (
                <div className="flex flex-col items-center gap-1">
                  <XCircle className="h-8 w-8 text-destructive" />
                  <p className="text-sm font-medium text-destructive text-center">{errorMsg}</p>
                </div>
              )}

              <form onSubmit={handleSubmit} className="w-full space-y-3">
                <Input
                  label={t('pinpad.username')}
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  autoComplete="username"
                  autoFocus
                />
                <Input
                  label={t('auth.password')}
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoComplete="current-password"
                />
                <Button type="submit" size="lg" className="w-full rounded-full" loading={state === 'loading'}>
                  {t('pinpad.memberLogin')}
                </Button>
              </form>

              {ssoProviders.length > 0 && (
                <>
                  <div className="flex items-center gap-3 w-full">
                    <Separator className="flex-1" />
                    <span className="text-xs text-muted-foreground">ou</span>
                    <Separator className="flex-1" />
                  </div>
                  {ssoProviders.map((p) => (
                    <Button
                      key={p.id}
                      variant="outline"
                      size="lg"
                      className="w-full rounded-full"
                      onClick={() => handleSSOLogin(p.id)}
                    >
                      {t('pinpad.loginWithSso', { provider: p.name })}
                    </Button>
                  ))}
                </>
              )}

              {gateId && (
                <button
                  type="button"
                  className="text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                  onClick={() => navigate(isSafeRedirect(redirectParam) ? redirectParam : `/gates/${gateId}/public`)}
                >
                  {t('pinpad.useAnotherMethod')}
                </button>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
