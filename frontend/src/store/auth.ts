import { create } from 'zustand'
import { api } from '@/lib/api'
import type { Member, RefreshResponse } from '@/types'

/** Session metadata populated from login/refresh response bodies (tokens are in HttpOnly cookies). */
interface SessionInfo {
  type: 'member' | 'pin_session'
  // member
  member?: Member
  // pin_session
  gateId?: string
  permissions?: string[]
}

interface AuthState {
  session: SessionInfo | null
  initializing: boolean
  setMemberSession: (member: Member) => void
  setPinSession: (gateId: string, permissions: string[]) => void
  clearSession: () => void
  logout: () => Promise<void>
  isAuthenticated: () => boolean
  isAdmin: () => boolean
  hydrate: () => Promise<void>
}

// Guard against double-invocation in React StrictMode (dev) which would
// consume the one-time-use refresh token twice, failing on the second call.
let hydratePromise: Promise<void> | null = null

export const useAuthStore = create<AuthState>((set, get) => ({
  session: (() => {
    try {
      const raw = localStorage.getItem('gatie_session')
      return raw ? (JSON.parse(raw) as SessionInfo) : null
    } catch {
      return null
    }
  })(),
  initializing: true,

  setMemberSession(member) {
    const session: SessionInfo = { type: 'member', member }
    localStorage.setItem('gatie_session', JSON.stringify(session))
    set({ session })
  },

  setPinSession(gateId, permissions) {
    const session: SessionInfo = { type: 'pin_session', gateId, permissions }
    localStorage.setItem('gatie_session', JSON.stringify(session))
    set({ session })
  },

  clearSession() {
    localStorage.removeItem('gatie_session')
    set({ session: null })
  },

  async logout() {
    try { await api.post('/auth/logout') } catch { /* ignore */ }
    localStorage.removeItem('gatie_session')
    set({ session: null })
  },

  isAuthenticated() {
    return get().session?.type === 'member'
  },

  isAdmin() {
    const s = get().session
    return s?.type === 'member' && s.member?.role === 'ADMIN'
  },

  hydrate() {
    if (hydratePromise) return hydratePromise
    hydratePromise = (async () => {
      try {
        const { data } = await api.post<RefreshResponse>('/auth/refresh')
        const session = refreshResponseToSession(data)
        if (session) {
          localStorage.setItem('gatie_session', JSON.stringify(session))
          set({ session, initializing: false })
        } else {
          localStorage.removeItem('gatie_session')
          set({ session: null, initializing: false })
        }
      } catch {
        localStorage.removeItem('gatie_session')
        set({ session: null, initializing: false })
      }
    })()
    return hydratePromise
  },
}))

function refreshResponseToSession(data: RefreshResponse): SessionInfo | null {
  switch (data.type) {
    case 'member':
      if (!data.member) return null
      return { type: 'member', member: data.member }
    case 'pin_session':
      return {
        type: 'pin_session',
        gateId: data.gate_id,
        permissions: data.permissions,
      }
    default:
      return null
  }
}
