import axios from 'axios'
import { api } from '@/lib/api'
import type { DomainResolveResult } from '@/types'

export interface GateSession {
  type: 'pin' | 'member'
  access_token: string
  refresh_token: string
  workspace_id?: string // member sessions only
}

export const publicApi = {
  resolve: (domain: string) =>
    api.get<DomainResolveResult>(`/public/resolve?domain=${encodeURIComponent(domain)}`).then((r) => r.data),

  /** One-shot unlock — backward-compatible, always opens without creating a session. */
  unlock: (gateId: string, pin: string) =>
    api.post('/public/unlock', { gate_id: gateId, pin }),

  /**
   * Smart open: triggers the gate and, if the PIN is type=session, returns session tokens.
   * response.session is undefined for one-shot PINs.
   */
  open: (gateId: string, pin: string) =>
    api.post<{ session?: { access_token: string; refresh_token: string } }>(
      '/public/open',
      { gate_id: gateId, pin },
    ).then((r) => r.data),

  /** Trigger gate with a stored pin_session JWT (bypasses global-token interceptor). */
  triggerWithPinSession: (accessToken: string) =>
    axios.post('/api/public/trigger', {}, { headers: { Authorization: `Bearer ${accessToken}` } }),

  /** Trigger gate as a local member (bypasses global-token interceptor). */
  triggerAsLocal: (workspaceId: string, gateId: string, localToken: string) =>
    axios.post(
      `/api/workspaces/${workspaceId}/gates/${gateId}/trigger`,
      {},
      { headers: { Authorization: `Bearer ${localToken}` } },
    ),

  /** Refresh any session token type (bypasses global-token interceptor). */
  refreshSession: (refreshToken: string) =>
    axios.post<{ access_token: string; refresh_token: string }>(
      '/api/auth/refresh',
      { refresh_token: refreshToken },
    ).then((r) => r.data),
}
