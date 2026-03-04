import { api } from '@/lib/api'
import type { DomainResolveResult } from '@/types'

export const publicApi = {
  resolve: (domain: string) =>
    api.get<DomainResolveResult>(`/public/resolve?domain=${encodeURIComponent(domain)}`).then((r) => r.data),

  unlock: (gateId: string, pin: string) =>
    api.post('/public/unlock', { gate_id: gateId, pin }),
}
