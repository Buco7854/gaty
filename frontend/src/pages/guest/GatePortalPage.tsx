import { useEffect, useRef, useMemo } from 'react'
import { useParams, useNavigate, useLocation } from 'react-router'
import { useQuery, useMutation } from '@tanstack/react-query'
import { publicApi, policiesApi } from '@/api'
import type { DomainResolveResult } from '@/types'
import { useTranslation } from 'react-i18next'
import { notifySuccess, notifyError } from '@/lib/notify'
import { Center, Stack, Group, Text, Title, Loader, Button, Anchor, Paper, Badge } from '@mantine/core'
import { Hash, KeyRound, Users, Activity, LayoutGrid } from 'lucide-react'
import type { GateStatus } from '@/types'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'
import { useAuthStore } from '@/store/auth'
import { getNestedValue, hasNestedKey } from '@/lib/utils'
import { useState } from 'react'

function getStatusColor(status: GateStatus | undefined): string {
  switch (status) {
    case 'online':
    case 'open': return 'green'
    case 'offline':
    case 'closed': return 'red'
    case 'unresponsive':
    case 'unavailable': return 'orange'
    default: return 'gray'
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

  // Resolve gate info
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

  // Determine if we have an active session for this gate
  const hasSession = session?.type === 'pin_session' || session?.type === 'member'

  // Permission derivation for member sessions
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
    onSuccess: () => {
      notifySuccess(t('pinpad.gateOpened'))
    },
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

  // Auto-trigger open when arriving just after authentication
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
      <Center mih="100vh">
        <Loader />
      </Center>
    )
  }

  const gateName = resolved?.gate_name ?? 'Gate'
  const isPending = triggerMutation.isPending

  return (
    <div style={{ position: 'relative', minHeight: '100vh' }}>
      <Group gap="xs" style={{ position: 'absolute', top: 12, right: 16, zIndex: 10 }}>
        <LangToggle />
        <ThemeToggle />
      </Group>

      <Center mih="100vh" p="md" style={{ userSelect: 'none' }}>
        <Stack align="center" gap="xl" w="100%" maw={320}>
          {/* Header */}
          <Stack align="center" gap={4}>
            <Group gap="sm" justify="center">
              <Title order={2}>{gateName}</Title>
              {canViewStatus && resolved?.status && (
                <Badge color={getStatusColor(resolved.status)} variant="light">
                  {t(`common.${resolved.status}`, { defaultValue: resolved.status })}
                </Badge>
              )}
            </Group>
            <Text size="sm" c="dimmed">
              {hasSession ? t('pinpad.sessionActive') : t('pinpad.chooseMethod')}
            </Text>
          </Stack>

          {/* Session active: gate controls */}
          {hasSession && (
            <Stack align="center" w="100%" gap="sm">
              {canOpen && (
                <Button
                  size="lg"
                  radius="xl"
                  fullWidth
                  loading={isPending}
                  onClick={() => triggerMutation.mutate('open')}
                >
                  {t('gates.open')}
                </Button>
              )}
              {canClose && (
                <Button
                  size="lg"
                  radius="xl"
                  fullWidth
                  variant="default"
                  loading={isPending}
                  onClick={() => triggerMutation.mutate('close')}
                >
                  {t('gates.close')}
                </Button>
              )}
              {isAdminSession && (
                <Button
                  variant="subtle"
                  size="xs"
                  leftSection={<LayoutGrid size={14} />}
                  onClick={() => navigate('/gates')}
                >
                  {t('pinpad.myDashboard')}
                </Button>
              )}
              {canViewStatus && metaRows.length > 0 && (
                <Paper withBorder p="sm" radius="md" w="100%">
                  <Group gap="xs" mb="xs">
                    <Activity size={14} opacity={0.6} />
                    <Text size="sm" fw={600}>{t('gates.liveData')}</Text>
                  </Group>
                  <Stack gap={4}>
                    {metaRows.map((row) => (
                      <Group key={row.label} justify="space-between" py={2}>
                        <Text size="sm">{row.label}</Text>
                        <Text size="sm" fw={500} ff="mono">
                          {row.value}{row.unit ? ` ${row.unit}` : ''}
                        </Text>
                      </Group>
                    ))}
                  </Stack>
                </Paper>
              )}
              <Anchor component="button" type="button" size="xs" c="dimmed" onClick={handleClearSession}>
                {t('pinpad.useAnotherMethod')}
              </Anchor>
            </Stack>
          )}

          {/* No session: auth options */}
          {!hasSession && effectiveGateId && (
            <Stack align="center" w="100%" gap="sm">
              <Button
                size="lg"
                radius="xl"
                fullWidth
                leftSection={<Hash size={16} />}
                onClick={navigateToPin}
              >
                {t('pinpad.enterPinCode')}
              </Button>
              <Button
                size="lg"
                radius="xl"
                fullWidth
                variant="default"
                leftSection={<KeyRound size={16} />}
                onClick={navigateToPassword}
              >
                {t('pinpad.enterPasswordCode')}
              </Button>
              <Button
                size="lg"
                radius="xl"
                fullWidth
                variant="default"
                leftSection={<Users size={16} />}
                onClick={navigateToMemberLogin}
              >
                {t('pinpad.memberLogin')}
              </Button>
            </Stack>
          )}
        </Stack>
      </Center>
    </div>
  )
}
