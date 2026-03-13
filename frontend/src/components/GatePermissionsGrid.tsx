import { useTranslation } from 'react-i18next'
import { Checkbox } from '@/components/ui/checkbox'
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
  maxHeight?: number
}

export function GatePermissionsGrid({
  gates,
  permissions,
  isChecked,
  onToggle,
  maxHeight,
}: GatePermissionsGridProps) {
  return (
    <div className="rounded-md border overflow-hidden" style={maxHeight ? { maxHeight, overflowY: 'auto' } : undefined}>
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b bg-muted/50">
            <th className="text-left p-2 font-medium min-w-[130px]" />
            {permissions.map(({ code, label }) => (
              <th key={code} className="text-center p-2 font-medium text-xs w-[80px]">{label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {gates.map((gate) => (
            <tr key={gate.id} className="border-b last:border-0 hover:bg-muted/30">
              <td className="p-2 font-medium truncate max-w-[200px]">{gate.name}</td>
              {permissions.map(({ code }) => (
                <td key={code} className="text-center p-2">
                  <Checkbox
                    checked={isChecked(gate.id, code)}
                    onCheckedChange={() => onToggle(gate.id, code)}
                  />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
