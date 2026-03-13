import { api } from '@/lib/api'
import type { AuthResponse, Member } from '@/types'

export const authApi = {
  login: (username: string, password: string) =>
    api.post<AuthResponse>('/auth/login', { username, password }).then((r) => r.data),

  me: () =>
    api.get<Member>('/auth/me').then((r) => r.data),
}
