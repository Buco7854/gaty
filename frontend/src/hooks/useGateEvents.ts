import { useEffect, useRef } from 'react'
import { useAuthStore } from '@/store/auth'
import axios from 'axios'

export interface GateEvent {
  gate_id: string
  status: string
  status_metadata?: Record<string, unknown>
}

/**
 * Subscribes to real-time gate status events via SSE.
 * Uses a one-time ticket obtained via POST (cookie auth) to avoid exposing the JWT in the URL.
 */
export function useGateEvents(onEvent: (event: GateEvent) => void) {
  const session = useAuthStore((s) => s.session)
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  useEffect(() => {
    if (!session || session.type !== 'member') return

    let es: EventSource | null = null
    let cancelled = false

    axios.post<{ ticket: string }>(
      '/api/events/ticket',
      null,
      { withCredentials: true },
    ).then(({ data }) => {
      if (cancelled) return
      const url = `/api/events?ticket=${encodeURIComponent(data.ticket)}`
      es = new EventSource(url)
      es.onmessage = (e) => {
        try {
          const event = JSON.parse(e.data) as GateEvent
          onEventRef.current(event)
        } catch {
          // ignore malformed events
        }
      }
    }).catch(() => {
      // ticket acquisition failed — SSE won't connect
    })

    return () => {
      cancelled = true
      es?.close()
    }
  }, [session])
}
