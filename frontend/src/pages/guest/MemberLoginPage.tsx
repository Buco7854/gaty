import { useEffect, useState } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router'
import { publicApi, authApi } from '@/api'
import type { DomainResolveResult } from '@/types'
import type { GateSession } from '@/api/public'
import { getRoleFromJWT } from '@/utils/session'
import { useTranslation } from 'react-i18next'
import {
  Center, Stack, Group, Text, Title, Loader, Button,
  TextInput, PasswordInput, Divider, Anchor,
} from '@mantine/core'
import { CheckCircle2, XCircle } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'

type PageState = 'idle' | 'loading' | 'success' | 'error'

function sessionKey(gateId: string) {
  return `gaty_session_${gateId}`
}

export default function MemberLoginPage() {
  const { wsId } = useParams<{ wsId: string }>()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const { t } = useTranslation()

  const gateId = searchParams.get('gate_id')
  const redirectParam = searchParams.get('redirect')
  const errorParam = searchParams.get('error')

  const [resolving, setResolving] = useState(true)
  const [resolved, setResolved] = useState<DomainResolveResult | null>(null)
  const [ssoProviders, setSsoProviders] = useState<{ id: string; name: string; type: string }[]>([])

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [state, setState] = useState<PageState>('idle')
  const [errorMsg, setErrorMsg] = useState(errorParam ? t('pinpad.ssoError') : '')

  useEffect(() => {
    if (!gateId) {
      // No gate_id: just resolve workspace SSO providers for display
      setResolving(false)
      return
    }
    publicApi.resolveByGateId(gateId)
      .then((data) => {
        setResolved(data)
        publicApi.ssoProviders(data.workspace_id)
          .then((providers) => setSsoProviders(providers))
          .catch(() => {})
      })
      .catch(() => {/* workspace info unavailable, show login form anyway */})
      .finally(() => setResolving(false))
  }, [gateId, wsId, navigate])

  function redirectAfterLogin(accessToken: string) {
    const authState = { state: { justAuthenticated: true } }
    // Explicit redirect param takes priority
    if (redirectParam) {
      navigate(redirectParam, authState)
      return
    }
    const role = getRoleFromJWT(accessToken)
    if (role === 'ADMIN' || role === 'OWNER') {
      navigate(`/workspaces/${wsId}`, authState)
    } else {
      navigate(gateId ? `/workspaces/${wsId}/gates/${gateId}/public` : `/workspaces/${wsId}`, authState)
    }
  }

  function showFeedback(result: 'success' | 'error', msg = '', accessToken?: string) {
    setState(result)
    setErrorMsg(msg)
    if (result === 'success' && accessToken) {
      setTimeout(() => redirectAfterLogin(accessToken), 1500)
    } else if (result === 'error') {
      setTimeout(() => {
        setState('idle')
        setErrorMsg('')
      }, 4500)
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!resolved || state !== 'idle') return
    setState('loading')
    try {
      const auth = await authApi.loginLocal(resolved.workspace_id, username, password)
      const session: GateSession = {
        type: 'member',
        access_token: auth.access_token,
        refresh_token: auth.refresh_token,
        workspace_id: wsId,
      }
      localStorage.setItem(sessionKey(gateId!), JSON.stringify(session))
      showFeedback('success', '', auth.access_token)
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status === 401 || status === 403) showFeedback('error', t('pinpad.invalidCredentials'))
      else showFeedback('error', t('pinpad.unreachable'))
    }
  }

  function handleSSOLogin(providerId: string) {
    if (!resolved || !gateId) return
    window.location.href = `/api/auth/sso/${encodeURIComponent(resolved.workspace_id)}/${encodeURIComponent(providerId)}/authorize?gate_id=${encodeURIComponent(gateId)}`
  }

  if (resolving) {
    return (
      <Center mih="100vh">
        <Loader />
      </Center>
    )
  }

  return (
    <div style={{ position: 'relative', minHeight: '100vh' }}>
      <Group gap="xs" style={{ position: 'absolute', top: 12, right: 16, zIndex: 10 }}>
        <LangToggle />
        <ThemeToggle />
      </Group>

      <Center mih="100vh" p="md">
        <Stack align="center" gap="xl" w="100%" maw={320}>
          <Stack align="center" gap={4}>
            <Title order={2}>{resolved?.workspace_name}</Title>
            <Text size="sm" c="dimmed">{t('pinpad.memberAccess')}</Text>
          </Stack>

          {state === 'success' ? (
            <Stack align="center" gap="sm">
              <CheckCircle2 size={40} color="var(--mantine-color-green-6)" />
              <Text size="sm" fw={500} c="green" ta="center">{t('pinpad.gateOpened')}</Text>
            </Stack>
          ) : (
            <>
              {(state === 'error' || errorMsg) && (
                <Stack align="center" gap={4}>
                  <XCircle size={32} color="var(--mantine-color-red-6)" />
                  <Text size="sm" fw={500} c="red" ta="center">{errorMsg}</Text>
                </Stack>
              )}

              <form onSubmit={handleSubmit} style={{ width: '100%' }}>
                <Stack>
                  <TextInput
                    label={t('pinpad.username')}
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    required
                    autoComplete="username"
                    autoFocus
                  />
                  <PasswordInput
                    label={t('auth.password')}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                    autoComplete="current-password"
                  />
                  <Button type="submit" size="md" radius="xl" loading={state === 'loading'}>
                    {t('pinpad.memberLogin')}
                  </Button>
                </Stack>
              </form>

              {ssoProviders.length > 0 && (
                <>
                  <Divider label="ou" labelPosition="center" w="100%" />
                  {ssoProviders.map((p) => (
                    <Button
                      key={p.id}
                      variant="default"
                      size="md"
                      radius="xl"
                      fullWidth
                      onClick={() => handleSSOLogin(p.id)}
                    >
                      {t('pinpad.loginWithSso', { provider: p.name })}
                    </Button>
                  ))}
                </>
              )}

              {gateId && (
                <Anchor
                  component="button"
                  type="button"
                  size="xs"
                  c="dimmed"
                  onClick={() => navigate(`/workspaces/${wsId}/gates/${gateId}/public`)}
                >
                  {t('pinpad.useAnotherMethod')}
                </Anchor>
              )}
            </>
          )}
        </Stack>
      </Center>
    </div>
  )
}
