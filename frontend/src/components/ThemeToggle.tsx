import { ActionIcon, Tooltip, Menu } from '@mantine/core'
import { useMantineColorScheme, useComputedColorScheme } from '@mantine/core'
import { Sun, Moon, Monitor } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export function ThemeToggle() {
  const { t } = useTranslation()
  const { setColorScheme } = useMantineColorScheme()
  const computed = useComputedColorScheme('light')

  const Icon = computed === 'dark' ? Moon : Sun

  return (
    <Menu shadow="md" width={140}>
      <Menu.Target>
        <Tooltip label={computed === 'dark' ? t('theme.dark') : t('theme.light')}>
          <ActionIcon variant="subtle" color="gray" size="sm">
            <Icon size={16} />
          </ActionIcon>
        </Tooltip>
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Item leftSection={<Sun size={14} />} onClick={() => setColorScheme('light')}>
          {t('theme.light')}
        </Menu.Item>
        <Menu.Item leftSection={<Moon size={14} />} onClick={() => setColorScheme('dark')}>
          {t('theme.dark')}
        </Menu.Item>
        <Menu.Item leftSection={<Monitor size={14} />} onClick={() => setColorScheme('auto')}>
          {t('theme.auto')}
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  )
}
