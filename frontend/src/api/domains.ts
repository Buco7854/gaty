import { api } from '@/lib/api'
import type { CustomDomain } from '@/types'

function normalizeList(data: unknown): CustomDomain[] {
  if (Array.isArray(data)) return data as CustomDomain[]
  return ((data as Record<string, unknown>).domains ?? []) as CustomDomain[]
}

export const domainsApi = {
  list: (gateId: string) =>
    api.get(`/gates/${gateId}/domains`)
      .then((r) => normalizeList(r.data)),

  create: (gateId: string, domain: string) =>
    api.post<CustomDomain>(`/gates/${gateId}/domains`, { domain }).then((r) => r.data),

  verify: (gateId: string, domainId: string) =>
    api.post<{ verified: boolean; message?: string }>(`/gates/${gateId}/domains/${domainId}/verify`, {}).then((r) => r.data),

  delete: (gateId: string, domainId: string) =>
    api.delete(`/gates/${gateId}/domains/${domainId}`),
}
