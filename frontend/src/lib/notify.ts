import { toast } from 'sonner'

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
  if (data.errors?.length) {
    const e = data.errors[0]
    return e.location ? `${e.location}: ${e.message}` : e.message
  }
  return data.detail ?? data.title ?? fallback
}

/** Show an error toast, extracting the server's validation detail when available. */
export function notifyError(err: unknown, fallback = 'An error occurred') {
  toast.error(extractMessage(err, fallback))
}

/** Show a success toast. */
export function notifySuccess(message: string) {
  toast.success(message)
}

/** Extract an API error message as a string (for pages that show errors inline). */
export function extractApiError(err: unknown, fallback: string): string {
  return extractMessage(err, fallback)
}
