import { api } from '@/lib/api'
import type { AccessSchedule, ExprNode } from '@/types'

function normalizeList(data: unknown): AccessSchedule[] {
  if (Array.isArray(data)) return data as AccessSchedule[]
  return ((data as Record<string, unknown>).schedules ?? []) as AccessSchedule[]
}

type ScheduleParams = { name: string; description?: string; expr: ExprNode | null }

export const schedulesApi = {
  // Workspace-level schedules (admin-managed)
  list: (wsId: string) =>
    api.get(`/workspaces/${wsId}/schedules`).then((r) => normalizeList(r.data)),

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
    api.get(`/workspaces/${wsId}/members/me/schedules`).then((r) => normalizeList(r.data)),

  createMine: (wsId: string, params: ScheduleParams) =>
    api.post<AccessSchedule>(`/workspaces/${wsId}/members/me/schedules`, params).then((r) => r.data),

  getMine: (wsId: string, scheduleId: string) =>
    api.get<AccessSchedule>(`/workspaces/${wsId}/members/me/schedules/${scheduleId}`).then((r) => r.data),

  updateMine: (wsId: string, scheduleId: string, params: ScheduleParams) =>
    api.put<AccessSchedule>(`/workspaces/${wsId}/members/me/schedules/${scheduleId}`, params).then((r) => r.data),

  deleteMine: (wsId: string, scheduleId: string) =>
    api.delete(`/workspaces/${wsId}/members/me/schedules/${scheduleId}`),
}
