import { Box, Checkbox, Chip, Divider, Group, Paper, ScrollArea, Stack, Table, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import type { Gate } from '@/types'

export interface GatePerm {
  code: string
  label: string
}

/** Returns the standard 4 gate permissions with translated labels. */
export function useGatePermissions(): GatePerm[] {
  const { t } = useTranslation()
  return [
    { code: 'gate:read_status', label: t('permissions.viewStatus') },
    { code: 'gate:trigger_open', label: t('permissions.triggerOpen') },
    { code: 'gate:trigger_close', label: t('permissions.triggerClose') },
    { code: 'gate:manage', label: t('permissions.manage') },
  ]
}

interface GatePermissionsGridProps {
  gates: Gate[]
  permissions: GatePerm[]
  isChecked: (gateId: string, code: string) => boolean
  onToggle: (gateId: string, code: string) => void
  /** Add "select all" controls per column. */
  withColumnSelect?: boolean
  /** Constrain height and enable vertical scrolling (e.g. inside modals). */
  maxHeight?: number
}

export function GatePermissionsGrid({
  gates,
  permissions,
  isChecked,
  onToggle,
  withColumnSelect = false,
  maxHeight,
}: GatePermissionsGridProps) {
  const { t } = useTranslation()

  function handleColumnToggle(code: string) {
    const allOn = gates.every((g) => isChecked(g.id, code))
    for (const gate of gates) {
      if (allOn && isChecked(gate.id, code)) onToggle(gate.id, code)
      if (!allOn && !isChecked(gate.id, code)) onToggle(gate.id, code)
    }
  }

  // ── Mobile view: chip cards ─────────────────────────────────────────────────
  const mobileView = (
    <Stack gap="md">
      {/* Column-select row — clean, no extra card */}
      {withColumnSelect && (
        <Box>
          <Text size="xs" fw={600} mb={6}>{t('common.selectAll')}</Text>
          <Group gap={6} wrap="wrap">
            {permissions.map(({ code, label }) => {
              const count = gates.filter((g) => isChecked(g.id, code)).length
              const allOn = count === gates.length && gates.length > 0
              return (
                <Chip key={code} size="xs" checked={allOn} onChange={() => handleColumnToggle(code)}>
                  {label}
                </Chip>
              )
            })}
          </Group>
          <Divider mt="md" />
        </Box>
      )}

      {gates.map((gate) => (
        <Paper key={gate.id} withBorder p="md" radius="md">
          <Text size="sm" fw={600} mb={10} truncate>{gate.name}</Text>
          <Group gap={8} wrap="wrap">
            {permissions.map(({ code, label }) => (
              <Chip
                key={code}
                size="xs"
                checked={isChecked(gate.id, code)}
                onChange={() => onToggle(gate.id, code)}
              >
                {label}
              </Chip>
            ))}
          </Group>
        </Paper>
      ))}
    </Stack>
  )

  // ── Desktop view: table — no header background, uniform checkbox size ────────
  const tableInner = (
    <Table
      highlightOnHover
      withRowBorders
      withColumnBorders={false}
      withTableBorder={false}
      verticalSpacing={8}
      horizontalSpacing="md"
      fz="sm"
    >
      <Table.Thead
        style={{ borderBottom: '1px solid var(--mantine-color-default-border)' }}
      >
        <Table.Tr>
          <Table.Th style={{ minWidth: 130 }} />
          {permissions.map(({ code, label }) => {
            const count = gates.filter((g) => isChecked(g.id, code)).length
            const allChecked = count === gates.length && gates.length > 0
            const someChecked = count > 0 && !allChecked
            return (
              <Table.Th key={code} ta="center" style={{ width: 90 }}>
                {withColumnSelect ? (
                  <Stack gap={4} align="center">
                    <Text size="xs" fw={700} style={{ lineHeight: 1.3 }}>{label}</Text>
                    <Checkbox
                      size="xs"
                      checked={allChecked}
                      indeterminate={someChecked}
                      onChange={() => handleColumnToggle(code)}
                    />
                  </Stack>
                ) : (
                  <Text size="xs" fw={700}>{label}</Text>
                )}
              </Table.Th>
            )
          })}
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {gates.map((gate) => (
          <Table.Tr key={gate.id}>
            <Table.Td>
              <Text size="sm" fw={500} truncate maw={220}>{gate.name}</Text>
            </Table.Td>
            {permissions.map(({ code }) => (
              <Table.Td key={code}>
                <Group justify="center">
                  <Checkbox
                    size="xs"
                    checked={isChecked(gate.id, code)}
                    onChange={() => onToggle(gate.id, code)}
                  />
                </Group>
              </Table.Td>
            ))}
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  )

  const desktopView = (
    <Paper withBorder radius="md" style={{ overflow: 'hidden' }}>
      {maxHeight ? (
        <ScrollArea.Autosize mah={maxHeight}>{tableInner}</ScrollArea.Autosize>
      ) : (
        tableInner
      )}
    </Paper>
  )

  return (
    <>
      <Box hiddenFrom="sm">{mobileView}</Box>
      <Box visibleFrom="sm">{desktopView}</Box>
    </>
  )
}
