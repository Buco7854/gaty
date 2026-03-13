import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { schedulesApi } from '@/api'
import type { AccessSchedule, ExprNode, ScheduleRule } from '@/types'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2, Pencil, CalendarClock, Lock, User, Loader2 } from 'lucide-react'
import { QueryError } from '@/components/QueryError'
import { extractApiError } from '@/lib/notify'
import { useAuthStore } from '@/store/auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { SimpleSelect } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'

// 0=Sun, 1=Mon, …, 6=Sat (Go's time.Weekday)
const WEEKDAY_OPTIONS = [
  { value: '0', label: 'Sun' },
  { value: '1', label: 'Mon' },
  { value: '2', label: 'Tue' },
  { value: '3', label: 'Wed' },
  { value: '4', label: 'Thu' },
  { value: '5', label: 'Fri' },
  { value: '6', label: 'Sat' },
]

const MONTH_OPTIONS = [
  { value: '1', label: 'Jan' }, { value: '2', label: 'Feb' },
  { value: '3', label: 'Mar' }, { value: '4', label: 'Apr' },
  { value: '5', label: 'May' }, { value: '6', label: 'Jun' },
  { value: '7', label: 'Jul' }, { value: '8', label: 'Aug' },
  { value: '9', label: 'Sep' }, { value: '10', label: 'Oct' },
  { value: '11', label: 'Nov' }, { value: '12', label: 'Dec' },
]

function ruleDescription(rule: ScheduleRule): string {
  switch (rule.type) {
    case 'time_range': {
      const dayLabels = (rule.days ?? []).map(d => WEEKDAY_OPTIONS[d]?.label ?? String(d)).join(',')
      const timeStr = rule.start_time && rule.end_time ? ` ${rule.start_time}–${rule.end_time}` : ''
      return dayLabels ? `${dayLabels}${timeStr}` : `All days${timeStr}`
    }
    case 'weekdays_range':
      return `${WEEKDAY_OPTIONS[rule.start_day ?? 0]?.label ?? '?'} → ${WEEKDAY_OPTIONS[rule.end_day ?? 6]?.label ?? '?'}`
    case 'date_range':
      return `${rule.start_date ?? '?'} → ${rule.end_date ?? '?'}`
    case 'day_of_month_range':
      return `Day ${rule.start_dom ?? '?'}–${rule.end_dom ?? '?'}`
    case 'month_range':
      return `${MONTH_OPTIONS[(rule.start_month ?? 1) - 1]?.label ?? '?'} → ${MONTH_OPTIONS[(rule.end_month ?? 12) - 1]?.label ?? '?'}`
    default:
      return rule.type
  }
}

function exprSummary(node: ExprNode, depth = 0): string {
  if (depth > 2) return '…'
  switch (node.op) {
    case 'rule': return node.rule ? ruleDescription(node.rule) : '?'
    case 'not': {
      const c = node.children?.[0]
      return `¬${c ? exprSummary(c, depth + 1) : '?'}`
    }
    case 'and':
      return `(${(node.children ?? []).map(c => exprSummary(c, depth + 1)).join(' ∧ ')})`
    case 'or':
      return `(${(node.children ?? []).map(c => exprSummary(c, depth + 1)).join(' ∨ ')})`
    default:
      return node.op
  }
}

function emptyRule(): ScheduleRule {
  return { type: 'time_range', days: [], start_time: '08:00', end_time: '18:00' }
}

function makeRuleNode(): ExprNode {
  return { op: 'rule', rule: emptyRule() }
}

function makeAndNode(): ExprNode {
  return { op: 'and', children: [] }
}

function makeOrNode(): ExprNode {
  return { op: 'or', children: [] }
}

function makeNotNode(): ExprNode {
  return { op: 'not', children: [] }
}

