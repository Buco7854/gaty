import type { GateSession } from '@/api/public'

export function decodeJWTPayload(token: string): Record<string, unknown> | null {
  try {
    const b64 = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    return JSON.parse(atob(b64))
  } catch {
    return null
  }
}

export function getRoleFromJWT(token: string): string | null {
  return (decodeJWTPayload(token)?.role as string) ?? null
}

/** Returns the `permissions` array embedded in a JWT (used for PIN session JWTs). */
export function getPermissionsFromJWT(token: string): string[] {
  const perms = decodeJWTPayload(token)?.permissions
  if (Array.isArray(perms)) return perms as string[]
  return []
}

/** Scans localStorage for a gaty_session that matches the given workspace ID. */
export function findLocalSession(wsId: string): (GateSession & { gateId: string; role: string }) | null {
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    if (!key?.startsWith('gaty_session_')) continue
    try {
      const s = JSON.parse(localStorage.getItem(key)!) as GateSession
      if (s?.type === 'member' && s?.workspace_id === wsId && s?.access_token) {
        const role = getRoleFromJWT(s.access_token) ?? 'MEMBER'
        return { ...s, gateId: key.slice('gaty_session_'.length), role }
      }
    } catch { /* ignore */ }
  }
  return null
}
