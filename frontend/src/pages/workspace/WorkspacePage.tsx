import { useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { Gate, WorkspaceWithRole } from '@/types'
import { Plus, DoorOpen, Wifi, WifiOff, HelpCircle, ChevronRight, Zap, Globe } from 'lucide-react'

function StatusDot({ status }: { status: Gate['status'] }) {
  const colors = {
    online: 'bg-green-500',
    offline: 'bg-red-500',
    unknown: 'bg-gray-400',
  }
  return (
    <span className="relative flex h-2.5 w-2.5">
      {status === 'online' && (
        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75" />
      )}
      <span className={`relative inline-flex rounded-full h-2.5 w-2.5 ${colors[status]}`} />
    </span>
  )
}

function StatusIcon({ status }: { status: Gate['status'] }) {
  if (status === 'online') return <Wifi className="w-4 h-4 text-green-600" />
  if (status === 'offline') return <WifiOff className="w-4 h-4 text-red-500" />
  return <HelpCircle className="w-4 h-4 text-gray-400" />
}

export default function WorkspacePage() {
  const { wsId } = useParams<{ wsId: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [gateName, setGateName] = useState('')
  const [triggeringId, setTriggeringId] = useState<string | null>(null)

  const ws = qc.getQueryData<WorkspaceWithRole[]>(['workspaces'])?.find((w) => w.id === wsId)

  const { data: gates, isLoading } = useQuery<Gate[]>({
    queryKey: ['gates', wsId],
    queryFn: () =>
      api.get(`/workspaces/${wsId}/gates`).then((r) => {
        const d = r.data as unknown
        if (Array.isArray(d)) return d as Gate[]
        return ((d as Record<string, unknown>).gates ?? []) as Gate[]
      }),
    refetchInterval: 10_000,
  })

  const createGate = useMutation({
    mutationFn: (body: { name: string; integration_type: string }) =>
      api.post(`/workspaces/${wsId}/gates`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['gates', wsId] })
      setShowCreate(false)
      setGateName('')
    },
  })

  const triggerGate = useMutation({
    mutationFn: (gateId: string) =>
      api.post(`/workspaces/${wsId}/gates/${gateId}/trigger`, {}),
    onMutate: (gateId) => setTriggeringId(gateId),
    onSettled: () => setTriggeringId(null),
  })

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-8 max-w-4xl">
        <div>
          <h1 className="text-2xl font-bold">{ws?.name ?? 'Workspace'}</h1>
          <p className="text-sm text-muted-foreground mt-0.5">Gate dashboard</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-1.5 bg-primary text-primary-foreground rounded-md px-3 py-2 text-sm font-medium hover:bg-primary/90 transition-colors"
        >
          <Plus className="w-4 h-4" />
          Add gate
        </button>
      </div>

      {showCreate && (
        <div className="mb-6 max-w-4xl rounded-lg border border-border p-4 bg-card">
          <h2 className="font-semibold mb-3">Add gate</h2>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              createGate.mutate({ name: gateName, integration_type: 'MQTT' })
            }}
            className="flex gap-2"
          >
            <input
              value={gateName}
              onChange={(e) => setGateName(e.target.value)}
              required
              className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
              placeholder="Parking entrance"
            />
            <button
              type="submit"
              disabled={createGate.isPending}
              className="bg-primary text-primary-foreground rounded-md px-3 py-2 text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
            >
              {createGate.isPending ? 'Adding…' : 'Add'}
            </button>
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="rounded-md px-3 py-2 text-sm hover:bg-accent transition-colors"
            >
              Cancel
            </button>
          </form>
        </div>
      )}

      {isLoading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 max-w-4xl">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-36 rounded-xl border border-border bg-muted/40 animate-pulse" />
          ))}
        </div>
      ) : gates?.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground max-w-4xl">
          <DoorOpen className="w-10 h-10 mx-auto mb-3 opacity-40" />
          <p className="font-medium">No gates yet</p>
          <p className="text-sm mt-0.5">Add your first gate to get started</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 max-w-4xl">
          {gates?.map((gate) => (
            <div
              key={gate.id}
              className="rounded-xl border border-border bg-card p-4 flex flex-col gap-3 hover:border-primary/30 transition-colors"
            >
              <div className="flex items-start justify-between gap-2">
                <div className="flex items-center gap-2 min-w-0">
                  <StatusDot status={gate.status} />
                  <span className="font-semibold truncate">{gate.name}</span>
                </div>
                <StatusIcon status={gate.status} />
              </div>

              <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <Globe className="w-3.5 h-3.5" />
                <span className="capitalize">{gate.integration_type.toLowerCase()}</span>
                <span>·</span>
                <span className="capitalize">{gate.status}</span>
              </div>

              <div className="flex gap-2 mt-auto">
                <button
                  onClick={() => triggerGate.mutate(gate.id)}
                  disabled={triggeringId === gate.id}
                  className="flex-1 flex items-center justify-center gap-1.5 bg-primary text-primary-foreground rounded-md py-1.5 text-xs font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
                >
                  <Zap className="w-3.5 h-3.5" />
                  {triggeringId === gate.id ? 'Opening…' : 'Open'}
                </button>
                <button
                  onClick={() => navigate(`/workspaces/${wsId}/gates/${gate.id}`)}
                  className="flex items-center justify-center gap-1 px-2.5 rounded-md border border-border hover:bg-accent text-xs transition-colors"
                >
                  <ChevronRight className="w-3.5 h-3.5" />
                  Details
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