function RuleEditor({ rule, onChange, onDelete, ruleTypeOptions }: {
  rule: ScheduleRule
  onChange: (r: ScheduleRule) => void
  onDelete: () => void
  ruleTypeOptions: { value: string; label: string }[]
}) {
  function changeType(type: string) {
    switch (type) {
      case 'time_range':         onChange({ type, days: [], start_time: '08:00', end_time: '18:00' }); break
      case 'weekdays_range':     onChange({ type, start_day: 1, end_day: 5 }); break
      case 'date_range':         onChange({ type, start_date: '', end_date: '' }); break
      case 'day_of_month_range': onChange({ type, start_dom: 1, end_dom: 7 }); break
      case 'month_range':        onChange({ type, start_month: 1, end_month: 12 }); break
    }
  }

  const selectedDays = new Set((rule.days ?? []).map(String))

  function toggleDay(dayValue: string, checked: boolean) {
    const current = rule.days ?? []
    const dayNum = Number(dayValue)
    const next = checked ? [...current, dayNum] : current.filter(d => d !== dayNum)
    onChange({ ...rule, days: next })
  }

  return (
    <div className="border rounded-md p-3 bg-muted/50">
      <div className="flex items-center justify-between mb-2 gap-2">
        <SimpleSelect
          value={rule.type}
          onValueChange={(v) => changeType(v)}
          data={ruleTypeOptions}
          className="flex-1"
        />
        <Button size="icon-sm" variant="ghost" className="text-destructive" onClick={onDelete}>
          <Trash2 size={13} />
        </Button>
      </div>

      {rule.type === 'time_range' && (
        <div className="space-y-2">
          <div>
            <p className="text-sm font-medium mb-1">Days of week</p>
            <div className="flex items-center gap-2 flex-wrap">
              {WEEKDAY_OPTIONS.map((d) => (
                <Checkbox
                  key={d.value}
                  label={d.label}
                  checked={selectedDays.has(d.value)}
                  onCheckedChange={(checked) => toggleDay(d.value, !!checked)}
                />
              ))}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <Input label="Start time" type="time" value={rule.start_time ?? ''} onChange={(e) => onChange({ ...rule, start_time: e.target.value })} />
            <Input label="End time" type="time" value={rule.end_time ?? ''} onChange={(e) => onChange({ ...rule, end_time: e.target.value })} />
          </div>
        </div>
      )}

      {rule.type === 'weekdays_range' && (
        <div className="grid grid-cols-2 gap-2">
          <SimpleSelect label="From" value={String(rule.start_day ?? 0)} onValueChange={(v) => onChange({ ...rule, start_day: Number(v) })} data={WEEKDAY_OPTIONS} />
          <SimpleSelect label="To" value={String(rule.end_day ?? 6)} onValueChange={(v) => onChange({ ...rule, end_day: Number(v) })} data={WEEKDAY_OPTIONS} />
        </div>
      )}

      {rule.type === 'date_range' && (
        <div className="grid grid-cols-2 gap-2">
          <Input label="Start date" type="date" value={rule.start_date ?? ''} onChange={(e) => onChange({ ...rule, start_date: e.target.value })} />
          <Input label="End date" type="date" value={rule.end_date ?? ''} onChange={(e) => onChange({ ...rule, end_date: e.target.value })} />
        </div>
      )}

      {rule.type === 'day_of_month_range' && (
        <div className="grid grid-cols-2 gap-2">
          <Input label="From day" type="number" min={1} max={31} value={rule.start_dom ?? 1} onChange={(e) => onChange({ ...rule, start_dom: Number(e.target.value) })} />
          <Input label="To day" type="number" min={1} max={31} value={rule.end_dom ?? 7} onChange={(e) => onChange({ ...rule, end_dom: Number(e.target.value) })} />
        </div>
      )}

      {rule.type === 'month_range' && (
        <div className="grid grid-cols-2 gap-2">
          <SimpleSelect label="From month" value={String(rule.start_month ?? 1)} onValueChange={(v) => onChange({ ...rule, start_month: Number(v) })} data={MONTH_OPTIONS} />
          <SimpleSelect label="To month" value={String(rule.end_month ?? 12)} onValueChange={(v) => onChange({ ...rule, end_month: Number(v) })} data={MONTH_OPTIONS} />
        </div>
      )}
    </div>
  )
}

