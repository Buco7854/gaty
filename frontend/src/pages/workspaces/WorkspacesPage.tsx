import { useState } from 'react'
import { useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { WorkspaceWithRole } from '@/types'
import { Plus, Building2, ChevronRight } from 'lucide-react'

export default function WorkspacesPage() {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [error, setError] = useState<string | null>(null)

  const { data: workspaces, isLoading } = useQuery<WorkspaceWithRole[]>({
    queryKey: ['workspaces'],
    queryFn: () =>
      api.get('/workspaces').then((r) => {
        const d = r.data as unknown
        if (Array.isArray(d)) return d as WorkspaceWithRole[]
        return ((d as Record<string, unknown>).workspaces ?? []) as WorkspaceWithRole[]
      }),
  })

  const create = useMutation({
    mutationFn: (body: { name: string; slug: string }) => api.post('/workspaces', body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['workspaces'] })
      setShowCreate(false)
      setName('')
      setSlug('')
    },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setError(msg ?? 'Failed to create workspace')
    },
  })

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    create.mutate({ name, slug })
  }

  function autoSlug(n: string) {
    return n.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')
  }

  return (
    <div className="p-8 max-w-3xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold">Workspaces</h1>
          <p className="text-sm text-muted-foreground mt-0.5">Select a workspace to manage your gates</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-1.5 bg-primary text-primary-foreground rounded-md px-3 py-2 text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          <Plus className="w-4 h-4" />
          New workspace
        </button>
      </div>

      {showCreate && (
        <div className="mb-6 rounded-lg border border-border p-4 bg-card">
          <h2 className="font-semibold mb-3">Create workspace</h2>
          <form onSubmit={handleCreate} className="space-y-3">
            <div className="space-y-1.5">
              <label className="text-sm font-medium">Name</label>
              <input
                value={name}
                onChange={(e) => { setName(e.target.value); if (!slug) setSlug(autoSlug(e.target.value)) }}
                required
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
                placeholder="My Building"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">Slug</label>
              <input
                value={slug}
                onChange={(e) => setSlug(autoSlug(e.target.value))}
                required
                pattern="[a-z0-9-]+"
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow font-mono"
                placeholder="my-building"
              />
              <p className="text-xs text-muted-foreground">Lowercase letters, numbers, hyphens</p>
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div className="flex gap-2 pt-1">
              <button
                type="submit"
                disabled={create.isPending}
                className="bg-primary text-primary-foreground rounded-md px-3 py-1.5 text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
              >
                {create.isPending ? 'Creating…' : 'Create'}
              </button>
              <button
                type="button"
                onClick={() => { setShowCreate(false); setError(null) }}
                className="rounded-md px-3 py-1.5 text-sm hover:bg-accent transition-colors"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {isLoading ? (
        <div className="space-y-2">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-16 rounded-lg border border-border bg-muted/40 animate-pulse" />
          ))}
        </div>
      ) : workspaces?.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground">
          <Building2 className="w-10 h-10 mx-auto mb-3 opacity-40" />
          <p className="font-medium">No workspaces yet</p>
          <p className="text-sm mt-0.5">Create one to get started</p>
        </div>
      ) : (
        <div className="space-y-2">
          {workspaces?.map((ws) => (
            <button
              key={ws.id}
              onClick={() => navigate(`/workspaces/${ws.id}`)}
              className="w-full flex items-center gap-4 p-4 rounded-lg border border-border hover:border-primary/40 hover:bg-accent/30 transition-all text-left group"
            >
              <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
                <span className="font-bold text-primary uppercase">{ws.name[0]}</span>
              </div>
              <div className="flex-1 min-w-0">
                <p className="font-semibold truncate">{ws.name}</p>
                <p className="text-xs text-muted-foreground font-mono">{ws.slug} · {ws.role}</p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground group-hover:text-foreground transition-colors" />
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
