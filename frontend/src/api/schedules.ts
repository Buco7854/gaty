import { api } from '@/lib/api'
import type { AccessSchedule, ExprNode } from '@/types'

function normalizeList(data: unknown): AccessSchedule[] {
  if (Array.isArray(data)) return data as AccessSchedule[]
  return ((data as Record<string, unknown>).schedules ?? []) as AccessSchedule[]
}

type ScheduleParams = { name: string; description?: string; expr: ExprNode | null }

export const schedulesApi = {
  // Admin-managed schedules
  list: () =>
    api.get('/schedules').then((r) => normalizeList(r.data)),

  create: (params: ScheduleParams) =>
    api.post<AccessSchedule>('/schedules', params).then((r) => r.data),

  get: (scheduleId: string) =>
    api.get<AccessSchedule>(`/schedules/${scheduleId}`).then((r) => r.data),

  update: (scheduleId: string, params: ScheduleParams) =>
    api.put<AccessSchedule>(`/schedules/${scheduleId}`, params).then((r) => r.data),

  delete: (scheduleId: string) =>
    api.delete(`/schedules/${scheduleId}`),

  // Member personal schedules
  listMine: () =>
    api.get('/members/me/schedules').then((r) => normalizeList(r.data)),

  createMine: (params: ScheduleParams) =>
    api.post<AccessSchedule>('/members/me/schedules', params).then((r) => r.data),

  getMine: (scheduleId: string) =>
    api.get<AccessSchedule>(`/members/me/schedules/${scheduleId}`).then((r) => r.data),

  updateMine: (scheduleId: string, params: ScheduleParams) =>
    api.put<AccessSchedule>(`/members/me/schedules/${scheduleId}`, params).then((r) => r.data),

  deleteMine: (scheduleId: string) =>
    api.delete(`/members/me/schedules/${scheduleId}`),
}