function AddChildButtons({ onAdd }: { onAdd: (n: ExprNode) => void }) {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-1">
      <Button size="sm" variant="outline" onClick={() => onAdd(makeRuleNode())} type="button">
        <Plus size={11} className="mr-1" />
        {t('schedules.opRule')}
      </Button>
      <Button size="sm" variant="outline" onClick={() => onAdd(makeAndNode())} type="button">AND</Button>
      <Button size="sm" variant="outline" onClick={() => onAdd(makeOrNode())} type="button">OR</Button>
      <Button size="sm" variant="outline" onClick={() => onAdd(makeNotNode())} type="button">NOT</Button>
    </div>
  )
}

function ExprNodeEditor({ node, onChange, onDelete, depth, ruleTypeOptions }: {
  node: ExprNode
  onChange: (n: ExprNode) => void
  onDelete?: () => void
  depth: number
  ruleTypeOptions: { value: string; label: string }[]
}) {
  const children = node.children ?? []

  if (node.op === 'rule') {
    return (
      <RuleEditor
        rule={node.rule ?? emptyRule()}
        onChange={(r) => onChange({ ...node, rule: r })}
        onDelete={onDelete ?? (() => {})}
        ruleTypeOptions={ruleTypeOptions}
      />
    )
  }

  if (node.op === 'not') {
    const child = children[0]
    return (
      <div className="border-2 border-orange-500 rounded-lg p-3">
        <div className="flex items-center justify-between mb-2">
          <Badge variant="warning">NOT</Badge>
          {onDelete && (
            <Button size="icon-sm" variant="ghost" className="text-destructive" onClick={onDelete}>
              <Trash2 size={13} />
            </Button>
          )}
        </div>
        {child ? (
          <ExprNodeEditor
            node={child}
            onChange={(n) => onChange({ ...node, children: [n] })}
            onDelete={() => onChange({ ...node, children: [] })}
            depth={depth + 1}
            ruleTypeOptions={ruleTypeOptions}
          />
        ) : (
          <AddChildButtons onAdd={(n) => onChange({ ...node, children: [n] })} />
        )}
      </div>
    )
  }

  const borderClass = node.op === 'and' ? 'border-blue-500' : 'border-green-500'

  return (
    <div className={cn('border rounded-lg p-3', borderClass, depth === 0 ? 'border-2' : 'border')}>
      <div className="flex items-center justify-between mb-3">
        <div className="inline-flex rounded-md border overflow-hidden">
          <button
            type="button"
            className={cn(
              'px-3 py-1 text-xs font-medium transition-colors',
              node.op === 'and' ? 'bg-primary text-primary-foreground' : 'bg-background hover:bg-muted'
            )}
            onClick={() => onChange({ ...node, op: 'and' })}
          >
            AND
          </button>
          <button
            type="button"
            className={cn(
              'px-3 py-1 text-xs font-medium transition-colors',
              node.op === 'or' ? 'bg-primary text-primary-foreground' : 'bg-background hover:bg-muted'
            )}
            onClick={() => onChange({ ...node, op: 'or' })}
          >
            OR
          </button>
        </div>
        {onDelete && (
          <Button size="icon-sm" variant="ghost" className="text-destructive" onClick={onDelete}>
            <Trash2 size={13} />
          </Button>
        )}
      </div>

      <div className="space-y-2">
        {children.map((child, i) => (
          <ExprNodeEditor
            key={i}
            node={child}
            onChange={(n) => onChange({ ...node, children: children.map((c, idx) => idx === i ? n : c) })}
            onDelete={() => onChange({ ...node, children: children.filter((_, idx) => idx !== i) })}
            depth={depth + 1}
            ruleTypeOptions={ruleTypeOptions}
          />
        ))}
        <AddChildButtons onAdd={(n) => onChange({ ...node, children: [...children, n] })} />
      </div>
    </div>
  )
}

