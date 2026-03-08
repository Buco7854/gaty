import { useState } from 'react'
import type { AxiosError } from 'axios'
import { useParams } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { schedulesApi } from '@/api'
import type { AccessSchedule, ExprNode, ScheduleRule, WorkspaceWithRole } from '@/types'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Stack, Paper, Group, Button, TextInput, ActionIcon,
  Badge, Modal, Select, NumberInput, Divider, Alert, Loader, Center,
  SimpleGrid, SegmentedControl, Checkbox, Tabs,
} from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { Plus, Trash2, Pencil, CalendarClock, Lock, User } from 'lucide-react'
import { QueryError } from '@/components/QueryError'
import { useAuthStore } from '@/store/auth'

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

  return (
    <Paper withBorder p="xs" radius="sm" style={{ background: 'var(--mantine-color-default-hover)' }}>
      <Group justify="space-between" mb="xs" gap="xs">
        <Select
          value={rule.type}
          onChange={(v) => v && changeType(v)}
          data={ruleTypeOptions}
          size="xs"
          style={{ flex: 1 }}
        />
        <ActionIcon size="sm" color="red" variant="subtle" onClick={onDelete}>
          <Trash2 size={13} />
        </ActionIcon>
      </Group>

      {rule.type === 'time_range' && (
        <Stack gap="xs">
          <Checkbox.Group
            label="Days of week"
            value={(rule.days ?? []).map(String)}
            onChange={(vals) => onChange({ ...rule, days: vals.map(Number) })}
          >
            <Group gap={6} mt={4} wrap="wrap">
              {WEEKDAY_OPTIONS.map((d) => (
                <Checkbox key={d.value} value={d.value} label={d.label} size="xs" />
              ))}
            </Group>
          </Checkbox.Group>
          <SimpleGrid cols={2}>
            <TextInput
              label="Start time"
              type="time"
              value={rule.start_time ?? ''}
              onChange={(e) => onChange({ ...rule, start_time: e.target.value })}
              size="xs"
            />
            <TextInput
              label="End time"
              type="time"
              value={rule.end_time ?? ''}
              onChange={(e) => onChange({ ...rule, end_time: e.target.value })}
              size="xs"
            />
          </SimpleGrid>
        </Stack>
      )}

      {rule.type === 'weekdays_range' && (
        <SimpleGrid cols={2}>
          <Select
            label="From"
            value={String(rule.start_day ?? 0)}
            onChange={(v) => onChange({ ...rule, start_day: Number(v) })}
            data={WEEKDAY_OPTIONS}
            size="xs"
          />
          <Select
            label="To"
            value={String(rule.end_day ?? 6)}
            onChange={(v) => onChange({ ...rule, end_day: Number(v) })}
            data={WEEKDAY_OPTIONS}
            size="xs"
          />
        </SimpleGrid>
      )}

      {rule.type === 'date_range' && (
        <SimpleGrid cols={2}>
          <TextInput
            label="Start date"
            type="date"
            value={rule.start_date ?? ''}
            onChange={(e) => onChange({ ...rule, start_date: e.target.value })}
            size="xs"
          />
          <TextInput
            label="End date"
            type="date"
            value={rule.end_date ?? ''}
            onChange={(e) => onChange({ ...rule, end_date: e.target.value })}
            size="xs"
          />
        </SimpleGrid>
      )}

      {rule.type === 'day_of_month_range' && (
        <SimpleGrid cols={2}>
          <NumberInput
            label="From day"
            min={1}
            max={31}
            value={rule.start_dom ?? 1}
            onChange={(v) => onChange({ ...rule, start_dom: Number(v) })}
            size="xs"
          />
          <NumberInput
            label="To day"
            min={1}
            max={31}
            value={rule.end_dom ?? 7}
            onChange={(v) => onChange({ ...rule, end_dom: Number(v) })}
            size="xs"
          />
        </SimpleGrid>
      )}

      {rule.type === 'month_range' && (
        <SimpleGrid cols={2}>
          <Select
            label="From month"
            value={String(rule.start_month ?? 1)}
            onChange={(v) => onChange({ ...rule, start_month: Number(v) })}
            data={MONTH_OPTIONS}
            size="xs"
          />
          <Select
            label="To month"
            value={String(rule.end_month ?? 12)}
            onChange={(v) => onChange({ ...rule, end_month: Number(v) })}
            data={MONTH_OPTIONS}
            size="xs"
          />
        </SimpleGrid>
      )}
    </Paper>
  )
}

