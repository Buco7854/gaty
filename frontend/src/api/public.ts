import axios from 'axios'
import { api } from '@/lib/api'
import type { DomainResolveResult } from '@/types'

export const publicApi = {
  resolve: (domain: string) =>
    api.get<DomainResolveResult>(`/public/resolve?domain=${encodeURIComponent(domain)}`).then((r) => r.data),

  resolveByGateId: (gateId: string) =>
    api.get<DomainResolveResult>(`/public/gates/${gateId}`).then((r) => r.data),

  /** One-shot unlock — backward-compatible, always opens without creating a session. */
  unlock: (gateId: string, pin: string) =>
    api.post('/public/unlock', { gate_id: gateId, pin }),

  /**
   * Smart open: triggers the gate and, if the PIN is type=session, sets session cookies.
   * Response body indicates whether a session was created and its permissions.
   */
  open: (gateId: string, pin: string) =>
    api.post<{ has_session: boolean; gate_id?: string; permissions?: string[] }>(
      '/public/open',
      { gate_id: gateId, pin },
    ).then((r) => r.data),

  /** Trigger gate with a stored pin_session cookie (sent automatically). */
  triggerWithPinSession: (action: 'open' | 'close' = 'open') =>
    api.post('/public/trigger', { action }),

  /** Trigger gate as a local member (cookie sent automatically). */
  triggerAsLocal: (workspaceId: string, gateId: string, action: 'open' | 'close' = 'open') =>
    api.post(`/workspaces/${workspaceId}/gates/${gateId}/trigger`, { action }),

  /** List public SSO providers for a workspace (public, no auth required). */
  ssoProviders: (wsId: string) =>
    axios.get<{ id: string; name: string; type: string }[]>(
      `/api/auth/sso/${encodeURIComponent(wsId)}/providers`,
    ).then((r) => r.data),
}
