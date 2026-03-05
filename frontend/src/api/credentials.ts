import { api } from '@/lib/api'

export interface MemberCredential {
  id: string
  type: 'API_TOKEN' | 'SSO_IDENTITY' | 'PASSWORD'
  label?: string
  expires_at?: string
  metadata?: Record<string, unknown>
  created_at: string
}

export interface CreatedToken extends MemberCredential {
  token: string
}

export const credentialsApi = {
  adminList: (wsId: string, memberId: string): Promise<MemberCredential[]> =>
    api
      .get<MemberCredential[]>(`/workspaces/${wsId}/members/${memberId}/credentials`)
      .then((r) => (Array.isArray(r.data) ? r.data : [])),

  adminCreateToken: (
    wsId: string,
    memberId: string,
    params: { label: string; expires_at?: string }
  ): Promise<CreatedToken> =>
    api
      .post<CreatedToken>(`/workspaces/${wsId}/members/${memberId}/api-tokens`, params)
      .then((r) => r.data),

  adminDelete: (wsId: string, memberId: string, credId: string): Promise<void> =>
    api.delete(`/workspaces/${wsId}/members/${memberId}/credentials/${credId}`).then(() => {}),
}
