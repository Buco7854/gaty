import { ActionIcon, Menu, Text } from '@mantine/core'
import { useMantineColorScheme, useComputedColorScheme } from '@mantine/core'
import { Sun, Moon, Monitor } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export function ThemeToggle() {
  const { t } = useTranslation()
  const { colorScheme, setColorScheme } = useMantineColorScheme()
  const computed = useComputedColorScheme('light')

  const Icon = computed === 'dark' ? Moon : Sun

  const schemes = [
    { value: 'light', label: t('theme.light'), icon: <Sun size={14} /> },
    { value: 'dark', label: t('theme.dark'), icon: <Moon size={14} /> },
    { value: 'auto', label: t('theme.auto'), icon: <Monitor size={14} /> },
  ] as const

  return (
    <Menu
      position="top-start"
      width={140}
      shadow="md"
      styles={{ dropdown: { padding: 4 }, item: { borderRadius: 'var(--mantine-radius-sm)', marginBottom: 2 } }}
    >
      <Menu.Target>
        <ActionIcon variant="subtle" color="gray" size="sm" title={t(`theme.${colorScheme}`)}>
          <Icon size={14} />
        </ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>
        {schemes.map(({ value, label, icon }) => (
          <Menu.Item
            key={value}
            leftSection={icon}
            onClick={() => setColorScheme(value)}
            fw={colorScheme === value ? 600 : undefined}
            c={colorScheme === value ? 'indigo' : undefined}
            rightSection={colorScheme === value ? <Text size="xs" c="indigo">✓</Text> : null}
          >
            {label}
          </Menu.Item>
        ))}
      </Menu.Dropdown>
    </Menu>
  )
}
