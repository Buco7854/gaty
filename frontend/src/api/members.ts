import { api } from '@/lib/api'
import type { WorkspaceMembership } from '@/types'

function normalizeList(data: unknown): WorkspaceMembership[] {
  if (Array.isArray(data)) return data as WorkspaceMembership[]
  return ((data as Record<string, unknown>).members ?? []) as WorkspaceMembership[]
}

export const membersApi = {
  list: (wsId: string) =>
    api.get(`/workspaces/${wsId}/members`).then((r) => normalizeList(r.data)),

  get: (wsId: string, memberId: string) =>
    api.get<WorkspaceMembership>(`/workspaces/${wsId}/members/${memberId}`).then((r) => r.data),

  invite: (wsId: string, email: string, role: string) =>
    api.post<WorkspaceMembership>(`/workspaces/${wsId}/members/invite`, { email, role }).then((r) => r.data),

  createLocal: (wsId: string, params: { local_username: string; display_name?: string; password: string; role: string }) =>
    api.post<WorkspaceMembership>(`/workspaces/${wsId}/members`, params).then((r) => r.data),

  updateRole: (wsId: string, memberId: string, role: string) =>
    api.patch<WorkspaceMembership>(`/workspaces/${wsId}/members/${memberId}`, { role }).then((r) => r.data),

  delete: (wsId: string, memberId: string) =>
    api.delete(`/workspaces/${wsId}/members/${memberId}`),
}
