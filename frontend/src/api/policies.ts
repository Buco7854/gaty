import { api } from '@/lib/api'
import type { AccessSchedule, MembershipPolicy } from '@/types'

function normalizeList(data: unknown): MembershipPolicy[] {
  if (Array.isArray(data)) return data as MembershipPolicy[]
  return ((data as Record<string, unknown>).policies ?? []) as MembershipPolicy[]
}

export const policiesApi = {
  list: (wsId: string, gateId: string) =>
    api.get(`/workspaces/${wsId}/gates/${gateId}/policies`).then((r) => normalizeList(r.data)),

  grant: (wsId: string, gateId: string, membershipId: string, permissionCode: string) =>
    api.post(`/workspaces/${wsId}/gates/${gateId}/policies`, { membership_id: membershipId, permission_code: permissionCode }),

  revoke: (wsId: string, gateId: string, membershipId: string, permissionCode: string) =>
    api.delete(`/workspaces/${wsId}/gates/${gateId}/policies/${membershipId}/${encodeURIComponent(permissionCode)}`),

  listByMembership: (wsId: string, membershipId: string) =>
    api.get(`/workspaces/${wsId}/members/${membershipId}/policies`).then((r) => normalizeList(r.data)),

  listMine: (wsId: string) =>
    api.get(`/workspaces/${wsId}/policies/me`).then((r) => normalizeList(r.data)),

  getMemberGateSchedule: (wsId: string, gateId: string, membershipId: string) =>
    api.get<AccessSchedule | null>(`/workspaces/${wsId}/gates/${gateId}/policies/${membershipId}/schedule`).then((r) => r.data),

  setMemberGateSchedule: (wsId: string, gateId: string, membershipId: string, scheduleId: string) =>
    api.put(`/workspaces/${wsId}/gates/${gateId}/policies/${membershipId}/schedule`, { schedule_id: scheduleId }),

  removeMemberGateSchedule: (wsId: string, gateId: string, membershipId: string) =>
    api.delete(`/workspaces/${wsId}/gates/${gateId}/policies/${membershipId}/schedule`),
}