function ScheduleCard({
  s, onEdit, onDelete, isDeleting, t,
}: {
  s: AccessSchedule; onEdit: (s: AccessSchedule) => void; onDelete: (id: string) => void; isDeleting: boolean; t: (key: string) => string
}) {
  return (
    <div className="border rounded-lg p-4">
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0 space-y-1">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-sm truncate">{s.name}</span>
            {s.expr ? (
              <Badge variant={
                s.expr.op === 'and' ? 'default' : s.expr.op === 'or' ? 'success' : s.expr.op === 'not' ? 'warning' : 'secondary'
              }>{s.expr.op.toUpperCase()}</Badge>
            ) : (
              <Badge variant="outline">{t('schedules.noRestriction')}</Badge>
            )}
          </div>
          {s.description && <p className="text-xs text-muted-foreground">{s.description}</p>}
          {s.expr && <p className="text-xs text-muted-foreground font-mono truncate">{exprSummary(s.expr)}</p>}
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button size="icon-sm" variant="ghost" onClick={() => onEdit(s)}><Pencil size={14} /></Button>
          <Button size="icon-sm" variant="ghost" className="text-destructive" loading={isDeleting} onClick={() => onDelete(s.id)}><Trash2 size={14} /></Button>
        </div>
      </div>
    </div>
  )
}

function ScheduleModal({
  opened, onClose, editing: _editing, onSave, isSaving, saveError,
  name, setName, description, setDescription, expr, setExpr, ruleTypeOptions, title, t,
}: {
  opened: boolean; onClose: () => void; editing: AccessSchedule | null; onSave: (e: React.FormEvent) => void; isSaving: boolean; saveError: string | null
  name: string; setName: (v: string) => void; description: string; setDescription: (v: string) => void
  expr: ExprNode | null; setExpr: (v: ExprNode | null) => void; ruleTypeOptions: { value: string; label: string }[]; title: string; t: (key: string) => string
}) {
  return (
    <Dialog open={opened} onOpenChange={(open) => { if (!open) onClose() }}>
      <DialogContent className="max-w-lg max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <form onSubmit={onSave}>
          <div className="space-y-4">
            <Input label={t('common.name')} value={name} onChange={(e) => setName(e.target.value)} required autoFocus />
            <Input label={t('schedules.description')} placeholder={t('schedules.descriptionPlaceholder')} value={description} onChange={(e) => setDescription(e.target.value)} />
            <p className="text-xs text-muted-foreground">{t('schedules.exprHint')}</p>
            <div className="relative">
              <div className="absolute inset-0 flex items-center">
                <Separator className="w-full" />
              </div>
              <div className="relative flex justify-start">
                <span className="bg-background pr-2 text-xs text-muted-foreground">{t('schedules.addExpr')}</span>
              </div>
            </div>
            {expr === null ? (
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">{t('schedules.noRestriction')}</p>
                <div className="flex items-center gap-1">
                  <Button size="sm" variant="outline" onClick={() => setExpr(makeAndNode())} type="button">
                    <Plus size={11} className="mr-1" />AND
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => setExpr(makeOrNode())} type="button">
                    <Plus size={11} className="mr-1" />OR
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => setExpr(makeNotNode())} type="button">
                    <Plus size={11} className="mr-1" />NOT
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => setExpr(makeRuleNode())} type="button">
                    <Plus size={11} className="mr-1" />{t('schedules.opRule')}
                  </Button>
                </div>
              </div>
            ) : (
              <ExprNodeEditor node={expr} onChange={setExpr} onDelete={() => setExpr(null)} depth={0} ruleTypeOptions={ruleTypeOptions} />
            )}
            {saveError && (
              <Alert variant="destructive">
                <AlertDescription className="whitespace-pre-line">{saveError}</AlertDescription>
              </Alert>
            )}
            <div className="flex items-center justify-end gap-2 pt-2">
              <Button variant="outline" onClick={onClose} type="button">{t('common.cancel')}</Button>
              <Button type="submit" loading={isSaving} disabled={!name.trim()}>{t('common.save')}</Button>
            </div>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function useScheduleForm() {
  const [editing, setEditing] = useState<AccessSchedule | null>(null)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [expr, setExpr] = useState<ExprNode | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [modalOpened, setModalOpened] = useState(false)

  function openCreate() { setEditing(null); setName(''); setDescription(''); setExpr(null); setSaveError(null); setModalOpened(true) }
  function openEdit(s: AccessSchedule) { setEditing(s); setName(s.name); setDescription(s.description ?? ''); setExpr(s.expr ?? null); setSaveError(null); setModalOpened(true) }
  function closeModal() { setModalOpened(false); setSaveError(null) }

  return { editing, name, setName, description, setDescription, expr, setExpr, saveError, setSaveError, modalOpened, openCreate, openEdit, closeModal }
}

