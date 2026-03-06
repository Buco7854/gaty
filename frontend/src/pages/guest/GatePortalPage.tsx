import { useEffect, useRef, useMemo } from 'react'
import { useParams, useNavigate, useLocation } from 'react-router'
import { useQuery, useMutation } from '@tanstack/react-query'
import { publicApi, policiesApi } from '@/api'
import type { GateSession } from '@/api/public'
import type { DomainResolveResult } from '@/types'
import { getPermissionsFromJWT, getRoleFromJWT } from '@/utils/session'
import { useTranslation } from 'react-i18next'
import { notifySuccess, notifyError } from '@/lib/notify'
import { Center, Stack, Group, Text, Title, Loader, Button, Anchor } from '@mantine/core'
import { XCircle, Hash, KeyRound, LayoutGrid, Users } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'
import { useAuthStore } from '@/store/auth'
import { useState } from 'react'

function sessionKey(gateId: string) {
  return `gatie_session_${gateId}`
}

export default function GatePortalPage() {
  const { wsId: wsIdParam, gateId: gateIdParam } = useParams<{ wsId?: string; gateId?: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const { t } = useTranslation()
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)

  const [resolving, setResolving] = useState(true)
  const [resolved, setResolved] = useState<DomainResolveResult | null>(null)
  const [resolveError, setResolveError] = useState(false)
  const [session, setSession] = useState<GateSession | null>(null)
  const autoTriggeredRef = useRef(false)

  // Resolve gate info: by gateId param or by custom domain
  useEffect(() => {
    if (gateIdParam) {
      publicApi.resolveByGateId(gateIdParam)
        .then((data) => setResolved(data))
        .catch(() => {/* gate info unavailable, gateIdParam still works for PIN */})
        .finally(() => setResolving(false))
      return
    }
    const domain = window.location.hostname
    publicApi.resolve(domain)
      .then((data) => setResolved(data))
      .catch(() => {
        // Not a custom domain — redirect based on auth state.
        if (isAuthenticated()) {
          navigate('/workspaces', { replace: true })
        } else {
          navigate('/login', { replace: true })
        }
        setResolveError(true)
      })
      .finally(() => setResolving(false))
  }, [gateIdParam])

  const effectiveGateId = gateIdParam ?? resolved?.gate_id
  const effectiveWsId = wsIdParam ?? resolved?.workspace_id

  // Load stored session
  useEffect(() => {
    if (!effectiveGateId) return
    const raw = localStorage.getItem(sessionKey(effectiveGateId))
    if (!raw) return
    try {
      setSession(JSON.parse(raw) as GateSession)
    } catch {
      localStorage.removeItem(sessionKey(effectiveGateId))
    }
  }, [effectiveGateId])

  function storeSession(sess: GateSession) {
    if (!effectiveGateId) return
    localStorage.setItem(sessionKey(effectiveGateId), JSON.stringify(sess))
    setSession(sess)
  }

  function clearSession() {
    if (!effectiveGateId) return
    localStorage.removeItem(sessionKey(effectiveGateId))
    setSession(null)
    autoTriggeredRef.current = false
  }

  // Permission derivation for member sessions
  const { data: myPolicies } = useQuery({
    queryKey: ['policies-me', session?.workspace_id],
    queryFn: () => policiesApi.listMine(session!.workspace_id!),
    enabled: session?.type === 'member' && !!session.workspace_id,
  })

  const permissions = useMemo(() => {
    if (!session) return []
    if (session.type === 'pin') return getPermissionsFromJWT(session.access_token)
    return myPolicies
      ?.filter((p) => p.gate_id === effectiveGateId)
      .map((p) => p.permission_code) ?? []
  }, [session, myPolicies, effectiveGateId])

  const gateHasOpen = resolved?.has_open_action ?? true
  const gateHasClose = resolved?.has_close_action ?? false
  const canOpen = permissions.includes('gate:trigger_open') && gateHasOpen
  const canClose = permissions.includes('gate:trigger_close') && gateHasClose
  const sessionRole = session?.type === 'member' ? getRoleFromJWT(session.access_token) : null
  const isAdminSession = sessionRole === 'ADMIN' || sessionRole === 'OWNER'
  const policiesReady = session?.type === 'pin' || !!myPolicies

  async function triggerWithSession(sess: GateSession, action: 'open' | 'close') {
    try {
      if (sess.type === 'pin') {
        await publicApi.triggerWithPinSession(sess.access_token, action)
      } else {
        await publicApi.triggerAsLocal(sess.workspace_id!, effectiveGateId!, sess.access_token, action)
      }
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status !== 401) throw err
      // Try refresh
      const newTokens = await publicApi.refreshSession(sess.refresh_token)
      const updated: GateSession = { ...sess, access_token: newTokens.access_token, refresh_token: newTokens.refresh_token }
      storeSession(updated)
      if (updated.type === 'pin') {
        await publicApi.triggerWithPinSession(updated.access_token, action)
      } else {
        await publicApi.triggerAsLocal(updated.workspace_id!, effectiveGateId!, updated.access_token, action)
      }
    }
  }

  const triggerMutation = useMutation({
    mutationFn: (action: 'open' | 'close') => triggerWithSession(session!, action),
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
    if (!session) return
    if (session.type === 'member' && !myPolicies) return // wait for policies
    if (!canOpen) return
    autoTriggeredRef.current = true
    triggerMutation.mutate('open')
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session, policiesReady, canOpen])

  function navigateToPin() {
    if (!effectiveGateId || !effectiveWsId) return
    navigate(`/workspaces/${effectiveWsId}/gates/${effectiveGateId}/public/pin`)
  }

  function navigateToPassword() {
    if (!effectiveGateId || !effectiveWsId) return
    navigate(`/workspaces/${effectiveWsId}/gates/${effectiveGateId}/public/password`)
  }

  function navigateToMemberLogin() {
    if (!effectiveWsId || !effectiveGateId) return
    const params = new URLSearchParams({ gate_id: effectiveGateId, redirect: window.location.pathname })
    navigate(`/workspaces/${effectiveWsId}/login?${params.toString()}`)
  }

  if (resolving) {
    return (
      <Center mih="100vh">
        <Loader />
      </Center>
    )
  }

  if (resolveError) {
    return (
      <Center mih="100vh" p="xl">
        <Stack align="center" gap="sm">
          <XCircle size={48} color="var(--mantine-color-red-6)" />
          <Title order={3}>{t('pinpad.domainNotConfigured')}</Title>
          <Text size="sm" c="dimmed" ta="center">{t('pinpad.domainNotConfiguredHint')}</Text>
        </Stack>
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
            <Title order={2}>{gateName}</Title>
            <Text size="sm" c="dimmed">
              {session ? t('pinpad.sessionActive') : t('pinpad.chooseMethod')}
            </Text>
          </Stack>

          {/* Session active: gate controls */}
          {session && (
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
              {isAdminSession && session.workspace_id && (
                <Button
                  variant="subtle"
                  size="xs"
                  leftSection={<LayoutGrid size={14} />}
                  onClick={() => navigate(`/workspaces/${session.workspace_id}`)}
                >
                  {t('pinpad.myWorkspace')}
                </Button>
              )}
              <Anchor component="button" type="button" size="xs" c="dimmed" onClick={clearSession}>
                {t('pinpad.useAnotherMethod')}
              </Anchor>
            </Stack>
          )}

          {/* No session: auth options */}
          {!session && effectiveGateId && (
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
              {effectiveWsId && (
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
              )}
            </Stack>
          )}
        </Stack>
      </Center>
    </div>
  )
}
