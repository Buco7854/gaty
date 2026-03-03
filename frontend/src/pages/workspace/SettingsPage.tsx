import { useState } from 'react'
import { useParams } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { WorkspaceWithRole } from '@/types'
import { Save, KeyRound } from 'lucide-react'

export default function SettingsPage() {
  const { wsId } = useParams<{ wsId: string }>()
  const qc = useQueryClient()
  const [saved, setSaved] = useState(false)

  const ws = qc.getQueryData<WorkspaceWithRole[]>(['workspaces'])?.find((w) => w.id === wsId)

  const sso = (ws?.sso_settings ?? {}) as Record<string, string>
  const [issuer, setIssuer] = useState(sso.issuer ?? '')
  const [clientId, setClientId] = useState(sso.client_id ?? '')
  const [clientSecret, setClientSecret] = useState('')
  const [provider, setProvider] = useState(sso.provider ?? 'oidc')

  const updateSSO = useMutation({
    mutationFn: (body: Record<string, string>) =>
      api.patch(`/workspaces/${wsId}/sso-settings`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['workspaces'] })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    },
  })

  function handleSSOSubmit(e: React.FormEvent) {
    e.preventDefault()
    const body: Record<string, string> = { provider, issuer, client_id: clientId }
    if (clientSecret) body.client_secret = clientSecret
    updateSSO.mutate(body)
  }

  return (
    <div className="p-8 max-w-2xl">
      <div className="mb-8">
        <h1 className="text-2xl font-bold">Settings</h1>
        <p className="text-sm text-muted-foreground mt-0.5">Workspace configuration</p>
      </div>

      {/* SSO */}
      <section className="rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-2 mb-4">
          <KeyRound className="w-5 h-5 text-muted-foreground" />
          <h2 className="font-semibold">Single Sign-On (OIDC)</h2>
        </div>
        <form onSubmit={handleSSOSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Provider</label>
            <select
              value={provider}
              onChange={(e) => setProvider(e.target.value)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
            >
              <option value="oidc">OIDC</option>
            </select>
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Issuer URL</label>
            <input
              value={issuer}
              onChange={(e) => setIssuer(e.target.value)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
              placeholder="https://accounts.google.com"
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Client ID</label>
            <input
              value={clientId}
              onChange={(e) => setClientId(e.target.value)}
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
              placeholder="your-client-id"
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium">Client Secret</label>
            <input
              value={clientSecret}
              onChange={(e) => setClientSecret(e.target.value)}
              type="password"
              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
              placeholder="Leave blank to keep existing"
            />
          </div>
          <button
            type="submit"
            disabled={updateSSO.isPending}
            className="flex items-center gap-1.5 bg-primary text-primary-foreground rounded-md px-3 py-2 text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
          >
            <Save className="w-4 h-4" />
            {saved ? 'Saved!' : updateSSO.isPending ? 'Saving…' : 'Save SSO settings'}
          </button>
        </form>
      </section>
    </div>
  )
}
