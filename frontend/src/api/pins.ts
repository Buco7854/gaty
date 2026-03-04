import { api } from '@/lib/api'
import type { GatePin } from '@/types'

function normalizeList(data: unknown): GatePin[] {
  if (Array.isArray(data)) return data as GatePin[]
  return ((data as Record<string, unknown>).pins ?? []) as GatePin[]
}

export const pinsApi = {
  list: (wsId: string, gateId: string) =>
    api.get(`/workspaces/${wsId}/gates/${gateId}/pins`).then((r) => normalizeList(r.data)),

  create: (wsId: string, gateId: string, params: { pin: string; label?: string; expires_at?: string }) =>
    api.post<GatePin>(`/workspaces/${wsId}/gates/${gateId}/pins`, params).then((r) => r.data),

  delete: (wsId: string, gateId: string, pinId: string) =>
    api.delete(`/workspaces/${wsId}/gates/${gateId}/pins/${pinId}`),
}
