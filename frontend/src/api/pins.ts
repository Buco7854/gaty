import { api } from '@/lib/api'
import type { GatePin } from '@/types'

function normalizeList(data: unknown): GatePin[] {
  if (Array.isArray(data)) return data as GatePin[]
  return ((data as Record<string, unknown>).pins ?? []) as GatePin[]
}

export interface PinMetadata {
  /** Refresh token TTL in seconds. 0 = infinite. Default: 7 days. Null reverts to default.
   *  Controls how long the browser session stays valid after entering the PIN. */
  session_duration?: number | null
  /** Maximum number of times this PIN can be used. 0 or absent = unlimited. Null clears the limit. */
  max_uses?: number | null
  /** ISO 8601 date after which this PIN code is no longer valid. Null clears the expiration. */
  expires_at?: string | null
  /** Permissions granted to the PIN session. Defaults to ['gate:trigger_open']. */
  permissions?: string[]
  /** Whether this code is a digit-only PIN (shown on numpad) or an alphanumeric password. */
  code_type?: 'pin' | 'password'
}

export const pinsApi = {
  list: (gateId: string) =>
    api.get(`/gates/${gateId}/pins`).then((r) => normalizeList(r.data)),

  create: (gateId: string, params: { pin: string; code_type?: 'pin' | 'password'; label: string; schedule_id?: string; metadata?: PinMetadata }) =>
    api.post<GatePin>(`/gates/${gateId}/pins`, params).then((r) => r.data),

  update: (gateId: string, pinId: string, params: { label: string; metadata?: PinMetadata }) =>
    api.patch<GatePin>(`/gates/${gateId}/pins/${pinId}`, params).then((r) => r.data),

  setSchedule: (gateId: string, pinId: string, scheduleId: string) =>
    api.put(`/gates/${gateId}/pins/${pinId}/schedule`, { schedule_id: scheduleId }),

  clearSchedule: (gateId: string, pinId: string) =>
    api.delete(`/gates/${gateId}/pins/${pinId}/schedule`),

  delete: (gateId: string, pinId: string) =>
    api.delete(`/gates/${gateId}/pins/${pinId}`),
}
