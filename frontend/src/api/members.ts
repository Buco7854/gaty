import { api } from '@/lib/api'
import type { Member } from '@/types'

function normalizeList(data: unknown): Member[] {
  if (Array.isArray(data)) return data as Member[]
  return ((data as Record<string, unknown>).members ?? []) as Member[]
}

export const membersApi = {
  list: () =>
    api.get('/members').then((r) => normalizeList(r.data)),

  get: (memberId: string) =>
    api.get<Member>(`/members/${memberId}`).then((r) => r.data),

  create: (params: {
    username: string
    display_name?: string
    password: string
    role: string
  }) =>
    api.post<Member>('/members', params).then((r) => r.data),

  update: (memberId: string, params: {
    role?: string
    display_name?: string
    username?: string
    auth_config?: Record<string, unknown>
  }) =>
    api.patch<Member>(`/members/${memberId}`, params).then((r) => r.data),

  delete: (memberId: string) =>
    api.delete(`/members/${memberId}`),
}
