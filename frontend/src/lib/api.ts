import axios from 'axios'
import type { RefreshResponse } from '@/types'

export const api = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
})

// Attach Bearer token from localStorage on every request.
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// On 401: try silent refresh, then retry. On failure, redirect to /login.
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
