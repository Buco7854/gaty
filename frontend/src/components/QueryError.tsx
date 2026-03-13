import { useTranslation } from 'react-i18next'
import { extractApiError } from '@/lib/notify'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { AlertCircle } from 'lucide-react'

export function QueryError({ error, message }: { error: unknown; message?: string }) {
  const { t } = useTranslation()
  return (
    <Alert variant="destructive">
      <AlertCircle className="h-4 w-4" />
      <AlertDescription>
        {extractApiError(error, message ?? t('common.loadError'))}
      </AlertDescription>
    </Alert>
  )
}
