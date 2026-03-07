import { notifications } from '@mantine/notifications'

type ApiError = {
  response?: {
    data?: {
      title?: string
      detail?: string
      errors?: { message: string; location?: string }[]
    }
  }
}

/** Extract a human-readable message from a Huma RFC-7807 error response. */
function extractMessage(err: unknown, fallback: string): string {
  const data = (err as ApiError)?.response?.data
  if (!data) return fallback
  // Structured field errors: show the first (usually most specific) one.
  if (data.errors?.length) {
    const e = data.errors[0]
    return e.location ? `${e.location}: ${e.message}` : e.message
  }
  // detail is the human message; title is the generic HTTP status text — prefer detail.
  return data.detail ?? data.title ?? fallback
}

/** Show a red error notification, extracting the server's validation detail when available. */
export function notifyError(err: unknown, fallback = 'An error occurred') {
  notifications.show({ color: 'red', message: extractMessage(err, fallback), autoClose: 5000 })
}

/** Show a green success notification. */
export function notifySuccess(message: string) {
  notifications.show({ color: 'green', message, autoClose: 3000 })
}

/** Extract an API error message as a string (for pages that show errors inline). */
export function extractApiError(err: unknown, fallback: string): string {
  return extractMessage(err, fallback)
}
