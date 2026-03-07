import { useState } from 'react'
import type { AxiosError } from 'axios'
import { useParams } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { schedulesApi } from '@/api'
import type { AccessSchedule, ExprNode, ScheduleRule } from '@/types'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Stack, Paper, Group, Button, TextInput, ActionIcon,
  Badge, Modal, Select, NumberInput, Divider, Alert, Loader, Center,
  SimpleGrid, SegmentedControl, Checkbox,
} from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { Plus, Trash2, Pencil, CalendarClock } from 'lucide-react'
import { QueryError } from '@/components/QueryError'

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

export default function SchedulesPage() {
  const { wsId } = useParams<{ wsId: string }>()
  const { t } = useTranslation()
  const qc = useQueryClient()

  const { data: schedules = [], isLoading, isError, error } = useQuery<AccessSchedule[]>({
    queryKey: ['schedules', wsId],
    queryFn: () => schedulesApi.list(wsId!),
  })

  const [modalOpened, { open, close }] = useDisclosure(false)
  const [editing, setEditing] = useState<AccessSchedule | null>(null)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [expr, setExpr] = useState<ExprNode | null>(null)
  const [saveError, setSaveError] = useState<string | null>(null)

  const ruleTypeOptions = [
    { value: 'time_range', label: t('schedules.timeRange') },
    { value: 'weekdays_range', label: t('schedules.weekdaysRange') },
    { value: 'date_range', label: t('schedules.dateRange') },
    { value: 'day_of_month_range', label: t('schedules.dayOfMonthRange') },
    { value: 'month_range', label: t('schedules.monthRange') },
  ]

  const createMut = useMutation({
    mutationFn: () =>
      schedulesApi.create(wsId!, { name: name.trim(), description: description.trim() || undefined, expr }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['schedules', wsId] }); closeModal() },
    onError: (err) => setSaveError(extractApiError(err)),
  })

  const updateMut = useMutation({
    mutationFn: () =>
      schedulesApi.update(wsId!, editing!.id, { name: name.trim(), description: description.trim() || undefined, expr }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['schedules', wsId] }); closeModal() },
    onError: (err) => setSaveError(extractApiError(err)),
  })

  const deleteMut = useMutation({
    mutationFn: (id: string) => schedulesApi.delete(wsId!, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['schedules', wsId] }),
  })

  function openCreate() {
    setEditing(null)
    setName('')
    setDescription('')
    setExpr(null)
    setSaveError(null)
    open()
  }

  function openEdit(s: AccessSchedule) {
    setEditing(s)
    setName(s.name)
    setDescription(s.description ?? '')
    setExpr(s.expr ?? null)
    setSaveError(null)
    open()
  }

  function closeModal() {
    close()
    setSaveError(null)
  }

  function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaveError(null)
    if (editing) updateMut.mutate()
    else createMut.mutate()
  }

  const isSaving = createMut.isPending || updateMut.isPending

  return (
    <Container size="sm" py="xl">
      <Group justify="space-between" mb="lg">
        <div>
          <Title order={3}>{t('schedules.title')}</Title>
          <Text size="sm" c="dimmed">{t('schedules.subtitle')}</Text>
        </div>
        <Button leftSection={<Plus size={14} />} size="sm" onClick={openCreate}>
          {t('schedules.add')}
        </Button>
      </Group>

      {isLoading ? (
        <Center py="xl"><Loader size="sm" /></Center>
      ) : isError ? (
        <QueryError error={error} />
      ) : schedules.length === 0 ? (
        <Paper withBorder p="xl" radius="md">
          <Center>
            <Stack align="center" gap="xs">
              <CalendarClock size={32} opacity={0.3} />
              <Text c="dimmed" size="sm">{t('schedules.noSchedules')}</Text>
              <Text c="dimmed" size="xs">{t('schedules.noSchedulesHint')}</Text>
            </Stack>
          </Center>
        </Paper>
      ) : (
        <Stack gap="xs">
          {schedules.map((s) => (
            <Paper key={s.id} withBorder p="md" radius="md">
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
                  <ActionIcon size="sm" variant="subtle" onClick={() => openEdit(s)}>
                    <Pencil size={14} />
                  </ActionIcon>
                  <ActionIcon
                    size="sm"
                    variant="subtle"
                    color="red"
                    loading={deleteMut.isPending}
                    onClick={() => deleteMut.mutate(s.id)}
                  >
                    <Trash2 size={14} />
                  </ActionIcon>
                </Group>
              </Group>
            </Paper>
          ))}
        </Stack>
      )}

      <Modal
        opened={modalOpened}
        onClose={closeModal}
        title={editing ? t('schedules.editSchedule') : t('schedules.add')}
        size="lg"
      >
        <form onSubmit={handleSave}>
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
                  <Button
                    size="xs"
                    variant="default"
                    leftSection={<Plus size={11} />}
                    onClick={() => setExpr(makeAndNode())}
                    type="button"
                  >
                    AND
                  </Button>
                  <Button
                    size="xs"
                    variant="default"
                    leftSection={<Plus size={11} />}
                    onClick={() => setExpr(makeOrNode())}
                    type="button"
                  >
                    OR
                  </Button>
                  <Button
                    size="xs"
                    variant="default"
                    leftSection={<Plus size={11} />}
                    onClick={() => setExpr(makeNotNode())}
                    type="button"
                  >
                    NOT
                  </Button>
                  <Button
                    size="xs"
                    variant="default"
                    leftSection={<Plus size={11} />}
                    onClick={() => setExpr(makeRuleNode())}
                    type="button"
                  >
                    {t('schedules.opRule')}
                  </Button>
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
              <Button variant="default" onClick={closeModal} type="button">
                {t('common.cancel')}
              </Button>
              <Button type="submit" loading={isSaving} disabled={!name.trim()}>
                {t('common.save')}
              </Button>
            </Group>
          </Stack>
        </form>
      </Modal>
    </Container>
  )
}
