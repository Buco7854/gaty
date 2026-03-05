import { useEffect } from 'react'
import { useNavigate } from 'react-router'
import { Center, Loader, Stack, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import type { GateSession } from '@/api/public'

function getWorkspaceIdFromJWT(token: string): string | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
    return payload.workspace_id ?? null
  } catch {
    return null
  }
}

export default function SsoCallbackPage() {
  const navigate = useNavigate()
  const { t } = useTranslation()

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const accessToken = params.get('access_token')
    const refreshToken = params.get('refresh_token')
    const error = params.get('error')
    const gateId = params.get('gate_id')
    const workspaceId = params.get('workspace_id')

    if (!error && accessToken && refreshToken && gateId) {
      const wsId = workspaceId ?? getWorkspaceIdFromJWT(accessToken) ?? null
      if (!wsId) { navigate('/', { replace: true }); return }
      const session: GateSession = {
        type: 'member',
        access_token: accessToken,
        refresh_token: refreshToken,
        workspace_id: wsId,
      }
      localStorage.setItem(`gaty_session_${gateId}`, JSON.stringify(session))
      // Navigate to the proper gate portal route, not the legacy /unlock path.
      navigate(`/workspaces/${wsId}/gates/${gateId}/public`, { replace: true, state: { justAuthenticated: true } })
      return
    }

    if (error) {
      if (workspaceId && gateId) {
        navigate(
          `/workspaces/${workspaceId}/login?gate_id=${encodeURIComponent(gateId)}&error=${encodeURIComponent(error)}`,
          { replace: true }
        )
      } else {
        navigate('/', { replace: true })
      }
      return
    }

    navigate('/', { replace: true })
  }, [navigate])

  return (
    <Center mih="100vh">
      <Stack align="center" gap="md">
        <Loader />
        <Text size="sm" c="dimmed">{t('common.loading')}</Text>
      </Stack>
    </Center>
  )
}
