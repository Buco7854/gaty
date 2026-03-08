import axios from 'axios'
import { useAuthStore } from '@/store/auth'

export const api = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
  withCredentials: true,
})

// On 401: try silent refresh once, then redirect to login.
let isRefreshing = false
let failQueue: Array<{ resolve: () => void; reject: (e: unknown) => void }> = []

function drainQueue(error: unknown) {
  failQueue.forEach((p) => (error ? p.reject(error) : p.resolve()))
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
      return new Promise<void>((resolve, reject) => {
        failQueue.push({ resolve, reject })
      }).then(() => api(original))
    }

    original._retry = true
    isRefreshing = true

    try {
      await axios.post('/api/auth/refresh', null, { withCredentials: true })
      drainQueue(null)
      return api(original)
    } catch (refreshError) {
      drainQueue(refreshError)
      useAuthStore.getState().clearSession()
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
