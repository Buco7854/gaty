import { useEffect, useState } from 'react'
import { useParams } from 'react-router'
import { api } from '@/lib/api'
import type { DomainResolveResult } from '@/types'
import { Delete, Loader2, CheckCircle2, XCircle } from 'lucide-react'

type PadState = 'idle' | 'loading' | 'success' | 'error'

const MAX_PIN = 12
const DIGITS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '', '0', '⌫']

export default function PinPadPage() {
  const { gateId: gateIdParam } = useParams<{ gateId?: string }>()

  const [resolving, setResolving] = useState(!gateIdParam)
  const [resolved, setResolved] = useState<DomainResolveResult | null>(null)
  const [resolveError, setResolveError] = useState(false)

  const [pin, setPin] = useState('')
  const [state, setState] = useState<PadState>('idle')
  const [errorMsg, setErrorMsg] = useState('')

  // Resolve domain → gate (only when no gateId in URL)
  useEffect(() => {
    if (gateIdParam) return
    const domain = window.location.hostname
    api.get<DomainResolveResult>(`/public/resolve?domain=${encodeURIComponent(domain)}`)
      .then((r) => setResolved(r.data))
      .catch(() => setResolveError(true))
      .finally(() => setResolving(false))
  }, [gateIdParam])

  const effectiveGateId = gateIdParam ?? resolved?.gate_id

  async function submit(finalPin: string) {
    if (!effectiveGateId || finalPin.length < 4) return
    setState('loading')
    try {
      await api.post('/public/unlock', { gate_id: effectiveGateId, pin: finalPin })
      setState('success')
      setTimeout(() => { setState('idle'); setPin('') }, 3000)
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status === 429) setErrorMsg('Too many attempts. Please wait.')
      else if (status === 403) setErrorMsg('Invalid PIN or time restriction')
      else setErrorMsg('Unable to reach the gate')
      setState('error')
      setTimeout(() => { setState('idle'); setErrorMsg(''); setPin('') }, 3000)
    }
  }

  function press(d: string) {
    if (state !== 'idle') return
    if (d === '⌫') {
      setPin((p) => p.slice(0, -1))
    } else if (d === '') {
      return
    } else {
      const next = pin + d
      setPin(next)
      if (next.length === MAX_PIN) submit(next)
    }
  }

  if (resolving) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (resolveError) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center bg-background p-8 text-center">
        <XCircle className="w-12 h-12 text-destructive mb-4" />
        <h1 className="text-xl font-bold">Domain not configured</h1>
        <p className="text-sm text-muted-foreground mt-2">
          This domain is not linked to any gate.
        </p>
      </div>
    )
  }

  const gateName = resolved?.gate_name ?? 'Gate'
  const workspaceName = resolved?.workspace_name

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-background p-4 select-none">
      {/* Header */}
      <div className="text-center mb-8 space-y-1">
        {workspaceName && (
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-widest">{workspaceName}</p>
        )}
        <h1 className="text-2xl font-bold">{gateName}</h1>
        <p className="text-sm text-muted-foreground">Enter PIN to open</p>
      </div>

      {/* PIN display */}
      <div className="flex gap-2 mb-8 h-10 items-center">
        {state === 'success' ? (
          <CheckCircle2 className="w-10 h-10 text-green-500" />
        ) : state === 'error' ? (
          <XCircle className="w-10 h-10 text-destructive" />
        ) : state === 'loading' ? (
          <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
        ) : (
          Array.from({ length: Math.max(pin.length, 4) }).map((_, i) => (
            <div
              key={i}
              className={`w-3 h-3 rounded-full transition-colors ${
                i < pin.length ? 'bg-primary' : 'bg-muted'
              }`}
            />
          ))
        )}
      </div>

      {/* Feedback text */}
      {(state === 'success' || state === 'error') && (
        <p className={`text-sm mb-6 font-medium ${state === 'success' ? 'text-green-600' : 'text-destructive'}`}>
          {state === 'success' ? 'Gate opened!' : errorMsg}
        </p>
      )}

      {/* Numpad */}
      <div className="grid grid-cols-3 gap-3 w-full max-w-xs">
        {DIGITS.map((d, i) => (
          d === '' ? (
            <div key={i} />
          ) : d === '⌫' ? (
            <button
              key={i}
              onPointerDown={() => press(d)}
              disabled={state !== 'idle' || pin.length === 0}
              className="aspect-square rounded-2xl flex items-center justify-center text-muted-foreground text-xl bg-muted hover:bg-muted/70 active:scale-95 disabled:opacity-30 transition-all"
            >
              <Delete className="w-5 h-5" />
            </button>
          ) : (
            <button
              key={i}
              onPointerDown={() => press(d)}
              disabled={state !== 'idle' || pin.length >= MAX_PIN}
              className="aspect-square rounded-2xl flex items-center justify-center text-2xl font-semibold bg-secondary hover:bg-secondary/70 active:scale-95 disabled:opacity-30 transition-all shadow-sm"
            >
              {d}
            </button>
          )
        ))}
      </div>

      {pin.length > 0 && pin.length < MAX_PIN && state === 'idle' && (
        <button
          onClick={() => submit(pin)}
          className="mt-6 bg-primary text-primary-foreground rounded-xl px-8 py-3 text-sm font-semibold hover:bg-primary/90 transition-colors"
        >
          Confirm
        </button>
      )}
    </div>
  )
}
