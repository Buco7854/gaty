import { useEffect, useRef } from 'react'
import { useNavigate, useLocation } from 'react-router'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/store/auth'
import { api } from '@/lib/api'
import { Loader2 } from 'lucide-react'

export default function SsoCallbackPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const setMemberSession = useAuthStore((s) => s.setMemberSession)
  const processed = useRef(false)

  useEffect(() => {
    if (processed.current) return
    processed.current = true

    const params = new URLSearchParams(location.search)
    const code = params.get('code')
    const state = params.get('state')
    const gateId = params.get('gate_id')

    if (!code || !state) {
      navigate('/login')
      return
    }

    api.post('/auth/sso/exchange', { code, state })
      .then((res) => {
        const member = res.data?.member
        if (member) {
          setMemberSession(member)
          if (gateId) {
            navigate(`/gates/${gateId}/public`)
          } else {
            navigate('/gates')
          }
        } else {
          navigate('/login')
        }
      })
      .catch(() => navigate('/login'))
  }, [location.search, navigate, setMemberSession])

  return (
    <div className="flex items-center justify-center min-h-screen">
      <div className="text-center space-y-4">
        <Loader2 className="h-6 w-6 animate-spin mx-auto text-muted-foreground" />
        <p className="text-sm text-muted-foreground">{t('common.loading')}</p>
      </div>
    </div>
  )
}
