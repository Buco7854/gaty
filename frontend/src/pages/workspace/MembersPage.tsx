import { useState } from 'react'
import { useQuery, useMutation, useQueryClient, useQueries } from '@tanstack/react-query'
import { membersApi, gatesApi, policiesApi, schedulesApi } from '@/api'
import type { Member, Gate, MemberPolicy, AccessSchedule } from '@/types'
import { useTranslation } from 'react-i18next'
import { GatePermissionsGrid, useGatePermissions } from '@/components/GatePermissionsGrid'
import { UserPlus, Trash2, Users, AlertCircle, Settings2, Pencil } from 'lucide-react'
import { notifySuccess, notifyError, extractApiError } from '@/lib/notify'
import { QueryError } from '@/components/QueryError'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { SimpleSelect } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { SimpleTooltip } from '@/components/ui/tooltip'

const ROLE_VARIANT: Record<string, 'default' | 'secondary'> = {
  ADMIN: 'default',
  MEMBER: 'secondary',
}

const AUTH_METHODS = [
  { key: 'password', labelKey: 'settings.passwordAuth' },
  { key: 'sso', labelKey: 'settings.ssoAuth' },
  { key: 'api_token', labelKey: 'settings.apiTokenAuth' },
] as const

// ---------- Schedule tab for member dialog ----------