export default function SchedulesPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()

  const session = useAuthStore((s) => s.session)
  const member = session?.type === 'member' ? session.member : null
  const isAdmin = member?.role === 'ADMIN'

  const ruleTypeOptions = [
    { value: 'time_range', label: t('schedules.timeRange') },
    { value: 'weekdays_range', label: t('schedules.weekdaysRange') },
    { value: 'date_range', label: t('schedules.dateRange') },
    { value: 'day_of_month_range', label: t('schedules.dayOfMonthRange') },
    { value: 'month_range', label: t('schedules.monthRange') },
  ]

  // --- Admin schedules ---
  const adminForm = useScheduleForm()

  const { data: adminSchedules = [], isLoading: adminLoading, isError: adminError, error: adminErr } = useQuery<AccessSchedule[]>({
    queryKey: ['schedules'],
    queryFn: () => schedulesApi.list(),
    enabled: isAdmin,
  })

  const adminCreateMut = useMutation({
    mutationFn: () => schedulesApi.create({ name: adminForm.name.trim(), description: adminForm.description.trim() || undefined, expr: adminForm.expr }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['schedules'] }); adminForm.closeModal() },
    onError: (err) => adminForm.setSaveError(extractApiError(err, 'An error occurred')),
  })

  const adminUpdateMut = useMutation({
    mutationFn: () => schedulesApi.update(adminForm.editing!.id, { name: adminForm.name.trim(), description: adminForm.description.trim() || undefined, expr: adminForm.expr }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['schedules'] }); adminForm.closeModal() },
    onError: (err) => adminForm.setSaveError(extractApiError(err, 'An error occurred')),
  })

  const adminDeleteMut = useMutation({
    mutationFn: (id: string) => schedulesApi.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['schedules'] }),
  })

  function handleAdminSave(e: React.FormEvent) {
    e.preventDefault()
    adminForm.setSaveError(null)
    if (adminForm.editing) adminUpdateMut.mutate()
    else adminCreateMut.mutate()
  }

  // --- My schedules (personal) ---
  const myForm = useScheduleForm()

  const { data: mySchedules = [], isLoading: myLoading, isError: myError, error: myErr } = useQuery<AccessSchedule[]>({
    queryKey: ['member-schedules'],
    queryFn: () => schedulesApi.listMine(),
  })

  const myCreateMut = useMutation({
    mutationFn: () => schedulesApi.createMine({ name: myForm.name.trim(), description: myForm.description.trim() || undefined, expr: myForm.expr }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['member-schedules'] }); myForm.closeModal() },
    onError: (err) => myForm.setSaveError(extractApiError(err, 'An error occurred')),
  })

  const myUpdateMut = useMutation({
    mutationFn: () => schedulesApi.updateMine(myForm.editing!.id, { name: myForm.name.trim(), description: myForm.description.trim() || undefined, expr: myForm.expr }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['member-schedules'] }); myForm.closeModal() },
    onError: (err) => myForm.setSaveError(extractApiError(err, 'An error occurred')),
  })

  const myDeleteMut = useMutation({
    mutationFn: (id: string) => schedulesApi.deleteMine(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['member-schedules'] }),
  })

  function handleMySave(e: React.FormEvent) {
    e.preventDefault()
    myForm.setSaveError(null)
    if (myForm.editing) myUpdateMut.mutate()
    else myCreateMut.mutate()
  }

  return (
    <div className="max-w-2xl mx-auto py-8 px-4">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{t('schedules.title')}</h2>
          <p className="text-sm text-muted-foreground">{t('schedules.subtitle')}</p>
        </div>
      </div>

      <Tabs defaultValue="personal">
        <TabsList className="mb-6">
          <TabsTrigger value="personal">
            <User size={14} className="mr-1.5" />
            {t('schedules.mySchedulesTitle')}
          </TabsTrigger>
          <TabsTrigger value="admin" disabled={!isAdmin}>
            <Lock size={14} className="mr-1.5" />
            {t('schedules.adminSchedulesTitle')}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="personal">
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">{t('schedules.mySchedulesSubtitle')}</p>
              <Button size="sm" variant="outline" onClick={myForm.openCreate}>
                <Plus size={14} className="mr-1.5" />
                {t('schedules.add')}
              </Button>
            </div>
            {myLoading ? (
              <div className="flex justify-center py-4">
                <Loader2 size={20} className="animate-spin text-muted-foreground" />
              </div>
            ) : myError ? (
              <QueryError error={myErr} />
            ) : mySchedules.length === 0 ? (
              <div className="border rounded-lg p-8">
                <div className="flex flex-col items-center gap-2">
                  <CalendarClock size={28} className="opacity-30" />
                  <p className="text-muted-foreground text-sm">{t('schedules.noSchedules')}</p>
                  <p className="text-muted-foreground text-xs">{t('schedules.mySchedulesHint')}</p>
                </div>
              </div>
            ) : (
              <div className="space-y-2">
                {mySchedules.map((s) => (
                  <ScheduleCard key={s.id} s={s} onEdit={myForm.openEdit} onDelete={(id) => myDeleteMut.mutate(id)} isDeleting={myDeleteMut.isPending} t={t} />
                ))}
              </div>
            )}
          </div>
        </TabsContent>

        <TabsContent value="admin">
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">{t('schedules.adminSchedulesSubtitle')}</p>
              <Button size="sm" variant="outline" onClick={adminForm.openCreate}>
                <Plus size={14} className="mr-1.5" />
                {t('schedules.add')}
              </Button>
            </div>
            {adminLoading ? (
              <div className="flex justify-center py-4">
                <Loader2 size={20} className="animate-spin text-muted-foreground" />
              </div>
            ) : adminError ? (
              <QueryError error={adminErr} />
            ) : adminSchedules.length === 0 ? (
              <div className="border rounded-lg p-8">
                <div className="flex flex-col items-center gap-2">
                  <CalendarClock size={28} className="opacity-30" />
                  <p className="text-muted-foreground text-sm">{t('schedules.noSchedules')}</p>
                  <p className="text-muted-foreground text-xs">{t('schedules.adminSchedulesHint')}</p>
                </div>
              </div>
            ) : (
              <div className="space-y-2">
                {adminSchedules.map((s) => (
                  <ScheduleCard key={s.id} s={s} onEdit={adminForm.openEdit} onDelete={(id) => adminDeleteMut.mutate(id)} isDeleting={adminDeleteMut.isPending} t={t} />
                ))}
              </div>
            )}
          </div>
        </TabsContent>
      </Tabs>

      {/* My schedules modal */}
      <ScheduleModal
        opened={myForm.modalOpened} onClose={myForm.closeModal} editing={myForm.editing} onSave={handleMySave}
        isSaving={myCreateMut.isPending || myUpdateMut.isPending} saveError={myForm.saveError}
        name={myForm.name} setName={myForm.setName} description={myForm.description} setDescription={myForm.setDescription}
        expr={myForm.expr} setExpr={myForm.setExpr} ruleTypeOptions={ruleTypeOptions}
        title={myForm.editing ? t('schedules.editSchedule') : t('schedules.addMySchedule')} t={t}
      />

      {/* Admin schedules modal */}
      {isAdmin && (
        <ScheduleModal
          opened={adminForm.modalOpened} onClose={adminForm.closeModal} editing={adminForm.editing} onSave={handleAdminSave}
          isSaving={adminCreateMut.isPending || adminUpdateMut.isPending} saveError={adminForm.saveError}
          name={adminForm.name} setName={adminForm.setName} description={adminForm.description} setDescription={adminForm.setDescription}
          expr={adminForm.expr} setExpr={adminForm.setExpr} ruleTypeOptions={ruleTypeOptions}
          title={adminForm.editing ? t('schedules.editSchedule') : t('schedules.addAdminSchedule')} t={t}
        />
      )}
    </div>
  )
}
