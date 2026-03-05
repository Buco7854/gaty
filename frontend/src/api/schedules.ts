import { api } from '@/lib/api'
import type { AccessSchedule, ScheduleRule } from '@/types'

function normalizeList(data: unknown): AccessSchedule[] {
  if (Array.isArray(data)) return data as AccessSchedule[]
  return ((data as Record<string, unknown>).schedules ?? []) as AccessSchedule[]
}

export const schedulesApi = {
  list: (wsId: string) =>
    api.get(`/workspaces/${wsId}/schedules`).then((r) => normalizeList(r.data)),

  create: (wsId: string, params: { name: string; description?: string; rules: ScheduleRule[] }) =>
    api.post<AccessSchedule>(`/workspaces/${wsId}/schedules`, params).then((r) => r.data),

  get: (wsId: string, scheduleId: string) =>
    api.get<AccessSchedule>(`/workspaces/${wsId}/schedules/${scheduleId}`).then((r) => r.data),

  update: (wsId: string, scheduleId: string, params: { name: string; description?: string; rules: ScheduleRule[] }) =>
    api.put<AccessSchedule>(`/workspaces/${wsId}/schedules/${scheduleId}`, params).then((r) => r.data),

  delete: (wsId: string, scheduleId: string) =>
    api.delete(`/workspaces/${wsId}/schedules/${scheduleId}`),
}
