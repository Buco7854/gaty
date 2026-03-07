import { api } from '@/lib/api'
import type { AccessSchedule, ExprNode } from '@/types'

function normalizeList(data: unknown): AccessSchedule[] {
  if (Array.isArray(data)) return data as AccessSchedule[]
  return ((data as Record<string, unknown>).schedules ?? []) as AccessSchedule[]
}

type ScheduleParams = { name: string; description?: string; expr: ExprNode | null }

export const schedulesApi = {
  // Workspace-level schedules (admin-managed)
  list: (wsId: string, bearerToken?: string) =>
    api.get(`/workspaces/${wsId}/schedules`, bearerToken ? { headers: { Authorization: `Bearer ${bearerToken}` } } : undefined).then((r) => normalizeList(r.data)),

  create: (wsId: string, params: ScheduleParams) =>
    api.post<AccessSchedule>(`/workspaces/${wsId}/schedules`, params).then((r) => r.data),

  get: (wsId: string, scheduleId: string) =>
    api.get<AccessSchedule>(`/workspaces/${wsId}/schedules/${scheduleId}`).then((r) => r.data),

  update: (wsId: string, scheduleId: string, params: ScheduleParams) =>
    api.put<AccessSchedule>(`/workspaces/${wsId}/schedules/${scheduleId}`, params).then((r) => r.data),

  delete: (wsId: string, scheduleId: string) =>
    api.delete(`/workspaces/${wsId}/schedules/${scheduleId}`),

  // Member personal schedules
  listMine: (wsId: string) =>
    api.get(`/workspaces/${wsId}/my-schedules`).then((r) => normalizeList(r.data)),

  createMine: (wsId: string, params: ScheduleParams) =>
    api.post<AccessSchedule>(`/workspaces/${wsId}/my-schedules`, params).then((r) => r.data),

  getMine: (wsId: string, scheduleId: string) =>
    api.get<AccessSchedule>(`/workspaces/${wsId}/my-schedules/${scheduleId}`).then((r) => r.data),

  updateMine: (wsId: string, scheduleId: string, params: ScheduleParams) =>
    api.put<AccessSchedule>(`/workspaces/${wsId}/my-schedules/${scheduleId}`, params).then((r) => r.data),

  deleteMine: (wsId: string, scheduleId: string) =>
    api.delete(`/workspaces/${wsId}/my-schedules/${scheduleId}`),
}
