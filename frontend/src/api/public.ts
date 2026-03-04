import axios from 'axios'
import { api } from '@/lib/api'
import type { DomainResolveResult } from '@/types'

export const publicApi = {
  resolve: (domain: string) =>
    api.get<DomainResolveResult>(`/public/resolve?domain=${encodeURIComponent(domain)}`).then((r) => r.data),

  unlock: (gateId: string, pin: string) =>
    api.post('/public/unlock', { gate_id: gateId, pin }),

  // Trigger gate as a local member — bypasses the global-token interceptor
  triggerAsLocal: (workspaceId: string, gateId: string, localToken: string) =>
    axios.post(
      `/api/workspaces/${workspaceId}/gates/${gateId}/trigger`,
      {},
      { headers: { Authorization: `Bearer ${localToken}` } },
    ),
}
