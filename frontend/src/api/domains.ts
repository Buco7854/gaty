import { api } from '@/lib/api'
import type { CustomDomain } from '@/types'

function normalizeList(data: unknown): CustomDomain[] {
  if (Array.isArray(data)) return data as CustomDomain[]
  return ((data as Record<string, unknown>).domains ?? []) as CustomDomain[]
}

export const domainsApi = {
  list: (wsId: string, gateId: string) =>
    api.get(`/workspaces/${wsId}/gates/${gateId}/domains`)
      .then((r) => normalizeList(r.data)),

  create: (wsId: string, gateId: string, domain: string) =>
    api.post<CustomDomain>(`/workspaces/${wsId}/gates/${gateId}/domains`, { domain }).then((r) => r.data),

  verify: (wsId: string, gateId: string, domainId: string) =>
    api.post<{ verified: boolean; message?: string }>(`/workspaces/${wsId}/gates/${gateId}/domains/${domainId}/verify`, {}).then((r) => r.data),

  delete: (wsId: string, gateId: string, domainId: string) =>
    api.delete(`/workspaces/${wsId}/gates/${gateId}/domains/${domainId}`),
}