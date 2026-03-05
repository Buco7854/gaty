import { notifications } from '@mantine/notifications'

type ApiError = { response?: { data?: { title?: string } } }

/** Show a red error notification, extracting the server's RFC-7807 `title` when available. */
export function notifyError(err: unknown, fallback = 'An error occurred') {
  const msg = (err as ApiError)?.response?.data?.title ?? fallback
  notifications.show({ color: 'red', message: msg, autoClose: 4000 })
}

/** Show a green success notification. */
export function notifySuccess(message: string) {
  notifications.show({ color: 'green', message, autoClose: 3000 })
}

/** Extract an API error message as a string (for pages that show errors inline). */
export function extractApiError(err: unknown, fallback: string): string {
  return (err as ApiError)?.response?.data?.title ?? fallback
}
