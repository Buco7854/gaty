import axios from 'axios'
import { api } from '@/lib/api'
import type { Member } from '@/types'

export const setupApi = {
  status: () =>
    axios.get<{ setup_required: boolean }>('/api/setup/status').then((r) => r.data),

  init: (username: string, password: string) =>
    api.post<{ member: Member }>('/setup/init', { username, password }).then((r) => r.data),
}