function AddChildButtons({ onAdd }: { onAdd: (n: ExprNode) => void }) {
  const { t } = useTranslation()
  return (
    <Group gap={4}>
      <Button size="xs" variant="default" leftSection={<Plus size={11} />} onClick={() => onAdd(makeRuleNode())} type="button">
        {t('schedules.opRule')}
      </Button>
      <Button size="xs" variant="default" onClick={() => onAdd(makeAndNode())} type="button">
        AND
      </Button>
      <Button size="xs" variant="default" onClick={() => onAdd(makeOrNode())} type="button">
        OR
      </Button>
      <Button size="xs" variant="default" onClick={() => onAdd(makeNotNode())} type="button">
        NOT
      </Button>
    </Group>
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
      <Paper withBorder p="sm" radius="md" style={{ borderColor: 'var(--mantine-color-orange-6)', borderWidth: 2 }}>
        <Group justify="space-between" mb="xs">
          <Badge color="orange" size="sm" variant="filled">NOT</Badge>
          {onDelete && (
            <ActionIcon size="sm" color="red" variant="subtle" onClick={onDelete}>
              <Trash2 size={13} />
            </ActionIcon>
          )}
        </Group>
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
      </Paper>
    )
  }

  // and / or
  const borderColor = node.op === 'and'
    ? 'var(--mantine-color-blue-6)'
    : 'var(--mantine-color-green-6)'

  return (
    <Paper withBorder p="sm" radius="md" style={{ borderColor, borderWidth: depth === 0 ? 2 : 1 }}>
      <Group justify="space-between" mb="sm">
        <SegmentedControl
          value={node.op}
          onChange={(v) => onChange({ ...node, op: v as 'and' | 'or' })}
          data={[
            { value: 'and', label: 'AND' },
            { value: 'or', label: 'OR' },
          ]}
          size="xs"
        />
        {onDelete && (
          <ActionIcon size="sm" color="red" variant="subtle" onClick={onDelete}>
            <Trash2 size={13} />
          </ActionIcon>
        )}
      </Group>

      <Stack gap="xs">
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
      </Stack>
    </Paper>
  )
}

type ApiErrorBody = { detail?: string; errors?: { message: string; location?: string }[] }

function extractApiError(err: unknown): string {
  const data = (err as AxiosError<ApiErrorBody>).response?.data
  if (data?.errors?.length) {
    return data.errors.map((e) => e.location ? `${e.location}: ${e.message}` : e.message).join('\n')
  }
  return data?.detail ?? 'An error occurred'
}

// Shared schedule card component
function ScheduleCard({
  s,
  onEdit,
  onDelete,
  isDeleting,
  t,
}: {
  s: AccessSchedule
  onEdit: (s: AccessSchedule) => void
  onDelete: (id: string) => void
  isDeleting: boolean
  t: (key: string) => string
}) {
  return (
    <Paper withBorder p="md" radius="md">
      <Group justify="space-between" wrap="nowrap">
        <Stack gap={4} style={{ minWidth: 0 }}>
          <Group gap="xs">
            <Text fw={600} size="sm" truncate>{s.name}</Text>
            {s.expr ? (
              <Badge size="xs" variant="filled" color={
                s.expr.op === 'and' ? 'blue' :
                s.expr.op === 'or' ? 'green' :
                s.expr.op === 'not' ? 'orange' : 'gray'
              }>
                {s.expr.op.toUpperCase()}
              </Badge>
            ) : (
              <Badge size="xs" variant="light" color="gray">{t('schedules.noRestriction')}</Badge>
            )}
          </Group>
          {s.description && (
            <Text size="xs" c="dimmed">{s.description}</Text>
          )}
          {s.expr && (
            <Text size="xs" c="dimmed" ff="mono" truncate>
              {exprSummary(s.expr)}
            </Text>
          )}
        </Stack>
        <Group gap={4} wrap="nowrap">
          <ActionIcon size="sm" variant="subtle" onClick={() => onEdit(s)}>
            <Pencil size={14} />
          </ActionIcon>
          <ActionIcon
            size="sm"
            variant="subtle"
            color="red"
            loading={isDeleting}
            onClick={() => onDelete(s.id)}
          >
            <Trash2 size={14} />
          </ActionIcon>
        </Group>
      </Group>
    </Paper>
  )
}

