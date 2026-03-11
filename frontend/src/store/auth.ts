import { create } from 'zustand'
import { api } from '@/lib/api'
import type { User, RefreshResponse, WorkspaceRole } from '@/types'

/** Session metadata populated from login/refresh response bodies (tokens are in HttpOnly cookies). */
interface SessionInfo {
  type: 'global' | 'local' | 'pin_session'
  // global
  user?: User
  // local
  membershipId?: string
  workspaceId?: string
  role?: WorkspaceRole
  displayName?: string
  // pin_session
  gateId?: string
  permissions?: string[]
}

interface AuthState {
  session: SessionInfo | null
  initializing: boolean
  setGlobalSession: (user: User) => void
  setLocalSession: (membershipId: string, workspaceId: string, role: WorkspaceRole, displayName?: string) => void
  setPinSession: (gateId: string, permissions: string[]) => void
  clearSession: () => void
  logout: () => Promise<void>
  isAuthenticated: () => boolean
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

  setGlobalSession(user) {
    const session: SessionInfo = { type: 'global', user }
    localStorage.setItem('gatie_session', JSON.stringify(session))
    set({ session })
  },

  setLocalSession(membershipId, workspaceId, role, displayName) {
    const session: SessionInfo = { type: 'local', membershipId, workspaceId, role, displayName }
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
    return get().session?.type === 'global'
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
    case 'global':
      if (!data.user) return null
      return { type: 'global', user: data.user }
    case 'local':
      return {
        type: 'local',
        membershipId: data.membership_id,
        workspaceId: data.workspace_id,
        role: data.role,
        displayName: data.display_name,
      }
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
