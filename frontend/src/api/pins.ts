import { api } from '@/lib/api'
import type { GatePin } from '@/types'

function normalizeList(data: unknown): GatePin[] {
  if (Array.isArray(data)) return data as GatePin[]
  return ((data as Record<string, unknown>).pins ?? []) as GatePin[]
}

export interface PinMetadata {
  /** 'one_shot' (default) opens immediately; 'session' issues a JWT for repeat access. */
  type?: 'one_shot' | 'session'
  /** Refresh token TTL in seconds (session type only). 0 = infinite. */
  session_duration?: number
  /** ISO 8601 date after which this PIN code is no longer valid. */
  expires_at?: string
  allowed_days?: number[]
  allowed_hours_start?: number
  allowed_hours_end?: number
}

export const pinsApi = {
  list: (wsId: string, gateId: string) =>
    api.get(`/workspaces/${wsId}/gates/${gateId}/pins`).then((r) => normalizeList(r.data)),

  create: (wsId: string, gateId: string, params: { pin: string; label?: string; metadata?: PinMetadata }) =>
    api.post<GatePin>(`/workspaces/${wsId}/gates/${gateId}/pins`, params).then((r) => r.data),

  delete: (wsId: string, gateId: string, pinId: string) =>
    api.delete(`/workspaces/${wsId}/gates/${gateId}/pins/${pinId}`),
}