// Shared schedule form modal
function ScheduleModal({
  opened,
  onClose,
  editing,
  onSave,
  isSaving,
  saveError,
  name,
  setName,
  description,
  setDescription,
  expr,
  setExpr,
  ruleTypeOptions,
  title,
  t,
}: {
  opened: boolean
  onClose: () => void
  editing: AccessSchedule | null
  onSave: (e: React.FormEvent) => void
  isSaving: boolean
  saveError: string | null
  name: string
  setName: (v: string) => void
  description: string
  setDescription: (v: string) => void
  expr: ExprNode | null
  setExpr: (v: ExprNode | null) => void
  ruleTypeOptions: { value: string; label: string }[]
  title: string
  t: (key: string) => string
}) {
  return (
    <Modal opened={opened} onClose={onClose} title={title} size="lg">
      <form onSubmit={onSave}>
        <Stack gap="md">
          <TextInput
            label={t('common.name')}
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            autoFocus
          />
          <TextInput
            label={t('schedules.description')}
            placeholder={t('schedules.descriptionPlaceholder')}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />

          <Text size="xs" c="dimmed">{t('schedules.exprHint')}</Text>

          <Divider label={t('schedules.addExpr')} labelPosition="left" />

          {expr === null ? (
            <Stack gap="xs">
              <Text size="xs" c="dimmed">{t('schedules.noRestriction')}</Text>
              <Group gap={4}>
                <Button size="xs" variant="default" leftSection={<Plus size={11} />} onClick={() => setExpr(makeAndNode())} type="button">AND</Button>
                <Button size="xs" variant="default" leftSection={<Plus size={11} />} onClick={() => setExpr(makeOrNode())} type="button">OR</Button>
                <Button size="xs" variant="default" leftSection={<Plus size={11} />} onClick={() => setExpr(makeNotNode())} type="button">NOT</Button>
                <Button size="xs" variant="default" leftSection={<Plus size={11} />} onClick={() => setExpr(makeRuleNode())} type="button">{t('schedules.opRule')}</Button>
              </Group>
            </Stack>
          ) : (
            <Stack gap="xs">
              <ExprNodeEditor
                node={expr}
                onChange={setExpr}
                onDelete={() => setExpr(null)}
                depth={0}
                ruleTypeOptions={ruleTypeOptions}
              />
            </Stack>
          )}

          {saveError && <Alert color="red" variant="light" style={{ whiteSpace: 'pre-line' }}>{saveError}</Alert>}

          <Group justify="flex-end" pt="xs">
            <Button variant="default" onClick={onClose} type="button">{t('common.cancel')}</Button>
            <Button type="submit" loading={isSaving} disabled={!name.trim()}>{t('common.save')}</Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  )
}

function useScheduleForm() {
  const [editing, setEditing] = useState<AccessSchedule | null>(null)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [expr, setExpr] = useState<ExprNode | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [modalOpened, { open, close }] = useDisclosure(false)

  function openCreate() {
    setEditing(null); setName(''); setDescription(''); setExpr(null); setSaveError(null); open()
  }
  function openEdit(s: AccessSchedule) {
    setEditing(s); setName(s.name); setDescription(s.description ?? ''); setExpr(s.expr ?? null); setSaveError(null); open()
  }
  function closeModal() { close(); setSaveError(null) }

  return { editing, name, setName, description, setDescription, expr, setExpr, saveError, setSaveError, modalOpened, openCreate, openEdit, closeModal }
}

