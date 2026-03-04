import { api } from '@/lib/api'
import type { Workspace, WorkspaceWithRole } from '@/types'

function normalizeList(data: unknown): WorkspaceWithRole[] {
  if (Array.isArray(data)) return data as WorkspaceWithRole[]
  return ((data as Record<string, unknown>).workspaces ?? []) as WorkspaceWithRole[]
}

export const workspacesApi = {
  list: () =>
    api.get('/workspaces').then((r) => normalizeList(r.data)),

  get: (wsId: string) =>
    api.get<Workspace>(`/workspaces/${wsId}`).then((r) => r.data),

  create: (name: string) =>
    api.post<Workspace>('/workspaces', { name }).then((r) => r.data),

  updateSsoSettings: (wsId: string, settings: Record<string, unknown>) =>
    api.patch(`/workspaces/${wsId}/sso-settings`, settings).then((r) => r.data),

  getSsoSettings: (wsId: string) =>
    api.get(`/workspaces/${wsId}/sso-settings`).then((r) => r.data),

  getMemberAuthConfig: (wsId: string) =>
    api.get<Record<string, unknown>>(`/workspaces/${wsId}/member-auth-config`).then((r) => r.data),

  updateMemberAuthConfig: (wsId: string, config: Record<string, unknown>) =>
    api.patch<Record<string, unknown>>(`/workspaces/${wsId}/member-auth-config`, config).then((r) => r.data),
}
