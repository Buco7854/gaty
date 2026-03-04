import { api } from '@/lib/api'
import type { AuthResponse, RefreshResponse, User } from '@/types'

export const authApi = {
  login: (email: string, password: string) =>
    api.post<AuthResponse>('/auth/login', { email, password }).then((r) => r.data),

  loginLocal: (workspaceId: string, localUsername: string, password: string) =>
    api.post<AuthResponse>('/auth/login/local', { workspace_id: workspaceId, local_username: localUsername, password }).then((r) => r.data),

  register: (email: string, password: string) =>
    api.post<AuthResponse>('/auth/register', { email, password }).then((r) => r.data),

  refresh: (refreshToken: string) =>
    api.post<RefreshResponse>('/auth/refresh', { refresh_token: refreshToken }).then((r) => r.data),

  me: () =>
    api.get<User>('/auth/me').then((r) => r.data),

  merge: (workspaceId: string, localUsername: string, localPassword: string) =>
    api.post('/auth/merge', { workspace_id: workspaceId, local_username: localUsername, local_password: localPassword }).then((r) => r.data),
}
