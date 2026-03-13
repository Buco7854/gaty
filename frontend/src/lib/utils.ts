import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Reads a potentially nested value from an object using dot-notation keys.
 * e.g. getNestedValue(obj, "a.b.c") returns obj.a.b.c
 */
export function getNestedValue(obj: Record<string, unknown>, key: string): unknown {
  if (key in obj) return obj[key]
  if (!key.includes('.')) return undefined
  const parts = key.split('.')
  let current: unknown = obj
  for (const part of parts) {
    if (current == null || typeof current !== 'object') return undefined
    current = (current as Record<string, unknown>)[part]
  }
  return current
}

/**
 * Returns true if a nested key exists (value is not undefined) in the object.
 */
export function hasNestedKey(obj: Record<string, unknown>, key: string): boolean {
  return getNestedValue(obj, key) !== undefined
}

/**
 * Validates that a redirect path is safe (starts with / and is not a protocol-relative URL).
 */
export function isSafeRedirect(path: string | null): path is string {
  return !!path && path.startsWith('/') && !path.startsWith('//')
}
