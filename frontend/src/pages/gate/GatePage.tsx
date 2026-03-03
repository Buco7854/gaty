import { useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { Gate, GatePin, CustomDomain, WorkspaceMembership, MembershipPolicy } from '@/types'
import {
  ArrowLeft, Zap, Hash, Globe, Plus, Trash2,
  CheckCircle2, XCircle, Clock, Copy, Check, Shield, Users
} from 'lucide-react'

const PERMISSIONS = [
  { code: 'gate:read_status', label: 'View status' },
  { code: 'gate:trigger_open', label: 'Open' },
  { code: 'gate:manage', label: 'Manage' },
] as const

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      onClick={() => { navigator.clipboard.writeText(text); setCopied(true); setTimeout(() => setCopied(false), 2000) }}
      className="p-1 rounded text-muted-foreground hover:text-foreground transition-colors"
      title="Copy"
    >
      {copied ? <Check className="w-3.5 h-3.5 text-green-500" /> : <Copy className="w-3.5 h-3.5" />}
    </button>
  )
}

export default function GatePage() {
  const { wsId, gateId } = useParams<{ wsId: string; gateId: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()

  // PIN state
  const [showPinForm, setShowPinForm] = useState(false)
  const [pinLabel, setPinLabel] = useState('')
  const [pinValue, setPinValue] = useState('')

  // Domain state
  const [showDomainForm, setShowDomainForm] = useState(false)
  const [domainValue, setDomainValue] = useState('')
  const [verifyResult, setVerifyResult] = useState<Record<string, { verified: boolean; message?: string }>>({})

  const { data: gate } = useQuery<Gate>({
    queryKey: ['gate', wsId, gateId],
    queryFn: () => api.get(`/workspaces/${wsId}/gates/${gateId}`).then((r) => r.data as Gate),
    refetchInterval: 10_000,
  })

  const { data: pins } = useQuery<GatePin[]>({
    queryKey: ['pins', wsId, gateId],
    queryFn: () =>
      api.get(`/workspaces/${wsId}/gates/${gateId}/pins`).then((r) => {
        const d = r.data as unknown
        if (Array.isArray(d)) return d as GatePin[]
        return ((d as Record<string, unknown>).pins ?? []) as GatePin[]
      }),
  })

  const { data: domains } = useQuery<CustomDomain[]>({
    queryKey: ['domains', wsId, gateId],
    queryFn: () =>
      api.get(`/workspaces/${wsId}/gates/${gateId}/domains`).then((r) => {
        const d = r.data as unknown
        if (Array.isArray(d)) return d as CustomDomain[]
        return ((d as Record<string, unknown>).domains ?? []) as CustomDomain[]
      }),
  })

  const trigger = useMutation({
    mutationFn: () => api.post(`/workspaces/${wsId}/gates/${gateId}/trigger`, {}),
  })

  const createPin = useMutation({
    mutationFn: (body: { label?: string; pin: string }) =>
      api.post(`/workspaces/${wsId}/gates/${gateId}/pins`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pins', wsId, gateId] })
      setShowPinForm(false); setPinLabel(''); setPinValue('')
    },
  })

  const deletePin = useMutation({
    mutationFn: (pinId: string) => api.delete(`/workspaces/${wsId}/gates/${gateId}/pins/${pinId}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['pins', wsId, gateId] }),
  })

  const addDomain = useMutation({
    mutationFn: (domain: string) =>
      api.post(`/workspaces/${wsId}/gates/${gateId}/domains`, { domain }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['domains', wsId, gateId] })
      setShowDomainForm(false); setDomainValue('')
    },
  })

  const deleteDomain = useMutation({
    mutationFn: (domainId: string) =>
      api.delete(`/workspaces/${wsId}/gates/${gateId}/domains/${domainId}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['domains', wsId, gateId] }),
  })

  const verifyDomain = useMutation({
    mutationFn: (domainId: string) =>
      api.post(`/workspaces/${wsId}/gates/${gateId}/domains/${domainId}/verify`, {}),
    onSuccess: (res, domainId) => {
      const data = res.data as { verified: boolean; message?: string }
      setVerifyResult((prev) => ({ ...prev, [domainId]: data }))
      if (data.verified) qc.invalidateQueries({ queryKey: ['domains', wsId, gateId] })
    },
  })

  // Permissions
  const { data: members } = useQuery<WorkspaceMembership[]>({
    queryKey: ['members', wsId],
    queryFn: () =>
      api.get(`/workspaces/${wsId}/members`).then((r) => {
        const d = r.data as unknown
        if (Array.isArray(d)) return d as WorkspaceMembership[]
        return ((d as Record<string, unknown>).members ?? []) as WorkspaceMembership[]
      }),
  })

  const { data: policies } = useQuery<MembershipPolicy[]>({
    queryKey: ['policies', wsId, gateId],
    queryFn: () =>
      api.get(`/workspaces/${wsId}/gates/${gateId}/policies`).then((r) => {
        const d = r.data as unknown
        if (Array.isArray(d)) return d as MembershipPolicy[]
        return []
      }),
  })

  const grantPerm = useMutation({
    mutationFn: ({ membershipId, permCode }: { membershipId: string; permCode: string }) =>
      api.post(`/workspaces/${wsId}/gates/${gateId}/policies`, {
        membership_id: membershipId,
        permission_code: permCode,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['policies', wsId, gateId] }),
  })

  const revokePerm = useMutation({
    mutationFn: ({ membershipId, permCode }: { membershipId: string; permCode: string }) =>
      api.delete(`/workspaces/${wsId}/gates/${gateId}/policies/${membershipId}/${permCode}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['policies', wsId, gateId] }),
  })

  const regularMembers = members?.filter((m) => m.role === 'MEMBER') ?? []

  function hasPermission(membershipId: string, permCode: string) {
    return policies?.some((p) => p.membership_id === membershipId && p.permission_code === permCode) ?? false
  }

  function togglePermission(membershipId: string, permCode: string) {
    if (hasPermission(membershipId, permCode)) {
      revokePerm.mutate({ membershipId, permCode })
    } else {
      grantPerm.mutate({ membershipId, permCode })
    }
  }

  const statusColor = {
    online: 'text-green-600 bg-green-50 dark:bg-green-900/20',
    offline: 'text-red-600 bg-red-50 dark:bg-red-900/20',
    unknown: 'text-gray-500 bg-gray-50 dark:bg-gray-800',
  }[gate?.status ?? 'unknown']

  return (
    <div className="p-8 max-w-3xl space-y-8">
      {/* Header */}
      <div>
        <button
          onClick={() => navigate(`/workspaces/${wsId}`)}
          className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-3 transition-colors"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          Back to dashboard
        </button>
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold">{gate?.name ?? '…'}</h1>
            {gate && (
              <span className={`inline-block mt-1 text-xs font-medium px-2 py-0.5 rounded-full ${statusColor}`}>
                {gate.status}
              </span>
            )}
          </div>
          <button
            onClick={() => trigger.mutate()}
            disabled={trigger.isPending}
            className="flex items-center gap-1.5 bg-primary text-primary-foreground rounded-md px-4 py-2 text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors"
          >
            <Zap className="w-4 h-4" />
            {trigger.isPending ? 'Opening…' : 'Open gate'}
          </button>
        </div>
      </div>

      {/* PIN codes */}
      <section>
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Hash className="w-4 h-4 text-muted-foreground" />
            <h2 className="font-semibold">PIN codes</h2>
            <span className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded-full">{pins?.length ?? 0}</span>
          </div>
          <button
            onClick={() => setShowPinForm(true)}
            className="flex items-center gap-1 text-sm hover:bg-accent rounded-md px-2 py-1 transition-colors"
          >
            <Plus className="w-3.5 h-3.5" />
            Add PIN
          </button>
        </div>

        {showPinForm && (
          <form
            onSubmit={(e) => { e.preventDefault(); createPin.mutate({ label: pinLabel || undefined, pin: pinValue }) }}
            className="mb-3 p-3 rounded-lg border border-border bg-card flex gap-2"
          >
            <input
              value={pinLabel}
              onChange={(e) => setPinLabel(e.target.value)}
              className="flex-1 rounded-md border border-input bg-background px-3 py-1.5 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow"
              placeholder="Label (optional)"
            />
            <input
              value={pinValue}
              onChange={(e) => setPinValue(e.target.value)}
              required type="password" minLength={4}
              className="w-28 rounded-md border border-input bg-background px-3 py-1.5 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow font-mono"
              placeholder="PIN"
            />
            <button type="submit" disabled={createPin.isPending} className="bg-primary text-primary-foreground rounded-md px-3 py-1.5 text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors">
              {createPin.isPending ? '…' : 'Add'}
            </button>
            <button type="button" onClick={() => setShowPinForm(false)} className="rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors">✕</button>
          </form>
        )}

        <div className="space-y-1">
          {pins?.length === 0 && <p className="text-sm text-muted-foreground py-2">No PIN codes yet</p>}
          {pins?.map((pin) => (
            <div key={pin.id} className="flex items-center justify-between px-3 py-2 rounded-lg hover:bg-accent/40 transition-colors">
              <div className="flex items-center gap-2">
                <Hash className="w-3.5 h-3.5 text-muted-foreground" />
                <span className="text-sm">{pin.label ?? <span className="text-muted-foreground italic">Unlabeled</span>}</span>
                {(pin.metadata as { expires_at?: string }).expires_at && (
                  <span className="flex items-center gap-1 text-xs text-muted-foreground">
                    <Clock className="w-3 h-3" />
                    {new Date((pin.metadata as { expires_at: string }).expires_at).toLocaleDateString()}
                  </span>
                )}
              </div>
              <button onClick={() => deletePin.mutate(pin.id)} className="p-1 rounded text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors">
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            </div>
          ))}
        </div>
      </section>

      {/* Custom domains */}
      <section>
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Globe className="w-4 h-4 text-muted-foreground" />
            <h2 className="font-semibold">Custom domains</h2>
            <span className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded-full">{domains?.length ?? 0}</span>
          </div>
          <button onClick={() => setShowDomainForm(true)} className="flex items-center gap-1 text-sm hover:bg-accent rounded-md px-2 py-1 transition-colors">
            <Plus className="w-3.5 h-3.5" />
            Add domain
          </button>
        </div>

        {showDomainForm && (
          <form
            onSubmit={(e) => { e.preventDefault(); addDomain.mutate(domainValue) }}
            className="mb-3 p-3 rounded-lg border border-border bg-card flex gap-2"
          >
            <input
              value={domainValue}
              onChange={(e) => setDomainValue(e.target.value)}
              required
              className="flex-1 rounded-md border border-input bg-background px-3 py-1.5 text-sm outline-none focus:ring-2 focus:ring-ring transition-shadow font-mono"
              placeholder="gate.example.com"
            />
            <button type="submit" disabled={addDomain.isPending} className="bg-primary text-primary-foreground rounded-md px-3 py-1.5 text-sm font-medium hover:bg-primary/90 disabled:opacity-50 transition-colors">
              {addDomain.isPending ? '…' : 'Add'}
            </button>
            <button type="button" onClick={() => setShowDomainForm(false)} className="rounded-md px-2 py-1.5 text-sm hover:bg-accent transition-colors">✕</button>
          </form>
        )}

        <div className="space-y-2">
          {domains?.length === 0 && <p className="text-sm text-muted-foreground py-2">No custom domains yet</p>}
          {domains?.map((d) => (
            <div key={d.id} className="rounded-lg border border-border bg-card p-3 space-y-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  {d.verified_at
                    ? <CheckCircle2 className="w-4 h-4 text-green-500 shrink-0" />
                    : <XCircle className="w-4 h-4 text-amber-500 shrink-0" />
                  }
                  <span className="font-mono text-sm">{d.domain}</span>
                </div>
                <div className="flex items-center gap-1">
                  {!d.verified_at && (
                    <button
                      onClick={() => verifyDomain.mutate(d.id)}
                      disabled={verifyDomain.isPending}
                      className="text-xs px-2 py-1 rounded-md bg-amber-100 text-amber-800 hover:bg-amber-200 dark:bg-amber-900/30 dark:text-amber-300 transition-colors"
                    >
                      Verify DNS
                    </button>
                  )}
                  <button onClick={() => deleteDomain.mutate(d.id)} className="p-1 rounded text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors">
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>

              {!d.verified_at && (
                <div className="text-xs bg-muted rounded-md p-2 space-y-1">
                  <p className="text-muted-foreground">Add a DNS TXT record to verify ownership:</p>
                  <div className="flex items-center gap-1 font-mono">
                    <span className="text-foreground">_gaty.{d.domain}</span>
                    <span className="text-muted-foreground mx-1">→</span>
                    <span className="text-foreground break-all">{d.dns_challenge_token}</span>
                    <CopyButton text={d.dns_challenge_token} />
                  </div>
                  {verifyResult[d.id] && !verifyResult[d.id].verified && (
                    <p className="text-destructive">{verifyResult[d.id].message}</p>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      </section>

      {/* Permissions */}
      <section>
        <div className="flex items-center gap-2 mb-3">
          <Users className="w-4 h-4 text-muted-foreground" />
          <h2 className="font-semibold">Member permissions</h2>
          <span className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded-full">{regularMembers.length}</span>
        </div>

        {regularMembers.length === 0 ? (
          <p className="text-sm text-muted-foreground py-2">No regular members in this workspace</p>
        ) : (
          <div className="rounded-lg border border-border overflow-hidden">
            {/* Header row */}
            <div className="grid bg-muted/50 border-b border-border px-3 py-2 text-xs font-medium text-muted-foreground"
              style={{ gridTemplateColumns: '1fr repeat(3, auto)' }}>
              <span>Member</span>
              {PERMISSIONS.map((p) => (
                <span key={p.code} className="text-center w-20">{p.label}</span>
              ))}
            </div>
            {/* Member rows */}
            {regularMembers.map((m) => (
              <div
                key={m.id}
                className="grid items-center px-3 py-2.5 border-b border-border last:border-0 hover:bg-accent/20 transition-colors"
                style={{ gridTemplateColumns: '1fr repeat(3, auto)' }}
              >
                <div className="flex items-center gap-2 min-w-0">
                  <div className="w-6 h-6 rounded-full bg-muted flex items-center justify-center shrink-0">
                    <Shield className="w-3 h-3 text-muted-foreground" />
                  </div>
                  <span className="text-sm truncate">
                    {m.display_name ?? m.local_username ?? `Member ${m.id.slice(0, 8)}`}
                  </span>
                </div>
                {PERMISSIONS.map((p) => {
                  const checked = hasPermission(m.id, p.code)
                  return (
                    <div key={p.code} className="w-20 flex justify-center">
                      <button
                        onClick={() => togglePermission(m.id, p.code)}
                        disabled={grantPerm.isPending || revokePerm.isPending}
                        className={`w-5 h-5 rounded flex items-center justify-center border transition-colors disabled:opacity-40 ${
                          checked
                            ? 'bg-primary border-primary text-primary-foreground'
                            : 'border-input bg-background hover:bg-accent'
                        }`}
                        title={checked ? `Revoke "${p.label}"` : `Grant "${p.label}"`}
                      >
                        {checked && <Check className="w-3 h-3" />}
                      </button>
                    </div>
                  )
                })}
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
