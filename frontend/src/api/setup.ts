import axios from 'axios'
import { api } from '@/lib/api'

export const setupApi = {
  status: () =>
    axios.get<{ setup_required: boolean }>('/api/setup/status').then((r) => r.data),

  init: (email: string, password: string) =>
    api.post<{ access_token: string; refresh_token: string }>('/setup/init', { email, password }).then((r) => r.data),
}
