import { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { publicApi } from '@/api'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'
import { Delete } from 'lucide-react'
import { notifyError } from '@/lib/notify'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const DIGITS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '', '0', '⌫']

export default function PinPadPage() {
  const { gateId } = useParams<{ gateId: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()

  const [gateName, setGateName] = useState<string | null>(null)
  const [code, setCode] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [usePassword, setUsePassword] = useState(false)

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

  const lastSubmitRef = useRef(0)

  async function submitCode(value: string) {
    const now = Date.now()
    if (!gateId || value.length < 1 || submitting || now - lastSubmitRef.current < 1000) return
    lastSubmitRef.current = now
    setSubmitting(true)
    try {
      const result = await publicApi.open(gateId, value)
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
      setCode('')
    } finally {
      setSubmitting(false)
    }
  }

  function press(d: string) {
    if (submitting) return
    if (d === '⌫') {
      setCode((p) => p.slice(0, -1))
    } else {
      setCode((p) => p + d)
    }
  }

  function switchMode(password: boolean) {
    setCode('')
    setUsePassword(password)
  }

  return (
    <div className="relative min-h-screen">
      <div className="absolute top-3 right-4 z-10 flex items-center gap-1">
        <LangToggle />
        <ThemeToggle />
      </div>

      <div className="flex items-center justify-center min-h-screen p-4 select-none">
        <div className="flex flex-col items-center gap-6 w-full max-w-xs">
          <div className="text-center space-y-1">
            <h2 className="text-xl font-bold">{gateName ?? 'Gate'}</h2>
            <p className="text-sm text-muted-foreground">{t('pinpad.enterPin')}</p>
          </div>

          {usePassword ? (
            <>
              <form onSubmit={(e) => { e.preventDefault(); submitCode(code) }} className="w-full space-y-3">
                <Input
                  type="password"
                  placeholder={t('pinpad.enterPin')}
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  autoFocus
                  disabled={submitting}
                />
                {code.length > 0 && (
                  <Button type="submit" size="lg" className="w-full rounded-full" loading={submitting}>
                    {t('common.confirm')}
                  </Button>
                )}
              </form>
              <button
                type="button"
                className="text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                onClick={() => switchMode(false)}
              >
                {t('pinpad.usePinInstead')}
              </button>
            </>
          ) : (
            <>
              {/* Dot indicator */}
              <div className="flex items-center gap-2">
                {Array.from({ length: Math.max(code.length, 4) }).map((_, i) => (
                  <div
                    key={i}
                    className={`w-3 h-3 rounded-full transition-colors ${
                      i < code.length ? 'bg-primary' : 'bg-border'
                    }`}
                  />
                ))}
              </div>

              {/* Numpad */}
              <div className="grid grid-cols-3 gap-3 w-full">
                {DIGITS.map((d, i) => {
                  if (d === '') return <div key={i} />
                  if (d === '⌫') {
                    return (
                      <button
                        key={i}
                        onPointerDown={() => press(d)}
                        disabled={submitting || code.length === 0}
                        className="aspect-square rounded-2xl bg-secondary text-muted-foreground flex items-center justify-center cursor-pointer transition-opacity disabled:opacity-30"
                      >
                        <Delete className="h-5 w-5" />
                      </button>
                    )
                  }
                  return (
                    <button
                      key={i}
                      onPointerDown={() => press(d)}
                      disabled={submitting}
                      className="aspect-square rounded-2xl border bg-background text-[22px] font-semibold flex items-center justify-center cursor-pointer shadow-sm transition-opacity disabled:opacity-30"
                    >
                      {d}
                    </button>
                  )
                })}
              </div>

              {code.length > 0 && (
                <Button size="lg" className="rounded-full px-8" loading={submitting} onClick={() => submitCode(code)}>
                  {t('common.confirm')}
                </Button>
              )}
            </>
          )}

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
