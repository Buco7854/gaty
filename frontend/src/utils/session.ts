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
