import { api } from '@/lib/api'
import type { AccessSchedule, MemberPolicy } from '@/types'

function normalizeList(data: unknown): MemberPolicy[] {
  if (Array.isArray(data)) return data as MemberPolicy[]
  return ((data as Record<string, unknown>).policies ?? []) as MemberPolicy[]
}

export const policiesApi = {
  list: (gateId: string) =>
    api.get(`/gates/${gateId}/policies`).then((r) => normalizeList(r.data)),

  grant: (gateId: string, memberId: string, permissionCode: string) =>
    api.post(`/gates/${gateId}/policies`, { member_id: memberId, permission_code: permissionCode }),

  revoke: (gateId: string, memberId: string, permissionCode: string) =>
    api.delete(`/gates/${gateId}/policies/${memberId}/${encodeURIComponent(permissionCode)}`),

  listByMember: (memberId: string) =>
    api.get(`/members/${memberId}/policies`).then((r) => normalizeList(r.data)),

  listMine: () =>
    api.get('/policies/me').then((r) => normalizeList(r.data)),

  getMemberGateSchedule: (gateId: string, memberId: string) =>
    api.get<AccessSchedule | null>(`/gates/${gateId}/policies/${memberId}/schedule`).then((r) => r.data),

  setMemberGateSchedule: (gateId: string, memberId: string, scheduleId: string) =>
    api.put(`/gates/${gateId}/policies/${memberId}/schedule`, { schedule_id: scheduleId }),

  removeMemberGateSchedule: (gateId: string, memberId: string) =>
    api.delete(`/gates/${gateId}/policies/${memberId}/schedule`),
}
