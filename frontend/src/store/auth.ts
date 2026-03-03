import { create } from 'zustand'
import type { User } from '@/types'

interface AuthState {
  user: User | null
  accessToken: string | null
  setAuth: (user: User, accessToken: string, refreshToken: string) => void
  updateTokens: (accessToken: string, refreshToken: string) => void
  logout: () => void
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: (() => {
    try {
      const raw = localStorage.getItem('user')
      return raw ? (JSON.parse(raw) as User) : null
    } catch {
      return null
    }
  })(),
  accessToken: localStorage.getItem('access_token'),

  setAuth(user, accessToken, refreshToken) {
    localStorage.setItem('access_token', accessToken)
    localStorage.setItem('refresh_token', refreshToken)
    localStorage.setItem('user', JSON.stringify(user))
    set({ user, accessToken })
  },

  updateTokens(accessToken, refreshToken) {
    localStorage.setItem('access_token', accessToken)
    localStorage.setItem('refresh_token', refreshToken)
    set({ accessToken })
  },

  logout() {
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
    localStorage.removeItem('user')
    set({ user: null, accessToken: null })
  },

  isAuthenticated() {
    return !!get().accessToken
  },
}))