function MemberSchedulesTab({
  member,
  gates,
  schedules,
}: {
  member: Member
  gates: Gate[]
  schedules: AccessSchedule[]
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()

  const scheduleQueries = useQueries({
    queries: gates.map((gate) => ({
      queryKey: ['member-gate-schedule', gate.id, member.id],
      queryFn: async () => {
        try {
          return await policiesApi.getMemberGateSchedule(gate.id, member.id)
        } catch (e: unknown) {
          if ((e as { response?: { status?: number } })?.response?.status === 404) return null
          throw e
        }
      },
    })),
  })

  const scheduleSelectData = [
    { value: '__none__', label: t('common.none') },
    ...schedules.map((s) => ({ value: s.id, label: s.name })),
  ]

  async function handleScheduleChange(gate: Gate, scheduleId: string) {
    try {
      if (scheduleId === '__none__') {
        await policiesApi.removeMemberGateSchedule(gate.id, member.id)
      } else {
        await policiesApi.setMemberGateSchedule(gate.id, member.id, scheduleId)
      }
      qc.invalidateQueries({ queryKey: ['member-gate-schedule', gate.id, member.id] })
      notifySuccess(t('common.saved'))
    } catch (err) {
      notifyError(err, t('common.error'))
    }
  }

  if (gates.length === 0) {
    return <p className="text-sm text-muted-foreground">{t('gates.noGates')}</p>
  }

  return (
    <div className="space-y-2">
      <p className="text-xs text-muted-foreground">{t('members.schedulesHint')}</p>
      {gates.map((gate, i) => {
        const query = scheduleQueries[i]
        const currentSchedule = query.data as AccessSchedule | null | undefined
        const currentValue = currentSchedule?.id ?? '__none__'
        return (
          <div key={gate.id} className="flex items-center justify-between gap-2">
            <p className="text-sm truncate max-w-[140px]">{gate.name}</p>
            <SimpleSelect
              value={query.isLoading ? '__none__' : currentValue}
              onValueChange={(v) => handleScheduleChange(gate, v)}
              data={scheduleSelectData}
              disabled={query.isLoading}
              placeholder={query.isLoading ? t('common.loading') : undefined}
              className="w-[180px]"
            />
          </div>
        )
      })}
    </div>
  )
}

// ---------- Member settings Dialog ----------

function MemberSettingsDialog({
  member,
  gates,
  opened,
  onClose,
}: {
  member: Member
  gates: Gate[]
  opened: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const gatePermissions = useGatePermissions()
  const qc = useQueryClient()

  const { data: schedules = [] } = useQuery<AccessSchedule[]>({
    queryKey: ['schedules'],
    queryFn: () => schedulesApi.list(),
    enabled: opened,
  })

  const updateAuth = useMutation({
    mutationFn: (cfg: Record<string, unknown>) =>
      membersApi.update(member.id, { auth_config: cfg }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['members'] }); notifySuccess(t('common.saved')) },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  function getAuthValue(key: string): string {
    const val = (member.auth_config as Record<string, unknown> | null)?.[key]
    if (val === true) return 'on'
    if (val === false) return 'off'
    return 'inherit'
  }

  function setAuthValue(key: string, value: string) {
    const mapped: boolean | null = value === 'on' ? true : value === 'off' ? false : null
    const current = (member.auth_config ?? {}) as Record<string, unknown>
    updateAuth.mutate({ ...current, [key]: mapped })
  }

  const { data: policies = [] } = useQuery<MemberPolicy[]>({
    queryKey: ['member-policies', member.id],
    queryFn: () => policiesApi.listByMember(member.id),
    enabled: opened,
  })

  const policySet = new Set(policies.map((p) => `${p.gate_id}:${p.permission_code}`))
  const hasPermission = (gateId: string, permCode: string) => policySet.has(`${gateId}:${permCode}`)

  function invalidatePolicies() {
    qc.invalidateQueries({ queryKey: ['member-policies', member.id] })
  }

  async function toggle(gateId: string, permCode: string, on: boolean) {
    try {
      if (on) await policiesApi.revoke(gateId, member.id, permCode)
      else await policiesApi.grant(gateId, member.id, permCode)
      invalidatePolicies()
    } catch (err) {
      notifyError(err, t('common.error'))
    }
  }

  const memberName = member.display_name ?? member.username ?? member.id.slice(0, 8)

  return (
    <Dialog open={opened} onOpenChange={(open) => { if (!open) onClose() }}>
      <DialogContent className="max-w-lg max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            <div className="flex items-center gap-2">
              <Avatar className="h-7 w-7">
                <AvatarFallback>{memberName[0].toUpperCase()}</AvatarFallback>
              </Avatar>
              <span>{memberName}</span>
            </div>
          </DialogTitle>
        </DialogHeader>

        <Tabs defaultValue="permissions">
          <TabsList className="w-full">
            <TabsTrigger value="permissions" className="flex-1">{t('members.gatePermissions')}</TabsTrigger>
            <TabsTrigger value="schedules" className="flex-1">{t('members.schedules')}</TabsTrigger>
            <TabsTrigger value="auth" className="flex-1">{t('members.authOverrides')}</TabsTrigger>
          </TabsList>

          <TabsContent value="permissions">
            <p className="text-xs text-muted-foreground mb-3">{t('members.gatePermissionsHint')}</p>
            {gates.length === 0 ? (
              <p className="text-sm text-muted-foreground">{t('gates.noGates')}</p>
            ) : (
              <GatePermissionsGrid
                gates={gates}
                permissions={gatePermissions}
                isChecked={hasPermission}
                onToggle={(gateId, code) => toggle(gateId, code, hasPermission(gateId, code))}
              />
            )}
          </TabsContent>

          <TabsContent value="schedules">
            <MemberSchedulesTab member={member} gates={gates} schedules={schedules} />
          </TabsContent>

          <TabsContent value="auth">
            <div className="space-y-4">
              <p className="text-xs text-muted-foreground">{t('members.authOverridesHint')}</p>
              {AUTH_METHODS.map(({ key, labelKey }) => (
                <div key={key} className="space-y-1">
                  <p className="text-sm font-medium">{t(labelKey as Parameters<typeof t>[0])}</p>
                  <div className="flex gap-1">
                    {(['inherit', 'on', 'off'] as const).map((val) => (
                      <Button
                        key={val}
                        size="sm"
                        variant={getAuthValue(key) === val ? 'default' : 'outline'}
                        onClick={() => setAuthValue(key, val)}
                        className="flex-1"
                      >
                        {val === 'inherit' ? t('members.authInherit') : val === 'on' ? 'On' : 'Off'}
                      </Button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}

// ---------- Edit member modal ----------

function EditMemberModal({
  member,
  opened,
  onClose,
}: {
  member: Member
  opened: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [displayName, setDisplayName] = useState(member.display_name ?? '')
  const [username, setUsername] = useState(member.username ?? '')
  const [role, setRole] = useState(member.role)
  const [error, setError] = useState<string | null>(null)

  const update = useMutation({
    mutationFn: () =>
      membersApi.update(member.id, {
        display_name: displayName || undefined,
        username: username || undefined,
        role: role !== member.role ? role : undefined,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['members'] })
      onClose()
    },
    onError: (err: unknown) => {
      setError(extractApiError(err, t('common.error')))
    },
  })

  return (
    <Dialog open={opened} onOpenChange={(open) => { if (!open) onClose() }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('members.editMemberInfo')}</DialogTitle>
        </DialogHeader>
        <form onSubmit={(e) => { e.preventDefault(); setError(null); update.mutate() }}>
          <div className="space-y-4">
            <Input
              label={t('members.displayName')}
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder={t('members.displayNamePlaceholder')}
            />
            <Input
              label={t('members.username')}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder={t('members.usernamePlaceholder')}
            />
            <SimpleSelect
              label={t('common.role')}
              value={role}
              onValueChange={(v) => setRole(v as Member['role'])}
              data={[
                { value: 'MEMBER', label: 'Member' },
                { value: 'ADMIN', label: 'Admin' },
              ]}
            />
            {error && (
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            <div className="flex justify-end gap-2">
              <Button variant="outline" type="button" onClick={onClose}>{t('common.cancel')}</Button>
              <Button type="submit" loading={update.isPending}>{t('common.save')}</Button>
            </div>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ---------- Main Page ----------

export default function MembersPage() {
  const qc = useQueryClient()
  const { t } = useTranslation()

  const [addOpened, setAddOpened] = useState(false)
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('MEMBER')
  const [addError, setAddError] = useState<string | null>(null)

  const [drawerMember, setDrawerMember] = useState<Member | null>(null)
  const [editMember, setEditMember] = useState<Member | null>(null)

  const { data: members, isLoading, isError, error } = useQuery<Member[]>({
    queryKey: ['members'],
    queryFn: () => membersApi.list(),
  })

  const { data: gates = [] } = useQuery<Gate[]>({
    queryKey: ['gates'],
    queryFn: () => gatesApi.list(),
  })

  const createMember = useMutation({
    mutationFn: () =>
      membersApi.create({
        username,
        display_name: displayName || undefined,
        password,
        role,
      }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['members'] }); resetAndClose(); notifySuccess(t('common.saved')) },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { title?: string } } })?.response?.data?.title
      setAddError(msg ?? t('common.error'))
    },
  })

  const deleteMember = useMutation({
    mutationFn: (memberId: string) => membersApi.delete(memberId),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['members'] }); notifySuccess(t('common.saved')) },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  function resetAndClose() {
    setAddOpened(false)
    setUsername(''); setDisplayName(''); setPassword('')
    setRole('MEMBER'); setAddError(null)
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setAddError(null)
    createMember.mutate()
  }

  return (
    <div className="max-w-2xl mx-auto py-8 px-4">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">{t('members.title')}</h2>
          <p className="text-sm text-muted-foreground">{t('members.subtitle')}</p>
        </div>
        <Button onClick={() => setAddOpened(true)}>
          <UserPlus size={16} />
          {t('members.add')}
        </Button>
      </div>

      {/* Add member modal */}
      <Dialog open={addOpened} onOpenChange={(open) => { if (!open) resetAndClose() }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('members.add')}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSubmit}>
            <div className="space-y-4">
              <Input
                label={t('members.username')}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                placeholder={t('members.usernamePlaceholder')}
              />
              <Input
                label={t('members.displayName')}
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder={t('members.displayNamePlaceholder')}
              />
              <Input
                type="password"
                label={t('auth.password')}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={8}
              />
              <SimpleSelect
                label={t('common.role')}
                value={role}
                onValueChange={(v) => setRole(v)}
                data={[
                  { value: 'MEMBER', label: 'Member' },
                  { value: 'ADMIN', label: 'Admin' },
                ]}
              />
              {addError && (
                <Alert variant="destructive">
                  <AlertCircle className="h-4 w-4" />
                  <AlertDescription>{addError}</AlertDescription>
                </Alert>
              )}
              <div className="flex justify-end gap-2">
                <Button variant="outline" type="button" onClick={resetAndClose}>{t('common.cancel')}</Button>
                <Button type="submit" loading={createMember.isPending}>{t('common.add')}</Button>
              </div>
            </div>
          </form>
        </DialogContent>
      </Dialog>

      {/* Member list */}
      {isLoading ? (
        <div className="space-y-2">
          {[0, 1, 2].map((i) => <Skeleton key={i} className="h-[60px] rounded-md" />)}
        </div>
      ) : isError ? (
        <QueryError error={error} />
      ) : members?.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 gap-2">
          <Users size={36} className="opacity-30" />
          <p className="text-sm text-muted-foreground">{t('members.noMembers')}</p>
        </div>
      ) : (
        <div className="space-y-1.5">
          {members?.map((m) => {
            const memberName = m.display_name ?? m.username ?? m.id.slice(0, 8)

            return (
              <div key={m.id} className="border rounded-lg p-3">
                <div className="flex items-center gap-2">
                  <Avatar className="h-8 w-8 shrink-0">
                    <AvatarFallback>{memberName[0].toUpperCase()}</AvatarFallback>
                  </Avatar>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate">{memberName}</p>
                    {m.username && m.display_name && (
                      <p className="text-xs text-muted-foreground font-mono truncate">{m.username}</p>
                    )}
                  </div>
                  <Badge variant={ROLE_VARIANT[m.role] ?? 'secondary'} className="shrink-0">{m.role}</Badge>
                  <div className="flex items-center gap-0.5 shrink-0">
                    <SimpleTooltip label={t('members.editMemberInfo')}>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => setEditMember(m)}
                      >
                        <Pencil size={14} />
                      </Button>
                    </SimpleTooltip>
                    {m.role === 'MEMBER' && (
                      <SimpleTooltip label={t('members.editMember')}>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => setDrawerMember(m)}
                        >
                          <Settings2 size={14} />
                        </Button>
                      </SimpleTooltip>
                    )}
                    <SimpleTooltip label={t('common.delete')}>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="text-destructive hover:text-destructive"
                        onClick={() => deleteMember.mutate(m.id)}
                      >
                        <Trash2 size={14} />
                      </Button>
                    </SimpleTooltip>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Edit member modal */}
      {editMember && (
        <EditMemberModal
          member={editMember}
          opened={!!editMember}
          onClose={() => setEditMember(null)}
        />
      )}

      {/* Per-member settings dialog */}
      {drawerMember && (
        <MemberSettingsDialog
          member={members?.find((m) => m.id === drawerMember.id) ?? drawerMember}
          gates={gates}
          opened={!!drawerMember}
          onClose={() => setDrawerMember(null)}
        />
      )}
    </div>
  )
}
