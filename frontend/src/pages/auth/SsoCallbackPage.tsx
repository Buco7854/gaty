import { useEffect } from 'react'
import { useNavigate, useLocation } from 'react-router'
import { Center, Loader, Stack, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import type { GateSession } from '@/api/public'

function getWorkspaceIdFromJWT(token: string): string | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
    return payload.workspace_id as string ?? null
  } catch {
    return null
  }
}

export default function SsoCallbackPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { t } = useTranslation()

  useEffect(() => {
    const params = new URLSearchParams(location.search)
    const accessToken = params.get('access_token')
    const refreshToken = params.get('refresh_token')
    const error = params.get('error')
    const gateId = params.get('gate_id')
    const workspaceId = params.get('workspace_id')

    if (!error && accessToken && refreshToken) {
      const wsId = workspaceId ?? getWorkspaceIdFromJWT(accessToken) ?? null
      if (!wsId) { navigate('/login', { replace: true }); return }
      const session: GateSession = {
        type: 'member',
        access_token: accessToken,
        refresh_token: refreshToken,
        workspace_id: wsId,
      }
      if (gateId) {
        localStorage.setItem(`gatie_session_${gateId}`, JSON.stringify(session))
        navigate(`/workspaces/${wsId}/gates/${gateId}/public`, { replace: true, state: { justAuthenticated: true } })
      } else {
        // Workspace-level session (SSO login without a specific gate)
        localStorage.setItem(`gatie_session_${wsId}`, JSON.stringify(session))
        navigate(`/workspaces/${wsId}`, { replace: true, state: { justAuthenticated: true } })
      }
      return
    }

    if (error) {
      console.error('[SSO] callback error:', error, { workspaceId, gateId })
      if (workspaceId) {
        const params = new URLSearchParams({ error })
        if (gateId) params.set('gate_id', gateId)
        navigate(`/workspaces/${workspaceId}/login?${params.toString()}`, { replace: true })
      } else {
        navigate('/login', { replace: true })
      }
      return
    }

    console.error('[SSO] callback: missing tokens', { accessToken: !!accessToken, refreshToken: !!refreshToken, error })
    navigate('/login', { replace: true })
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
