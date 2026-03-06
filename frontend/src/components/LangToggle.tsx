import { Menu, ActionIcon, Text } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { Languages } from 'lucide-react'

const LANGUAGES = [
  { code: 'fr', label: 'Français' },
  { code: 'en', label: 'English' },
]

export function LangToggle() {
  const { i18n } = useTranslation()
  const current = i18n.language.startsWith('fr') ? 'fr' : 'en'

  return (
    <Menu position="top-start" width={140} shadow="md" styles={{ dropdown: { padding: 4 }, item: { borderRadius: 'var(--mantine-radius-sm)', marginBottom: 2 } }}>
      <Menu.Target>
        <ActionIcon variant="subtle" color="gray" size="sm" title={current.toUpperCase()}>
          <Languages size={14} />
        </ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>
        {LANGUAGES.map((lang) => (
          <Menu.Item
            key={lang.code}
            onClick={() => i18n.changeLanguage(lang.code)}
            fw={current === lang.code ? 600 : undefined}
            c={current === lang.code ? 'indigo' : undefined}
            rightSection={current === lang.code ? <Text size="xs" c="indigo">✓</Text> : null}
          >
            {lang.label}
          </Menu.Item>
        ))}
      </Menu.Dropdown>
    </Menu>
  )
}
