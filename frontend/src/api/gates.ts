import { api } from '@/lib/api'
import type { Gate, MetaField, StatusRule, StatusTransition } from '@/types'

function normalizeList(data: unknown): Gate[] {
  if (Array.isArray(data)) return data as Gate[]
  return ((data as Record<string, unknown>).gates ?? []) as Gate[]
}

export type ActionDriverType = 'MQTT_GATIE' | 'MQTT_CUSTOM' | 'HTTP' | 'HTTP_INBOUND' | 'HTTP_WEBHOOK' | 'NONE'

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
  meta_config?: MetaField[]
  status_rules?: StatusRule[]
  custom_statuses?: string[]
}

export interface UpdateGateParams {
  name?: string
  open_config?: ActionConfig | null
  close_config?: ActionConfig | null
  status_config?: ActionConfig | null
  meta_config?: MetaField[]
  status_rules?: StatusRule[]
  custom_statuses?: string[]
  ttl_seconds?: number | null
  status_transitions?: StatusTransition[]
}

export interface GateTokenResponse {
  gate_id: string
  gate_token: string
}

export const gatesApi = {
  list: () =>
    api.get('/gates').then((r) => normalizeList(r.data)),

  get: (gateId: string) =>
    api.get<Gate>(`/gates/${gateId}`).then((r) => r.data),

  create: (params: CreateGateParams) =>
    api.post<Gate>('/gates', params).then((r) => r.data),

  update: (gateId: string, params: UpdateGateParams) =>
    api.patch<Gate>(`/gates/${gateId}`, params).then((r) => r.data),

  delete: (gateId: string) =>
    api.delete(`/gates/${gateId}`),

  trigger: (gateId: string, action: 'open' | 'close' = 'open') =>
    api.post(`/gates/${gateId}/trigger`, { action }),

  getToken: (gateId: string) =>
    api.get<GateTokenResponse>(`/gates/${gateId}/token`).then((r) => r.data),

  rotateToken: (gateId: string) =>
    api
      .post<GateTokenResponse>(`/gates/${gateId}/token/rotate`, {})
      .then((r) => r.data),
}
