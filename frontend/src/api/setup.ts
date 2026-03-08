import axios from 'axios'
import { api } from '@/lib/api'
import type { User } from '@/types'

export const setupApi = {
  status: () =>
    axios.get<{ setup_required: boolean }>('/api/setup/status').then((r) => r.data),

  init: (email: string, password: string) =>
    api.post<{ user: User }>('/setup/init', { email, password }).then((r) => r.data),
}
