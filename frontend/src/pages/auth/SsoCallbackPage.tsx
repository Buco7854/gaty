import { useEffect, useRef } from 'react'
import { useNavigate, useLocation } from 'react-router'
import { Center, Loader, Stack, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/store/auth'
import axios from 'axios'

export default function SsoCallbackPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { t } = useTranslation()
  const exchangedRef = useRef(false)

  useEffect(() => {
    if (exchangedRef.current) return
    exchangedRef.current = true
    const params = new URLSearchParams(location.search)
    const code = params.get('code')
    const error = params.get('error')
    const gateId = params.get('gate_id')
    const workspaceId = params.get('workspace_id')

    // Clear sensitive params from browser history/URL immediately.
    if (code) {
      window.history.replaceState({}, '', '/auth/sso/callback')
    }

    if (!error && code) {
      axios.post<{ type: string; membership_id: string; workspace_id?: string; gate_id?: string }>(
        '/api/auth/sso/exchange',
        { code },
        { withCredentials: true },
      ).then(({ data }) => {
        const wsId = data.workspace_id || workspaceId
        if (!wsId) { navigate('/login', { replace: true }); return }

        // Store local session metadata in zustand (cookies set by backend)
        useAuthStore.getState().setLocalSession(
          data.membership_id,
          wsId,
          'MEMBER',
        )

        const effectiveGateId = data.gate_id || gateId
        if (effectiveGateId) {
          navigate(`/workspaces/${wsId}/gates/${effectiveGateId}/public`, { replace: true, state: { justAuthenticated: true } })
        } else {
          navigate(`/workspaces/${wsId}`, { replace: true, state: { justAuthenticated: true } })
        }
      }).catch(() => {
        navigate(workspaceId ? `/workspaces/${workspaceId}/login?error=exchange_failed` : '/login', { replace: true })
      })
      return
    }

    if (error) {
      if (import.meta.env.DEV) console.error('[SSO] callback error:', error)
      if (workspaceId) {
        const errParams = new URLSearchParams({ error })
        if (gateId) errParams.set('gate_id', gateId)
        navigate(`/workspaces/${workspaceId}/login?${errParams.toString()}`, { replace: true })
      } else {
        navigate('/login', { replace: true })
      }
      return
    }

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
