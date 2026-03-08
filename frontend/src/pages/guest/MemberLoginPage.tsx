import { useEffect, useState } from 'react'
import { useParams, useNavigate, useSearchParams } from 'react-router'
import { publicApi, authApi } from '@/api'
import type { DomainResolveResult } from '@/types'
import { useAuthStore } from '@/store/auth'
import { useTranslation } from 'react-i18next'
import {
  Center, Stack, Group, Text, Title, Loader, Button,
  TextInput, PasswordInput, Divider, Anchor,
} from '@mantine/core'
import { CheckCircle2, XCircle } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'

type PageState = 'idle' | 'loading' | 'success' | 'error'

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
    if (wsId) {
      publicApi.ssoProviders(wsId)
        .then((providers) => setSsoProviders(providers))
        .catch(() => {})
    }

    if (!gateId) {
      setResolving(false)
      return
    }
    publicApi.resolveByGateId(gateId)
      .then((data) => setResolved(data))
      .catch(() => {})
      .finally(() => setResolving(false))
  }, [gateId, wsId, navigate])

  function isSafeRedirect(path: string | null): path is string {
    return !!path && path.startsWith('/') && !path.startsWith('//')
  }

  function redirectAfterLogin(role: string) {
    const authState = { state: { justAuthenticated: true } }
    if (isSafeRedirect(redirectParam)) {
      navigate(redirectParam, authState)
      return
    }
    if (role === 'ADMIN' || role === 'OWNER') {
      navigate(`/workspaces/${wsId}`, authState)
    } else {
      navigate(gateId ? `/workspaces/${wsId}/gates/${gateId}/public` : `/workspaces/${wsId}`, authState)
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
    if (!resolved || state !== 'idle') return
    setState('loading')
    try {
      const data = await authApi.loginLocal(resolved.workspace_id, username, password)
      useAuthStore.getState().setLocalSession(data.membership_id, data.workspace_id, data.role, data.display_name)
      showFeedback('success', '', data.role)
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status === 401 || status === 403) showFeedback('error', t('pinpad.invalidCredentials'))
      else showFeedback('error', t('pinpad.unreachable'))
    }
  }

  function handleSSOLogin(providerId: string) {
    const workspaceId = resolved?.workspace_id ?? wsId
    if (!workspaceId) return
    const url = `/api/auth/sso/${encodeURIComponent(workspaceId)}/${encodeURIComponent(providerId)}/authorize`
    window.location.href = gateId ? `${url}?gate_id=${encodeURIComponent(gateId)}` : url
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
                  onClick={() => navigate(isSafeRedirect(redirectParam) ? redirectParam : `/workspaces/${wsId}/gates/${gateId}/public`)}
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
