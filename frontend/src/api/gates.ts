import { api } from '@/lib/api'
import type { Gate } from '@/types'

function normalizeList(data: unknown): Gate[] {
  if (Array.isArray(data)) return data as Gate[]
  return ((data as Record<string, unknown>).gates ?? []) as Gate[]
}

export type ActionDriverType = 'MQTT' | 'HTTP' | 'NONE'

export interface ActionConfig {
  type: ActionDriverType
  config?: Record<string, unknown>
}

export interface CreateGateParams {
  name: string
  integration_type?: string
  open_config?: ActionConfig | null
  close_config?: ActionConfig | null
  status_config?: ActionConfig | null
}

export interface UpdateGateParams {
  name?: string
  open_config?: ActionConfig | null
  close_config?: ActionConfig | null
  status_config?: ActionConfig | null
}

export const gatesApi = {
  list: (wsId: string) =>
    api.get(`/workspaces/${wsId}/gates`).then((r) => normalizeList(r.data)),

  get: (wsId: string, gateId: string) =>
    api.get<Gate>(`/workspaces/${wsId}/gates/${gateId}`).then((r) => r.data),

  create: (wsId: string, params: CreateGateParams) =>
    api.post<Gate>(`/workspaces/${wsId}/gates`, params).then((r) => r.data),

  update: (wsId: string, gateId: string, params: UpdateGateParams) =>
    api.patch<Gate>(`/workspaces/${wsId}/gates/${gateId}`, params).then((r) => r.data),

  delete: (wsId: string, gateId: string) =>
    api.delete(`/workspaces/${wsId}/gates/${gateId}`),

  trigger: (wsId: string, gateId: string) =>
    api.post(`/workspaces/${wsId}/gates/${gateId}/trigger`, {}),
}
