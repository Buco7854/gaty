import { useEffect, useState } from 'react'
import { useParams } from 'react-router'
import { publicApi, authApi } from '@/api'
import type { GateSession } from '@/api/public'
import type { DomainResolveResult } from '@/types'
import { useTranslation } from 'react-i18next'
import { Center, Stack, Group, Text, Title, Loader, Button, TextInput, PasswordInput, Anchor } from '@mantine/core'
import { Delete, CheckCircle2, XCircle } from 'lucide-react'

type PadState = 'idle' | 'loading' | 'success' | 'error'
type Mode = 'pin' | 'password'

const MAX_PIN = 12
const DIGITS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '', '0', '⌫']

function sessionKey(gateId: string) {
  return `gaty_session_${gateId}`
}

export default function PinPadPage() {
  const { gateId: gateIdParam } = useParams<{ gateId?: string }>()
  const { t } = useTranslation()
  const [resolving, setResolving] = useState(!gateIdParam)
  const [resolved, setResolved] = useState<DomainResolveResult | null>(null)
  const [resolveError, setResolveError] = useState(false)

  const [mode, setMode] = useState<Mode>('pin')
  const [pin, setPin] = useState('')
  const [state, setState] = useState<PadState>('idle')
  const [errorMsg, setErrorMsg] = useState('')

  // Stored session
  const [session, setSession] = useState<GateSession | null>(null)

  // Member credentials form
  const [localUsername, setLocalUsername] = useState('')
  const [localPassword, setLocalPassword] = useState('')

  useEffect(() => {
    if (gateIdParam) return
    const domain = window.location.hostname
    publicApi.resolve(domain)
      .then((data) => setResolved(data))
      .catch(() => setResolveError(true))
      .finally(() => setResolving(false))
  }, [gateIdParam])

  const effectiveGateId = gateIdParam ?? resolved?.gate_id

  // Load stored session when gate ID is known
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
  }

  function showFeedback(result: 'success' | 'error', msg = '') {
    setState(result)
    setErrorMsg(msg)
    setTimeout(() => {
      setState('idle')
      setErrorMsg('')
      setPin('')
    }, 3000)
  }

  async function triggerWithSession(sess: GateSession): Promise<'success' | 'expired' | 'error'> {
    try {
      if (sess.type === 'pin') {
        await publicApi.triggerWithPinSession(sess.access_token)
      } else {
        const gateId = effectiveGateId ?? resolved?.gate_id
        await publicApi.triggerAsLocal(sess.workspace_id!, gateId!, sess.access_token)
      }
      return 'success'
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status !== 401) return 'error'
      // Try refresh
      try {
        const newTokens = await publicApi.refreshSession(sess.refresh_token)
        const updated: GateSession = { ...sess, access_token: newTokens.access_token, refresh_token: newTokens.refresh_token }
        storeSession(updated)
        // Retry
        if (updated.type === 'pin') {
          await publicApi.triggerWithPinSession(updated.access_token)
        } else {
          const gateId = effectiveGateId ?? resolved?.gate_id
          await publicApi.triggerAsLocal(updated.workspace_id!, gateId!, updated.access_token)
        }
        return 'success'
      } catch {
        return 'expired'
      }
    }
  }

  async function openGateWithSession() {
    if (!session || state !== 'idle') return
    setState('loading')
    const result = await triggerWithSession(session)
    if (result === 'success') {
      showFeedback('success')
    } else if (result === 'expired') {
      clearSession()
      showFeedback('error', t('pinpad.sessionExpired'))
    } else {
      showFeedback('error', t('pinpad.unreachable'))
    }
  }

  async function submitPin(finalPin: string) {
    if (!effectiveGateId || finalPin.length < 4) return
    setState('loading')
    try {
      const result = await publicApi.open(effectiveGateId, finalPin)
      if (result.session) {
        storeSession({ type: 'pin', ...result.session })
      }
      showFeedback('success')
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status === 429) showFeedback('error', t('pinpad.tooManyAttempts'))
      else if (status === 403) showFeedback('error', t('pinpad.invalidPin'))
      else showFeedback('error', t('pinpad.unreachable'))
    }
  }

  async function submitCredentials(e: React.FormEvent) {
    e.preventDefault()
    if (!resolved || state !== 'idle') return
    setState('loading')
    try {
      const auth = await authApi.loginLocal(resolved.workspace_slug, localUsername, localPassword)
      await publicApi.triggerAsLocal(resolved.workspace_id, resolved.gate_id, auth.access_token)
      storeSession({
        type: 'member',
        access_token: auth.access_token,
        refresh_token: auth.refresh_token,
        workspace_id: resolved.workspace_id,
      })
      showFeedback('success')
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status === 401 || status === 403) showFeedback('error', t('pinpad.invalidCredentials'))
      else showFeedback('error', t('pinpad.unreachable'))
    }
  }

  function press(d: string) {
    if (state !== 'idle') return
    if (d === '⌫') {
      setPin((p) => p.slice(0, -1))
    } else if (d === '') {
      return
    } else {
      const next = pin + d
      setPin(next)
      if (next.length === MAX_PIN) submitPin(next)
    }
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
  const workspaceName = resolved?.workspace_name

  return (
    <Center mih="100vh" p="md" style={{ userSelect: 'none' }}>
      <Stack align="center" gap="xl" w="100%" maw={320}>
        {/* Header */}
        <Stack align="center" gap={4}>
          {workspaceName && (
            <Text size="xs" fw={600} c="dimmed" style={{ textTransform: 'uppercase', letterSpacing: 2 }}>
              {workspaceName}
            </Text>
          )}
          <Title order={2}>{gateName}</Title>
          <Text size="sm" c="dimmed">
            {session
              ? t('pinpad.sessionActive')
              : mode === 'pin'
                ? t('pinpad.enterPin')
                : t('pinpad.memberAccess')}
          </Text>
        </Stack>

        {/* Feedback indicator */}
        <Center h={40}>
          {state === 'success' ? (
            <CheckCircle2 size={40} color="var(--mantine-color-green-6)" />
          ) : state === 'error' ? (
            <XCircle size={40} color="var(--mantine-color-red-6)" />
          ) : state === 'loading' ? (
            <Loader size="md" />
          ) : !session && mode === 'pin' ? (
            <Group gap="sm">
              {Array.from({ length: Math.max(pin.length, 4) }).map((_, i) => (
                <div
                  key={i}
                  style={{
                    width: 12,
                    height: 12,
                    borderRadius: '50%',
                    backgroundColor: i < pin.length
                      ? 'var(--mantine-color-indigo-6)'
                      : 'var(--mantine-color-default-border)',
                    transition: 'background-color 150ms',
                  }}
                />
              ))}
            </Group>
          ) : null}
        </Center>

        {/* Feedback text */}
        {(state === 'success' || state === 'error') && (
          <Text size="sm" fw={500} c={state === 'success' ? 'green' : 'red'} ta="center">
            {state === 'success' ? t('pinpad.gateOpened') : errorMsg}
          </Text>
        )}

        {/* Session active — show open button */}
        {session && state === 'idle' && (
          <Stack align="center" w="100%" gap="sm">
            <Button size="lg" radius="xl" fullWidth onClick={openGateWithSession}>
              {t('pinpad.openGate')}
            </Button>
            <Anchor
              component="button"
              type="button"
              size="xs"
              c="dimmed"
              onClick={clearSession}
            >
              {t('pinpad.useAnotherMethod')}
            </Anchor>
          </Stack>
        )}

        {/* PIN mode */}
        {!session && mode === 'pin' && state === 'idle' && (
          <>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(3, 1fr)',
                gap: 12,
                width: '100%',
              }}
            >
              {DIGITS.map((d, i) => {
                if (d === '') return <div key={i} />

                if (d === '⌫') {
                  return (
                    <button
                      key={i}
                      onPointerDown={() => press(d)}
                      disabled={pin.length === 0}
                      style={{
                        aspectRatio: '1',
                        borderRadius: 16,
                        border: 'none',
                        backgroundColor: 'var(--mantine-color-default)',
                        color: 'var(--mantine-color-dimmed)',
                        cursor: 'pointer',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        transition: 'opacity 150ms',
                        opacity: pin.length === 0 ? 0.3 : 1,
                      }}
                    >
                      <Delete size={20} />
                    </button>
                  )
                }

                return (
                  <button
                    key={i}
                    onPointerDown={() => press(d)}
                    disabled={pin.length >= MAX_PIN}
                    style={{
                      aspectRatio: '1',
                      borderRadius: 16,
                      border: '1px solid var(--mantine-color-default-border)',
                      backgroundColor: 'var(--mantine-color-body)',
                      fontSize: 22,
                      fontWeight: 600,
                      cursor: 'pointer',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      boxShadow: '0 1px 3px rgba(0,0,0,0.1)',
                      transition: 'opacity 150ms',
                      opacity: pin.length >= MAX_PIN ? 0.3 : 1,
                    }}
                  >
                    {d}
                  </button>
                )
              })}
            </div>

            {pin.length > 0 && pin.length < MAX_PIN && (
              <Button onClick={() => submitPin(pin)} size="md" radius="xl" px="xl">
                {t('common.confirm')}
              </Button>
            )}
          </>
        )}

        {/* Password mode — only available when domain-resolved */}
        {!session && mode === 'password' && state === 'idle' && resolved && (
          <form onSubmit={submitCredentials} style={{ width: '100%' }}>
            <Stack>
              <TextInput
                label={t('pinpad.username')}
                value={localUsername}
                onChange={(e) => setLocalUsername(e.target.value)}
                required
                autoComplete="username"
              />
              <PasswordInput
                label={t('auth.password')}
                value={localPassword}
                onChange={(e) => setLocalPassword(e.target.value)}
                required
                autoComplete="current-password"
              />
              <Button type="submit" size="md" radius="xl">
                {t('pinpad.openWithCredentials')}
              </Button>
            </Stack>
          </form>
        )}

        {/* Mode toggle — only when domain-resolved and no active session */}
        {!session && resolved && state === 'idle' && (
          <Anchor
            component="button"
            type="button"
            size="xs"
            c="dimmed"
            onClick={() => {
              setMode((m) => m === 'pin' ? 'password' : 'pin')
              setPin('')
              setLocalUsername('')
              setLocalPassword('')
            }}
          >
            {mode === 'pin' ? t('pinpad.useCredentials') : t('pinpad.usePinInstead')}
          </Anchor>
        )}
      </Stack>
    </Center>
  )
}
