import axios from 'axios'
import type { RefreshResponse } from '@/types'
import type { GateSession } from '@/api/public'
import { findLocalSession } from '@/utils/session'
import { useAuthStore } from '@/store/auth'

declare module 'axios' {
  interface InternalAxiosRequestConfig {
    _authMeta?: { type: 'global' | 'local'; wsId?: string; gateId?: string }
  }
}

export const api = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
})

// Attach token on every request: global first, then local member fallback.
api.interceptors.request.use((config) => {
  const globalToken = localStorage.getItem('access_token')
  if (globalToken) {
    config.headers.Authorization = `Bearer ${globalToken}`
    config._authMeta = { type: 'global' }
    return config
  }

  // Local member fallback: parse wsId from current URL and attach session token.
  const wsMatch = window.location.pathname.match(/\/workspaces\/([^/]+)/)
  if (wsMatch) {
    const session = findLocalSession(wsMatch[1])
    if (session?.access_token) {
      config.headers.Authorization = `Bearer ${session.access_token}`
      config._authMeta = { type: 'local', wsId: wsMatch[1], gateId: session.gateId }
      return config
    }
  }

  return config
})

// On 401: for global sessions try silent refresh; for local sessions clear and redirect.
let isRefreshing = false
let failQueue: Array<{ resolve: (t: string) => void; reject: (e: unknown) => void }> = []

function drainQueue(error: unknown, token?: string) {
  failQueue.forEach((p) => (token ? p.resolve(token) : p.reject(error)))
  failQueue = []
}

api.interceptors.response.use(
  (res) => res,
  async (error) => {
    const original = error.config
    if (error.response?.status !== 401 || original._retry) {
      return Promise.reject(error)
    }

    const { _authMeta } = original ?? {}

    // Local member session expired → clear session and redirect to member login.
    if (_authMeta?.type === 'local') {
      if (_authMeta.gateId) localStorage.removeItem(`gaty_session_${_authMeta.gateId}`)
      if (_authMeta.wsId) {
        const params = new URLSearchParams()
        if (_authMeta.gateId) params.set('gate_id', _authMeta.gateId)
        const currentPath = window.location.pathname
        if (currentPath !== `/workspaces/${_authMeta.wsId}/login`) params.set('redirect', currentPath)
        const loginUrl = `/workspaces/${_authMeta.wsId}/login?${params.toString()}`
        if (window.location.pathname !== `/workspaces/${_authMeta.wsId}/login`) window.location.href = loginUrl
      } else {
        if (window.location.pathname !== '/') window.location.href = '/'
      }
      return Promise.reject(error)
    }

    // Global session: try silent refresh.
    if (isRefreshing) {
      return new Promise((resolve, reject) => {
        failQueue.push({ resolve, reject })
      }).then((token) => {
        original.headers.Authorization = `Bearer ${token}`
        return api(original)
      })
    }

    original._retry = true
    isRefreshing = true

    const refreshToken = localStorage.getItem('refresh_token')
    if (!refreshToken) {
      isRefreshing = false
      drainQueue(error)
      redirectToLogin()
      return Promise.reject(error)
    }

    try {
      const { data } = await axios.post<{ access_token: string; refresh_token: string }>(
        '/api/auth/refresh',
        { refresh_token: refreshToken },
      )
      const tokens = data as RefreshResponse
      localStorage.setItem('access_token', tokens.access_token)
      localStorage.setItem('refresh_token', tokens.refresh_token)
      api.defaults.headers.common.Authorization = `Bearer ${tokens.access_token}`
      // Sync Zustand store so hooks using accessToken (e.g. SSE) reconnect with the fresh token.
      useAuthStore.getState().updateTokens(tokens.access_token, tokens.refresh_token)
      drainQueue(null, tokens.access_token)
      original.headers.Authorization = `Bearer ${tokens.access_token}`
      return api(original)
    } catch (refreshError) {
      drainQueue(refreshError)
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
      redirectToLogin()
      return Promise.reject(refreshError)
    } finally {
      isRefreshing = false
    }
  },
)

function redirectToLogin() {
  if (window.location.pathname !== '/login') {
    window.location.href = '/login'
  }
}
