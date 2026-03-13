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

interface PolicyInput {
  gate_id: string
  permission_code: string
}

export interface MyEffectiveAuthConfig {
  api_token: boolean
}

/** Self-service credential management: calls /api/auth/me/* */
export const credentialsApi = {
  listTokens: (): Promise<MemberCredential[]> =>
    api
      .get<MemberCredential[]>('/auth/me/credentials')
      .then((r) => (Array.isArray(r.data) ? r.data : []))
      .then((data) => data.filter((c) => c.type === 'API_TOKEN')),

  createToken: (
    label: string,
    expiresAt?: string,
    policies?: PolicyInput[],
    scheduleId?: string,
  ): Promise<CreatedToken> =>
    api
      .post<CreatedToken>('/auth/me/api-tokens', {
        label,
        ...(expiresAt ? { expires_at: expiresAt + 'T23:59:59Z' } : {}),
        ...(policies?.length ? { policies } : {}),
        ...(scheduleId ? { schedule_id: scheduleId } : {}),
      })
      .then((r) => r.data),

  deleteToken: (credId: string): Promise<void> =>
    api.delete(`/auth/me/credentials/${credId}`).then(() => {}),
}
