import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { gatesApi, publicApi } from '@/api'
import type { Gate } from '@/types'
import { useTranslation } from 'react-i18next'
import { GatePermissionsGrid, useGatePermissions } from '@/components/GatePermissionsGrid'
import { KeyRound, Save, CheckCircle2, Info, Loader2 } from 'lucide-react'
import { QueryError } from '@/components/QueryError'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'

export default function SettingsPage() {
  const qc = useQueryClient()
  const { t } = useTranslation()
  const gatePermissions = useGatePermissions()
  const [macSaved, setMacSaved] = useState(false)

  const { data: ssoProviders = [], isLoading: ssoLoading } = useQuery({
    queryKey: ['sso-providers'],
    queryFn: () => publicApi.ssoProviders().catch(() => []),
  })

  const { data: macData, isLoading: macLoading, isError: macError, error: macFetchError } = useQuery({
    queryKey: ['member-auth-config'],
    queryFn: () => api.get<Record<string, unknown>>('/auth/sso/settings').then((r) => r.data),
  })

  const { data: gatesData } = useQuery({
    queryKey: ['gates'],
    queryFn: () => gatesApi.list(),
  })
  const gates = (gatesData ?? []) as Gate[]

  const [passwordAuth, setPasswordAuth] = useState<boolean>(true)
  const [ssoAuth, setSsoAuth] = useState<boolean>(false)
  const [apiTokenAuth, setApiTokenAuth] = useState<boolean>(false)
  const [apiTokenMax, setApiTokenMax] = useState<number>(5)
  const [sessionDuration, setSessionDuration] = useState<number | string>('')
  const [defaultGatePerms, setDefaultGatePerms] = useState<Record<string, Set<string>>>({})

  const [macInitialized, setMacInitialized] = useState(false)
  if (macData && !macInitialized) {
    setPasswordAuth((macData.password as boolean) ?? true)
    setSsoAuth((macData.sso as boolean) ?? false)
    setApiTokenAuth((macData.api_token as boolean) ?? false)
    setApiTokenMax((macData.api_token_max as number) ?? 5)
    if (macData.session_duration != null) setSessionDuration(macData.session_duration as number)
    const dgp = macData.default_gate_permissions
    if (Array.isArray(dgp)) {
      const map: Record<string, Set<string>> = {}
      for (const entry of dgp as { gate_id: string; permissions: string[] }[]) {
        if (entry.gate_id) map[entry.gate_id] = new Set(entry.permissions ?? [])
      }
      setDefaultGatePerms(map)
    }
    setMacInitialized(true)
  }

  const updateMAC = useMutation({
    mutationFn: (body: Record<string, unknown>) => api.put('/auth/sso/settings', body).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['member-auth-config'] })
      setMacSaved(true)
      setTimeout(() => setMacSaved(false), 2000)
    },
  })

  function handleSaveMemberAuth() {
    const dur = typeof sessionDuration === 'number' ? sessionDuration : (sessionDuration === '' ? null : parseInt(String(sessionDuration), 10))
    const defaultGatePermsList = Object.entries(defaultGatePerms)
      .filter(([, perms]) => perms.size > 0)
      .map(([gate_id, perms]) => ({ gate_id, permissions: Array.from(perms) }))
    updateMAC.mutate({
      password: passwordAuth,
      sso: ssoAuth,
      api_token: apiTokenAuth,
      api_token_max: apiTokenMax,
      session_duration: dur,
      default_gate_permissions: defaultGatePermsList,
    })
  }

  if (macLoading || ssoLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (macError) {
    return (
      <div className="max-w-xl mx-auto p-6">
        <QueryError error={macFetchError} />
      </div>
    )
  }

  return (
    <div className="max-w-xl mx-auto p-6 space-y-6">
      <div>
        <h2 className="text-xl font-bold">{t('settings.title')}</h2>
        <p className="text-sm text-muted-foreground">{t('settings.subtitle')}</p>
      </div>

      {/* SSO Providers — read-only */}
      <div className="border rounded-lg p-5 space-y-4">
        <div className="flex items-center gap-2">
          <KeyRound className="h-4 w-4 opacity-60" />
          <span className="font-semibold">{t('settings.sso')}</span>
        </div>

        <Alert>
          <Info className="h-4 w-4" />
          <AlertDescription>{t('settings.ssoEnvConfigured')}</AlertDescription>
        </Alert>

        {ssoProviders.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('settings.noProviders')}</p>
        ) : (
          <div className="space-y-2">
            {ssoProviders.map((p) => (
              <div key={p.id} className="border rounded-md p-3 flex items-center gap-2">
                <span className="text-sm font-medium">{p.name}</span>
                <Badge variant="secondary">{p.type.toUpperCase()}</Badge>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Member auth defaults */}
      <div className="border rounded-lg p-5 space-y-4">
        <div>
          <p className="font-semibold">{t('settings.memberAuthDefaults')}</p>
          <p className="text-xs text-muted-foreground">{t('settings.memberAuthDefaultsHint')}</p>
        </div>

        <div className="space-y-4">
          <Switch
            label={t('settings.passwordAuth')}
            checked={passwordAuth}
            onCheckedChange={setPasswordAuth}
          />
          <Switch
            label={t('settings.ssoAuth')}
            checked={ssoAuth}
            onCheckedChange={setSsoAuth}
          />
          <Switch
            label={t('settings.apiTokenAuth')}
            checked={apiTokenAuth}
            onCheckedChange={setApiTokenAuth}
          />
          {apiTokenAuth && (
            <Input
              label={t('settings.apiTokenMax')}
              type="number"
              value={String(apiTokenMax)}
              onChange={(e) => setApiTokenMax(Number(e.target.value) || 5)}
              min={1}
              max={100}
              className="w-28"
            />
          )}
          <div>
            <Input
              label={t('settings.memberSessionDuration')}
              description={t('settings.memberSessionDurationHint')}
              type="number"
              value={String(sessionDuration)}
              onChange={(e) => setSessionDuration(e.target.value === '' ? '' : Number(e.target.value))}
              min={0}
              step={3600}
              placeholder={t('settings.memberSessionDurationPlaceholder')}
              className="w-48"
            />
          </div>
        </div>
      </div>

      {/* Default member permissions */}
      <div className="border rounded-lg p-5 space-y-4">
        <div>
          <p className="font-semibold">{t('settings.defaultMemberPermissions')}</p>
          <p className="text-xs text-muted-foreground">{t('settings.defaultMemberPermissionsHint')}</p>
        </div>

        {gates.length === 0 ? (
          <p className="text-xs text-muted-foreground italic">{t('settings.noGatesForDefaults')}</p>
        ) : (
          <GatePermissionsGrid
            gates={gates}
            permissions={gatePermissions}
            isChecked={(gateId, code) => (defaultGatePerms[gateId] ?? new Set()).has(code)}
            onToggle={(gateId, code) => {
              setDefaultGatePerms((prev) => {
                const next = { ...prev }
                const set = new Set(next[gateId] ?? [])
                if (set.has(code)) set.delete(code)
                else set.add(code)
                next[gateId] = set
                return next
              })
            }}
          />
        )}

        {macSaved && (
          <Alert variant="success">
            <CheckCircle2 className="h-4 w-4" />
            <AlertDescription>{t('settings.saved')}</AlertDescription>
          </Alert>
        )}

        <Button onClick={handleSaveMemberAuth} loading={updateMAC.isPending}>
          <Save className="h-4 w-4" />
          {t('settings.saveSso')}
        </Button>
      </div>
    </div>
  )
}
