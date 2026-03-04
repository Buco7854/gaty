import { ActionIcon, Tooltip } from '@mantine/core'
import { useTranslation } from 'react-i18next'

export function LangToggle() {
  const { i18n } = useTranslation()
  const current = i18n.language.startsWith('fr') ? 'fr' : 'en'

  function toggle() {
    i18n.changeLanguage(current === 'fr' ? 'en' : 'fr')
  }

  return (
    <Tooltip label={current === 'fr' ? 'Switch to English' : 'Passer en français'}>
      <ActionIcon variant="subtle" color="gray" size="sm" onClick={toggle}>
        <span style={{ fontSize: 13, fontWeight: 600 }}>{current.toUpperCase()}</span>
      </ActionIcon>
    </Tooltip>
  )
}
