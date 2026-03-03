import { useState } from 'react'
import { useParams } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { WorkspaceMembership } from '@/types'
import { UserPlus, Trash2, Users, Shield } from 'lucide-react'

const ROLE_BADGE: Record<string, string> = {
  OWNER: 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300',
  ADMIN: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300',
  MEMBER: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
}

export default function MembersPage() {
  const { wsId } = useParams<{ wsId: string }>()
  const qc = useQueryClient()
  const [tab, setTab] = useState<'invite' | 'create'>('invite')
  const [showForm, setShowForm] = useState(false)
  const [email, setEmail] = useState('')
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('MEMBER')
  const [error, setError] = useState<string | null>(null)

  const { data: members, isLoading } = useQuery<WorkspaceMembership[]>({
    queryKey: ['members', wsId],
    queryFn: () =>
      api.get(`/workspaces/${wsId}/members`).then((r) => {
        const d = r.data as unknown
        if (Array.isArray(d)) return d as WorkspaceMembership[]
        return ((d as Record<string, unknown>).members ?? []) as WorkspaceMembership[]
      }),
  })

  const invite = useMutation({
    mutationFn: (body: { email: string; role: string }) =>
      api.post(`/workspaces/${wsId}/members/invite`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['members', wsId] })
      setShowForm(false); setEmail(''); setError(null)
    },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setError(msg ?? 'Failed to invite')
    },
  })

  const createLocal = useMutation({
    mutationFn: (body: { local_username: string; display_name?: string; password: string; role: string }) =>
      api.post(`/workspaces/${wsId}/members`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['members', wsId] })
      setShowForm(false); setUsername(''); setDisplayName(''); setPassword(''); setError(null)
    },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setError(msg ?? 'Failed to create')
    },
  })

  const deleteMember = useMutation({
    mutationFn: (memberId: string) => api.delete(`/workspaces/${wsId}/members/${memberId}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['members', wsId] }),
  })

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (tab === 'invite') {
      invite.mutate({ email, role })
    } else {
      createLocal.mutate({ local_username: username, display_name: displayName || undefined, password, role })
    }
  }

  return (
    <div className="p-8 max-w-3xl">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold">Members</h1>
          <p className="text-sm text-muted-foreground mt-0.5">Manage workspace access</p>
        </div>
        <button
          onClick={() => setShowForm(true)}
          className="flex items-center gap-1.5 bg-primary text-primary-foreground rounded-md px-3 py-2 text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          <UserPlus className="w-4 h-4" />
          Add member
        </button>
      </div>

      {showForm && (
        <div className="mb-6 rounded-lg border border-border p-4 bg-card space-y-4">
          <div className="flex gap-2 text-sm">
            <button
              className={`px-3 py-1 rounded-md font-medium transition-colors ${tab === 'invite' ? 'bg-primary text-primary-foreground' : 'hover:bg-accent'}`}
              onClick={() => setTab('invite')}
            >
              Invite by email
            </button>
            <button
              className={`px-3 py-1 rounded-md font-medium transition-colors ${tab === 'create' ? 'bg-primary text-primary-foreground' : 'hover:bg-accent'}`}
              onClick={() => setTab('create')}
            >
              Create local member
            </button>
          </div>
          <form onSubmit={handleSubmit} className="space-y-3">
            {tab === 'invite' ? (
              <input
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required type="email"
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
                placeholder="user@example.com"
              />
            ) : (
              <>
                <input
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
                  placeholder="Username (local login)"
                />
                <input
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
                  placeholder="Display name (optional)"
                />
                <input
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required type="password" minLength={8}
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
                  placeholder="Password"
                />
              </>
            )}
            <select
              value={role}
              onChange={(e) => setRole(e.target.value)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
            >
              <option value="MEMBER">Member</option>
              <option value="ADMIN">Admin</option>
            </select>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div className="flex gap-2">
              <button
                type="submit"
                disabled={invite.isPending || createLocal.isPending}
                className="bg-primary text-primary-foreground rounded-md px-3 py-1.5 text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
              >
                {invite.isPending || createLocal.isPending ? 'Adding…' : 'Add'}
              </button>
              <button type="button" onClick={() => setShowForm(false)} className="rounded-md px-3 py-1.5 text-sm hover:bg-accent transition-colors">
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {isLoading ? (
        <div className="space-y-2">
          {[0, 1, 2].map((i) => <div key={i} className="h-14 rounded-lg border border-border bg-muted/40 animate-pulse" />)}
        </div>
      ) : members?.length === 0 ? (
        <div className="text-center py-12 text-muted-foreground">
          <Users className="w-8 h-8 mx-auto mb-2 opacity-40" />
          <p className="font-medium text-sm">No members yet</p>
        </div>
      ) : (
        <div className="space-y-1">
          {members?.map((m) => (
            <div key={m.id} className="flex items-center justify-between p-3 rounded-lg hover:bg-accent/40 transition-colors">
              <div className="flex items-center gap-3 min-w-0">
                <div className="w-8 h-8 rounded-full bg-muted flex items-center justify-center shrink-0">
                  <Shield className="w-3.5 h-3.5 text-muted-foreground" />
                </div>
                <div className="min-w-0">
                  <p className="text-sm font-medium truncate">
                    {m.display_name ?? m.local_username ?? `User ${m.id.slice(0, 8)}`}
                  </p>
                  {m.local_username && (
                    <p className="text-xs text-muted-foreground font-mono">{m.local_username}</p>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${ROLE_BADGE[m.role]}`}>
                  {m.role}
                </span>
                {m.role !== 'OWNER' && (
                  <button
                    onClick={() => deleteMember.mutate(m.id)}
                    className="p-1 rounded text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
