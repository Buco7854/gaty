import { useTheme } from '@/components/ThemeProvider'
import { useTranslation } from 'react-i18next'
import { Sun, Moon, Monitor } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export function ThemeToggle() {
  const { t } = useTranslation()
  const { theme, setTheme, resolvedTheme } = useTheme()

  const Icon = resolvedTheme === 'dark' ? Moon : Sun

  const schemes = [
    { value: 'light' as const, label: t('theme.light'), icon: <Sun className="h-3.5 w-3.5" /> },
    { value: 'dark' as const, label: t('theme.dark'), icon: <Moon className="h-3.5 w-3.5" /> },
    { value: 'system' as const, label: t('theme.auto'), icon: <Monitor className="h-3.5 w-3.5" /> },
  ]

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm">
          <Icon className="h-3.5 w-3.5" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-36">
        {schemes.map(({ value, label, icon }) => (
          <DropdownMenuItem
            key={value}
            onClick={() => setTheme(value)}
            className={theme === value ? 'text-primary font-medium' : ''}
          >
            {icon}
            {label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
