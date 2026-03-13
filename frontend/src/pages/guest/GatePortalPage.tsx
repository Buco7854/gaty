import { useEffect, useRef, useMemo, useState } from 'react'
import { useParams, useNavigate, useLocation } from 'react-router'
import { useQuery, useMutation } from '@tanstack/react-query'
import { publicApi, policiesApi } from '@/api'
import type { DomainResolveResult, GateStatus } from '@/types'
import { useTranslation } from 'react-i18next'
import { notifySuccess, notifyError } from '@/lib/notify'
import { Hash, KeyRound, Users, Activity, LayoutGrid, Loader2 } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'
import { useAuthStore } from '@/store/auth'
import { getNestedValue, hasNestedKey } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

function getStatusVariant(status: GateStatus | undefined): 'success' | 'destructive' | 'warning' | 'secondary' {
  switch (status) {
    case 'online':
    case 'open': return 'success'
    case 'offline':
    case 'closed': return 'destructive'
    case 'unresponsive':
    case 'unavailable': return 'warning'
    default: return 'secondary'
  }
}

export default function GatePortalPage() {
  const { gateId: gateIdParam } = useParams<{ gateId?: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const { t } = useTranslation()
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const session = useAuthStore((s) => s.session)
  const clearSession = useAuthStore((s) => s.clearSession)

  const [resolving, setResolving] = useState(true)
  const [resolved, setResolved] = useState<DomainResolveResult | null>(null)

  const autoTriggeredRef = useRef(false)

  useEffect(() => {
    if (gateIdParam) {
      publicApi.resolveByGateId(gateIdParam)
        .then((data) => setResolved(data))
        .catch(() => {})
        .finally(() => setResolving(false))
      return
    }
    publicApi.resolve(window.location.hostname)
      .then((data) => setResolved(data))
      .catch(() => {
        if (isAuthenticated()) {
          navigate('/gates', { replace: true })
        } else {
          navigate('/login', { replace: true })
        }
      })
      .finally(() => setResolving(false))
  }, [gateIdParam])

  const effectiveGateId = gateIdParam ?? resolved?.gate_id
  const hasSession = session?.type === 'pin_session' || session?.type === 'member'

  const { data: myPolicies } = useQuery({
    queryKey: ['policies-me'],
    queryFn: () => policiesApi.listMine(),
    enabled: session?.type === 'member',
  })

  const permissions = useMemo(() => {
    if (!session) return []
    if (session.type === 'pin_session') return session.permissions ?? []
    if (session.type === 'member') {
      return myPolicies
        ?.filter((p) => p.gate_id === effectiveGateId)
        .map((p) => p.permission_code) ?? []
    }
    return []
  }, [session, myPolicies, effectiveGateId])

  const gateHasOpen = resolved?.has_open_action ?? true
  const gateHasClose = resolved?.has_close_action ?? false
  const canOpen = permissions.includes('gate:trigger_open') && gateHasOpen
  const canClose = permissions.includes('gate:trigger_close') && gateHasClose
  const canViewStatus = permissions.includes('gate:read_status')
  const isAdminSession = session?.type === 'member' && session.member?.role === 'ADMIN'
  const policiesReady = session?.type === 'pin_session' || !!myPolicies

  const metaRows = useMemo(() => {
    if (!resolved?.status_metadata || !resolved?.meta_config) return []
    const meta = resolved.status_metadata as Record<string, unknown>
    return resolved.meta_config
      .filter((f) => hasNestedKey(meta, f.key))
      .map((f) => ({
        label: f.label,
        value: String(getNestedValue(meta, f.key) ?? ''),
        unit: f.unit,
      }))
  }, [resolved])

  async function triggerGate(action: 'open' | 'close') {
    if (session?.type === 'pin_session') {
      await publicApi.triggerWithPinSession(action)
    } else if (session?.type === 'member' && effectiveGateId) {
      await publicApi.triggerAsMember(effectiveGateId, action)
    }
  }

  const triggerMutation = useMutation({
    mutationFn: (action: 'open' | 'close') => triggerGate(action),
    onSuccess: () => notifySuccess(t('pinpad.gateOpened')),
    onError: (err: unknown) => {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status === 401) {
        clearSession()
        notifyError(null, t('pinpad.sessionExpired'))
      } else {
        notifyError(null, t('pinpad.unreachable'))
      }
    },
  })

  useEffect(() => {
    if (!location.state?.justAuthenticated) return
    if (autoTriggeredRef.current) return
    if (!hasSession) return
    if (session?.type === 'member' && !myPolicies) return
    if (!canOpen) return
    autoTriggeredRef.current = true
    triggerMutation.mutate('open')
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hasSession, policiesReady, canOpen])

  function navigateToPin() {
    if (!effectiveGateId) return
    navigate(`/gates/${effectiveGateId}/public/pin`)
  }

  function navigateToPassword() {
    if (!effectiveGateId) return
    navigate(`/gates/${effectiveGateId}/public/password`)
  }

  function navigateToMemberLogin() {
    if (!effectiveGateId) return
    const params = new URLSearchParams({ gate_id: effectiveGateId, redirect: window.location.pathname })
    navigate(`/member-login?${params.toString()}`)
  }

  function handleClearSession() {
    clearSession()
    autoTriggeredRef.current = false
  }

  if (resolving) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  const gateName = resolved?.gate_name ?? 'Gate'

  return (
    <div className="relative min-h-screen">
      <div className="absolute top-3 right-4 z-10 flex items-center gap-1">
        <LangToggle />
        <ThemeToggle />
      </div>

      <div className="flex items-center justify-center min-h-screen p-4 select-none">
        <div className="flex flex-col items-center gap-6 w-full max-w-xs">
          {/* Header */}
          <div className="text-center space-y-1">
            <div className="flex items-center gap-2 justify-center">
              <h2 className="text-xl font-bold">{gateName}</h2>
              {canViewStatus && resolved?.status && (
                <Badge variant={getStatusVariant(resolved.status)}>
                  {t(`common.${resolved.status}`, { defaultValue: resolved.status })}
                </Badge>
              )}
            </div>
            <p className="text-sm text-muted-foreground">
              {hasSession ? t('pinpad.sessionActive') : t('pinpad.chooseMethod')}
            </p>
          </div>

          {/* Session active: gate controls */}
          {hasSession && (
            <div className="flex flex-col items-center w-full gap-3">
              {canOpen && (
                <Button
                  size="lg"
                  className="w-full rounded-full"
                  loading={triggerMutation.isPending}
                  onClick={() => triggerMutation.mutate('open')}
                >
                  {t('gates.open')}
                </Button>
              )}
              {canClose && (
                <Button
                  size="lg"
                  variant="outline"
                  className="w-full rounded-full"
                  loading={triggerMutation.isPending}
                  onClick={() => triggerMutation.mutate('close')}
                >
                  {t('gates.close')}
                </Button>
              )}
              {isAdminSession && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => navigate('/gates')}
                >
                  <LayoutGrid className="h-3.5 w-3.5" />
                  {t('pinpad.myDashboard')}
                </Button>
              )}
              {canViewStatus && metaRows.length > 0 && (
                <div className="border rounded-lg p-3 w-full">
                  <div className="flex items-center gap-1.5 mb-2">
                    <Activity className="h-3.5 w-3.5 opacity-60" />
                    <p className="text-sm font-semibold">{t('gates.liveData')}</p>
                  </div>
                  <div className="space-y-1">
                    {metaRows.map((row) => (
                      <div key={row.label} className="flex items-center justify-between py-0.5">
                        <span className="text-sm">{row.label}</span>
                        <span className="text-sm font-medium font-mono">
                          {row.value}{row.unit ? ` ${row.unit}` : ''}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
              <button
                type="button"
                className="text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
                onClick={handleClearSession}
              >
                {t('pinpad.useAnotherMethod')}
              </button>
            </div>
          )}

          {/* No session: auth options */}
          {!hasSession && effectiveGateId && (
            <div className="flex flex-col items-center w-full gap-3">
              <Button
                size="lg"
                className="w-full rounded-full"
                onClick={navigateToPin}
              >
                <Hash className="h-4 w-4" />
                {t('pinpad.enterPinCode')}
              </Button>
              <Button
                size="lg"
                variant="outline"
                className="w-full rounded-full"
                onClick={navigateToPassword}
              >
                <KeyRound className="h-4 w-4" />
                {t('pinpad.enterPasswordCode')}
              </Button>
              <Button
                size="lg"
                variant="outline"
                className="w-full rounded-full"
                onClick={navigateToMemberLogin}
              >
                <Users className="h-4 w-4" />
                {t('pinpad.memberLogin')}
              </Button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
