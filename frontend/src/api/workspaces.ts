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

  create: (name: string, slug: string) =>
    api.post<Workspace>('/workspaces', { name, slug }).then((r) => r.data),

  updateSsoSettings: (wsId: string, settings: Record<string, string>) =>
    api.patch(`/workspaces/${wsId}/sso-settings`, settings).then((r) => r.data),

  getSsoSettings: (wsId: string) =>
    api.get(`/workspaces/${wsId}/sso-settings`).then((r) => r.data),
}
