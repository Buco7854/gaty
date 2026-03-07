import { Alert } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { extractApiError } from '@/lib/notify'

export function QueryError({ error, message }: { error: unknown; message?: string }) {
  const { t } = useTranslation()
  return (
    <Alert color="red" title={t('common.error')}>
      {extractApiError(error, message ?? t('common.loadError'))}
    </Alert>
  )
}
