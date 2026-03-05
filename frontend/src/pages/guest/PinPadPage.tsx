import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { publicApi } from '@/api'
import type { GateSession } from '@/api/public'
import { useTranslation } from 'react-i18next'
import { Center, Stack, Group, Text, Title, Button, Anchor, PasswordInput } from '@mantine/core'
import { Delete } from 'lucide-react'
import { notifyError } from '@/lib/notify'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'

const DIGITS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', '', '0', '⌫']

function sessionKey(gateId: string) {
  return `gaty_session_${gateId}`
}

export default function PinPadPage() {
  const { wsId, gateId } = useParams<{ wsId?: string; gateId: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()

  const [gateName, setGateName] = useState<string | null>(null)
  const [code, setCode] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [usePassword, setUsePassword] = useState(false)

  const portalPath = wsId && gateId
    ? `/workspaces/${wsId}/gates/${gateId}/public`
    : gateId ? `/unlock/${gateId}` : '/'

  useEffect(() => {
    if (!gateId) return
    const raw = localStorage.getItem(sessionKey(gateId))
    if (raw) {
      try {
        const s = JSON.parse(raw) as GateSession
        if (s?.access_token) {
          navigate(portalPath, { replace: true })
          return
        }
      } catch { /* ignore */ }
    }
    publicApi.resolveByGateId(gateId)
      .then((data) => setGateName(data.gate_name))
      .catch(() => {})
  }, [gateId, navigate, portalPath])

  async function submitCode(value: string) {
    if (!gateId || value.length < 1 || submitting) return
    setSubmitting(true)
    try {
      const result = await publicApi.open(gateId, value)
      if (result.session) {
        const session: GateSession = { type: 'pin', ...result.session }
        localStorage.setItem(sessionKey(gateId), JSON.stringify(session))
      }
      navigate(portalPath, { replace: true, state: { justAuthenticated: true } })
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      const msg = status === 429 ? t('pinpad.tooManyAttempts')
        : (status === 401 || status === 403) ? t('pinpad.invalidPin')
        : t('pinpad.unreachable')
      notifyError(null, msg)
      setCode('')
    } finally {
      setSubmitting(false)
    }
  }

  function press(d: string) {
    if (submitting) return
    if (d === '⌫') {
      setCode((p) => p.slice(0, -1))
    } else {
      setCode((p) => p + d)
    }
  }

  function switchMode(password: boolean) {
    setCode('')
    setUsePassword(password)
  }

  return (
    <div style={{ position: 'relative', minHeight: '100vh' }}>
      <Group gap="xs" style={{ position: 'absolute', top: 12, right: 16, zIndex: 10 }}>
        <LangToggle />
        <ThemeToggle />
      </Group>

      <Center mih="100vh" p="md" style={{ userSelect: 'none' }}>
        <Stack align="center" gap="xl" w="100%" maw={320}>
          <Stack align="center" gap={4}>
            <Title order={2}>{gateName ?? 'Gate'}</Title>
            <Text size="sm" c="dimmed">{t('pinpad.enterPin')}</Text>
          </Stack>

          {usePassword ? (
            <>
              <form onSubmit={(e) => { e.preventDefault(); submitCode(code) }} style={{ width: '100%' }}>
                <Stack>
                  <PasswordInput
                    placeholder={t('pinpad.enterPin')}
                    value={code}
                    onChange={(e) => setCode(e.target.value)}
                    autoFocus
                    disabled={submitting}
                  />
                  {code.length > 0 && (
                    <Button type="submit" size="md" radius="xl" loading={submitting}>
                      {t('common.confirm')}
                    </Button>
                  )}
                </Stack>
              </form>
              <Anchor component="button" type="button" size="xs" c="dimmed" onClick={() => switchMode(false)}>
                {t('pinpad.usePinInstead')}
              </Anchor>
            </>
          ) : (
            <>
              {/* Dot indicator */}
              <Group gap="sm">
                {Array.from({ length: Math.max(code.length, 4) }).map((_, i) => (
                  <div
                    key={i}
                    style={{
                      width: 12,
                      height: 12,
                      borderRadius: '50%',
                      backgroundColor: i < code.length
                        ? 'var(--mantine-color-indigo-6)'
                        : 'var(--mantine-color-default-border)',
                      transition: 'background-color 150ms',
                    }}
                  />
                ))}
              </Group>

              {/* Numpad */}
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12, width: '100%' }}>
                {DIGITS.map((d, i) => {
                  if (d === '') return <div key={i} />
                  if (d === '⌫') {
                    return (
                      <button
                        key={i}
                        onPointerDown={() => press(d)}
                        disabled={submitting || code.length === 0}
                        style={{
                          aspectRatio: '1',
                          borderRadius: 16,
                          border: 'none',
                          backgroundColor: 'var(--mantine-color-default)',
                          color: 'var(--mantine-color-dimmed)',
                          cursor: 'pointer',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          transition: 'opacity 150ms',
                          opacity: submitting || code.length === 0 ? 0.3 : 1,
                        }}
                      >
                        <Delete size={20} />
                      </button>
                    )
                  }
                  return (
                    <button
                      key={i}
                      onPointerDown={() => press(d)}
                      disabled={submitting}
                      style={{
                        aspectRatio: '1',
                        borderRadius: 16,
                        border: '1px solid var(--mantine-color-default-border)',
                        backgroundColor: 'var(--mantine-color-body)',
                        fontSize: 22,
                        fontWeight: 600,
                        cursor: 'pointer',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        boxShadow: '0 1px 3px rgba(0,0,0,0.1)',
                        transition: 'opacity 150ms',
                        opacity: submitting ? 0.3 : 1,
                      }}
                    >
                      {d}
                    </button>
                  )
                })}
              </div>

              {code.length > 0 && (
                <Button onClick={() => submitCode(code)} size="md" radius="xl" px="xl" loading={submitting}>
                  {t('common.confirm')}
                </Button>
              )}
            </>
          )}

          <Anchor component="button" type="button" size="xs" c="dimmed" onClick={() => navigate(portalPath)}>
            {t('pinpad.useAnotherMethod')}
          </Anchor>
        </Stack>
      </Center>
    </div>
  )
}
