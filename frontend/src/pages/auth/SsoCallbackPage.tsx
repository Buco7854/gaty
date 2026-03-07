import { useEffect } from 'react'
import { useNavigate, useLocation } from 'react-router'
import { Center, Loader, Stack, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import axios from 'axios'
import type { GateSession } from '@/api/public'

export default function SsoCallbackPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { t } = useTranslation()

  useEffect(() => {
    const params = new URLSearchParams(location.search)
    const code = params.get('code')
    const error = params.get('error')
    const gateId = params.get('gate_id')
    const workspaceId = params.get('workspace_id')

    if (error) {
      console.error('[SSO] callback error:', error)
      if (workspaceId) {
        const p = new URLSearchParams({ error })
        if (gateId) p.set('gate_id', gateId)
        navigate(`/workspaces/${workspaceId}/login?${p.toString()}`, { replace: true })
      } else {
        navigate('/login', { replace: true })
      }
      return
    }

    if (!code) {
      console.error('[SSO] callback: missing code')
      navigate('/login', { replace: true })
      return
    }

    // Exchange the one-time code for tokens via a secure POST request.
    axios
      .post<{
        access_token: string
        refresh_token: string
        gate_id?: string
        workspace_id?: string
      }>('/api/auth/sso/exchange', { code })
      .then(({ data }) => {
        const wsId = data.workspace_id ?? workspaceId
        if (!wsId) {
          navigate('/login', { replace: true })
          return
        }
        const session: GateSession = {
          type: 'member',
          access_token: data.access_token,
          refresh_token: data.refresh_token,
          workspace_id: wsId,
        }
        const gid = data.gate_id ?? gateId
        if (gid) {
          localStorage.setItem(`gatie_session_${gid}`, JSON.stringify(session))
          navigate(`/workspaces/${wsId}/gates/${gid}/public`, {
            replace: true,
            state: { justAuthenticated: true },
          })
        } else {
          localStorage.setItem(`gatie_session_${wsId}`, JSON.stringify(session))
          navigate(`/workspaces/${wsId}`, {
            replace: true,
            state: { justAuthenticated: true },
          })
        }
      })
      .catch((err) => {
        console.error('[SSO] exchange failed:', err)
        if (workspaceId) {
          navigate(`/workspaces/${workspaceId}/login?error=exchange_failed`, {
            replace: true,
          })
        } else {
          navigate('/login', { replace: true })
        }
      })
  }, [navigate, location.search])

  return (
    <Center mih="100vh">
      <Stack align="center" gap="md">
        <Loader />
        <Text size="sm" c="dimmed">{t('common.loading')}</Text>
      </Stack>
    </Center>
  )
}