export default function SchedulesPage() {
  const { wsId } = useParams<{ wsId: string }>()
  const { t } = useTranslation()
  const qc = useQueryClient()

  // Determine current user's role (same pattern as WorkspacePage)
  const session = useAuthStore((s) => s.session)
  const globalAuth = session?.type === 'global'
  const localSession = !globalAuth && session?.type === 'local' ? session : null
  const ws = qc.getQueryData<WorkspaceWithRole[]>(['workspaces'])?.find((w) => w.id === wsId)
  const effectiveRole = globalAuth ? ws?.role : localSession?.role
  const isAdmin = effectiveRole === 'ADMIN' || effectiveRole === 'OWNER'

  const ruleTypeOptions = [
    { value: 'time_range', label: t('schedules.timeRange') },
    { value: 'weekdays_range', label: t('schedules.weekdaysRange') },
    { value: 'date_range', label: t('schedules.dateRange') },
    { value: 'day_of_month_range', label: t('schedules.dayOfMonthRange') },
    { value: 'month_range', label: t('schedules.monthRange') },
  ]

  // --- Workspace schedules (admin) ---
  const wsForm = useScheduleForm()

  const { data: wsSchedules = [], isLoading: wsLoading, isError: wsError, error: wsErr } = useQuery<AccessSchedule[]>({
    queryKey: ['schedules', wsId],
    queryFn: () => schedulesApi.list(wsId!),
    enabled: isAdmin,
  })

  const wsCreateMut = useMutation({
    mutationFn: () => schedulesApi.create(wsId!, { name: wsForm.name.trim(), description: wsForm.description.trim() || undefined, expr: wsForm.expr }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['schedules', wsId] }); wsForm.closeModal() },
    onError: (err) => wsForm.setSaveError(extractApiError(err)),
  })

  const wsUpdateMut = useMutation({
    mutationFn: () => schedulesApi.update(wsId!, wsForm.editing!.id, { name: wsForm.name.trim(), description: wsForm.description.trim() || undefined, expr: wsForm.expr }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['schedules', wsId] }); wsForm.closeModal() },
    onError: (err) => wsForm.setSaveError(extractApiError(err)),
  })

  const wsDeleteMut = useMutation({
    mutationFn: (id: string) => schedulesApi.delete(wsId!, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['schedules', wsId] }),
  })

  function handleWsSave(e: React.FormEvent) {
    e.preventDefault()
    wsForm.setSaveError(null)
    if (wsForm.editing) wsUpdateMut.mutate()
    else wsCreateMut.mutate()
  }

  // --- My schedules (personal) ---
  const myForm = useScheduleForm()

  const { data: mySchedules = [], isLoading: myLoading, isError: myError, error: myErr } = useQuery<AccessSchedule[]>({
    queryKey: ['member-schedules', wsId],
    queryFn: () => schedulesApi.listMine(wsId!),
  })

  const myCreateMut = useMutation({
    mutationFn: () => schedulesApi.createMine(wsId!, { name: myForm.name.trim(), description: myForm.description.trim() || undefined, expr: myForm.expr }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['member-schedules', wsId] }); myForm.closeModal() },
    onError: (err) => myForm.setSaveError(extractApiError(err)),
  })

  const myUpdateMut = useMutation({
    mutationFn: () => schedulesApi.updateMine(wsId!, myForm.editing!.id, { name: myForm.name.trim(), description: myForm.description.trim() || undefined, expr: myForm.expr }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['member-schedules', wsId] }); myForm.closeModal() },
    onError: (err) => myForm.setSaveError(extractApiError(err)),
  })

  const myDeleteMut = useMutation({
    mutationFn: (id: string) => schedulesApi.deleteMine(wsId!, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['member-schedules', wsId] }),
  })

  function handleMySave(e: React.FormEvent) {
    e.preventDefault()
    myForm.setSaveError(null)
    if (myForm.editing) myUpdateMut.mutate()
    else myCreateMut.mutate()
  }

  return (
    <Container size="sm" py="xl">
      <Group justify="space-between" mb="md">
        <div>
          <Title order={2}>{t('schedules.title')}</Title>
          <Text size="sm" c="dimmed">{t('schedules.subtitle')}</Text>
        </div>
      </Group>

      <Tabs defaultValue="personal">
        <Tabs.List mb="lg">
          <Tabs.Tab value="personal" leftSection={<User size={14} />}>
            {t('schedules.mySchedulesTitle')}
          </Tabs.Tab>
          <Tabs.Tab value="workspace" leftSection={<Lock size={14} />} disabled={!isAdmin}>
            {t('schedules.workspaceSchedulesTitle')}
          </Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="personal">
          <Stack gap="md">
            <Group justify="space-between">
              <Text size="sm" c="dimmed">{t('schedules.mySchedulesSubtitle')}</Text>
              <Button leftSection={<Plus size={14} />} size="sm" variant="default" onClick={myForm.openCreate}>
                {t('schedules.add')}
              </Button>
            </Group>
            {myLoading ? (
              <Center py="md"><Loader size="sm" /></Center>
            ) : myError ? (
              <QueryError error={myErr} />
            ) : mySchedules.length === 0 ? (
              <Paper withBorder p="lg" radius="md">
                <Center>
                  <Stack align="center" gap="xs">
                    <CalendarClock size={28} opacity={0.3} />
                    <Text c="dimmed" size="sm">{t('schedules.noSchedules')}</Text>
                    <Text c="dimmed" size="xs">{t('schedules.mySchedulesHint')}</Text>
                  </Stack>
                </Center>
              </Paper>
            ) : (
              <Stack gap="xs">
                {mySchedules.map((s) => (
                  <ScheduleCard key={s.id} s={s} onEdit={myForm.openEdit} onDelete={(id) => myDeleteMut.mutate(id)} isDeleting={myDeleteMut.isPending} t={t} />
                ))}
              </Stack>
            )}
          </Stack>
        </Tabs.Panel>

        <Tabs.Panel value="workspace">
          <Stack gap="md">
            <Group justify="space-between">
              <Text size="sm" c="dimmed">{t('schedules.workspaceSchedulesSubtitle')}</Text>
              <Button leftSection={<Plus size={14} />} size="sm" variant="default" onClick={wsForm.openCreate}>
                {t('schedules.add')}
              </Button>
            </Group>
            {wsLoading ? (
              <Center py="md"><Loader size="sm" /></Center>
            ) : wsError ? (
              <QueryError error={wsErr} />
            ) : wsSchedules.length === 0 ? (
              <Paper withBorder p="lg" radius="md">
                <Center>
                  <Stack align="center" gap="xs">
                    <CalendarClock size={28} opacity={0.3} />
                    <Text c="dimmed" size="sm">{t('schedules.noSchedules')}</Text>
                    <Text c="dimmed" size="xs">{t('schedules.workspaceSchedulesHint')}</Text>
                  </Stack>
                </Center>
              </Paper>
            ) : (
              <Stack gap="xs">
                {wsSchedules.map((s) => (
                  <ScheduleCard key={s.id} s={s} onEdit={wsForm.openEdit} onDelete={(id) => wsDeleteMut.mutate(id)} isDeleting={wsDeleteMut.isPending} t={t} />
                ))}
              </Stack>
            )}
          </Stack>
        </Tabs.Panel>
      </Tabs>

      {/* My schedules modal */}
      <ScheduleModal
        opened={myForm.modalOpened}
        onClose={myForm.closeModal}
        editing={myForm.editing}
        onSave={handleMySave}
        isSaving={myCreateMut.isPending || myUpdateMut.isPending}
        saveError={myForm.saveError}
        name={myForm.name}
        setName={myForm.setName}
        description={myForm.description}
        setDescription={myForm.setDescription}
        expr={myForm.expr}
        setExpr={myForm.setExpr}
        ruleTypeOptions={ruleTypeOptions}
        title={myForm.editing ? t('schedules.editSchedule') : t('schedules.addMySchedule')}
        t={t}
      />

      {/* Workspace schedules modal (admin) */}
      {isAdmin && (
        <ScheduleModal
          opened={wsForm.modalOpened}
          onClose={wsForm.closeModal}
          editing={wsForm.editing}
          onSave={handleWsSave}
          isSaving={wsCreateMut.isPending || wsUpdateMut.isPending}
          saveError={wsForm.saveError}
          name={wsForm.name}
          setName={wsForm.setName}
          description={wsForm.description}
          setDescription={wsForm.setDescription}
          expr={wsForm.expr}
          setExpr={wsForm.setExpr}
          ruleTypeOptions={ruleTypeOptions}
          title={wsForm.editing ? t('schedules.editSchedule') : t('schedules.addWorkspaceSchedule')}
          t={t}
        />
      )}
    </Container>
  )
}
