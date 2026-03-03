import { useEffect, useRef } from 'react'
import { useAuthStore } from '@/store/auth'

export interface GateEvent {
  gate_id: string
  workspace_id: string
  status: string
}

/**
 * Subscribes to real-time gate status events via SSE for the given workspace.
 * Uses a ref for the callback so the EventSource is not recreated on every render.
 */
export function useGateEvents(wsId: string | undefined, onEvent: (event: GateEvent) => void) {
  const accessToken = useAuthStore((s) => s.accessToken)
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  useEffect(() => {
    if (!wsId || !accessToken) return

    const url = `/api/workspaces/${wsId}/events?token=${encodeURIComponent(accessToken)}`
    const es = new EventSource(url)

    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data) as GateEvent
        onEventRef.current(data)
      } catch {
        // ignore malformed events
      }
    }

    return () => {
      es.close()
    }
  }, [wsId, accessToken])
}
