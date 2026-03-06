import { useState } from 'react'
import { useParams, useNavigate } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { workspacesApi } from '@/api'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Stack, Paper, Group, Button, TextInput, PasswordInput, Select, Alert,
  Switch, Divider, ActionIcon, Badge, Modal, NumberInput, Loader, Center, Collapse, Anchor,
} from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { KeyRound, Save, CheckCircle2, Plus, Trash2, Pencil, AlertTriangle } from 'lucide-react'
import { notifySuccess, notifyError } from '@/lib/notify'

interface RoleMappingEntry {
  claim: string
  role: string
}

interface SSOProvider {
  id: string
  name: string
  type: string
  client_id: string
  client_secret: string
  issuer: string
  scopes: string[]
  auth_endpoint: string
  token_endpoint: string
  jwks_uri: string
  auto_provision: boolean
  default_role: string
  role_claim: string
  role_mapping: Record<string, string>
}

const ROLE_OPTIONS = [
  { value: 'MEMBER', label: 'Member' },
  { value: 'ADMIN', label: 'Admin' },
  { value: 'OWNER', label: 'Owner' },
]

const PROVIDER_TYPE_OPTIONS = [
  { value: 'oidc', label: 'OIDC' },
]

function ProviderModal({
  opened,
  onClose,
  initial,
  onSave,
}: {
  opened: boolean
  onClose: () => void
  initial: SSOProvider | null
  onSave: (p: SSOProvider) => void
}) {
  const { t } = useTranslation()
  const isNew = !initial?.id

  const [name, setName] = useState(initial?.name ?? '')
  const [type, setType] = useState(initial?.type ?? 'oidc')
  const [issuer, setIssuer] = useState(initial?.issuer ?? '')
  const [clientId, setClientId] = useState(initial?.client_id ?? '')
  const [clientSecret, setClientSecret] = useState('')
  const [scopes, setScopes] = useState((initial?.scopes ?? []).join(' '))
  const [authEndpoint, setAuthEndpoint] = useState(initial?.auth_endpoint ?? '')
  const [tokenEndpoint, setTokenEndpoint] = useState(initial?.token_endpoint ?? '')
  const [jwksUri, setJwksUri] = useState(initial?.jwks_uri ?? '')
  const [advancedOpen, setAdvancedOpen] = useState(!!(initial?.auth_endpoint))
  const [autoProvision, setAutoProvision] = useState(initial?.auto_provision ?? false)
  const [defaultRole, setDefaultRole] = useState(initial?.default_role ?? 'MEMBER')
  const [roleClaim, setRoleClaim] = useState(initial?.role_claim ?? '')
  const [roleMapping, setRoleMapping] = useState<RoleMappingEntry[]>(
    Object.entries(initial?.role_mapping ?? {}).map(([claim, role]) => ({ claim, role }))
  )

  function addRoleMappingEntry() {
    setRoleMapping((prev) => [...prev, { claim: '', role: 'MEMBER' }])
  }

  function removeRoleMappingEntry(i: number) {
    setRoleMapping((prev) => prev.filter((_, idx) => idx !== i))
  }

  function updateRoleMappingEntry(i: number, field: 'claim' | 'role', value: string) {
    setRoleMapping((prev) => prev.map((e, idx) => idx === i ? { ...e, [field]: value } : e))
  }

  function handleSave() {
    const provider: SSOProvider = {
      id: initial?.id || crypto.randomUUID().slice(0, 8),
      name,
      type,
      issuer,
      client_id: clientId,
      client_secret: clientSecret || (initial?.client_secret ? '***' : ''),
      scopes: scopes.split(/\s+/).filter(Boolean),
      auth_endpoint: authEndpoint,
      token_endpoint: tokenEndpoint,
      jwks_uri: jwksUri,
      auto_provision: autoProvision,
      default_role: defaultRole,
      role_claim: roleClaim,
      role_mapping: Object.fromEntries(
        roleMapping.filter((m) => m.claim.trim()).map((m) => [m.claim.trim(), m.role])
      ),
    }
    onSave(provider)
    onClose()
  }

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={isNew ? t('settings.addProvider') : t('settings.editProvider')}
      size="md"
    >
      <Stack>
        <TextInput
          label={t('settings.providerName')}
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t('settings.providerNamePlaceholder')}
          required
        />
        <Select
          label={t('settings.providerType')}
          value={type}
          onChange={(v) => setType(v ?? 'oidc')}
          data={PROVIDER_TYPE_OPTIONS}
        />

        {type === 'oidc' && (
          <>
            <TextInput
              label={t('settings.issuerUrl')}
              value={issuer}
              onChange={(e) => setIssuer(e.target.value)}
              placeholder="https://accounts.google.com"
            />
            <TextInput
              label={t('settings.clientId')}
              value={clientId}
              onChange={(e) => setClientId(e.target.value)}
              placeholder="your-client-id"
            />
            <PasswordInput
              label={t('settings.clientSecret')}
              value={clientSecret}
              onChange={(e) => setClientSecret(e.target.value)}
              placeholder={isNew ? '' : t('settings.clientSecretPlaceholder')}
            />
            <TextInput
              label={t('settings.providerScopes')}
              description={t('common.optional')}
              value={scopes}
              onChange={(e) => setScopes(e.target.value)}
              placeholder={t('settings.providerScopesPlaceholder')}
            />
            <Anchor
              component="button"
              type="button"
              size="xs"
              c="dimmed"
              onClick={() => setAdvancedOpen((o) => !o)}
            >
              {t('settings.advancedEndpoints')} {advancedOpen ? '▲' : '▼'}
            </Anchor>
            <Collapse in={advancedOpen}>
              <Stack gap="xs">
                <TextInput
                  label={t('settings.authEndpoint')}
                  value={authEndpoint}
                  onChange={(e) => setAuthEndpoint(e.target.value)}
                  placeholder="https://provider.com/oauth/authorize"
                />
                <TextInput
                  label={t('settings.tokenEndpoint')}
                  value={tokenEndpoint}
                  onChange={(e) => setTokenEndpoint(e.target.value)}
                  placeholder="https://provider.com/oauth/token"
                />
                <TextInput
                  label={t('settings.jwksUri')}
                  value={jwksUri}
                  onChange={(e) => setJwksUri(e.target.value)}
                  placeholder="https://provider.com/.well-known/jwks.json"
                />
              </Stack>
            </Collapse>
          </>
        )}

        <Divider my="xs" />

        <Switch
          label={t('settings.autoProvision')}
          description={t('settings.autoProvisionHint')}
          checked={autoProvision}
          onChange={(e) => setAutoProvision(e.currentTarget.checked)}
        />
        <Select
          label={t('settings.defaultRole')}
          value={defaultRole}
          onChange={(v) => setDefaultRole(v ?? 'MEMBER')}
          data={ROLE_OPTIONS}
        />

        <Divider my="xs" />

        <TextInput
          label={t('settings.roleClaim')}
          description={t('settings.roleClaimHint')}
          value={roleClaim}
          onChange={(e) => setRoleClaim(e.target.value)}
          placeholder="groups"
        />

        <Stack gap="xs">
          <Group justify="space-between">
            <Text size="sm" fw={500}>{t('settings.roleMapping')}</Text>
            <Button size="xs" variant="subtle" leftSection={<Plus size={14} />} onClick={addRoleMappingEntry} type="button">
              {t('settings.addRoleMapping')}
            </Button>
          </Group>
          {roleMapping.length === 0 && (
            <Text size="xs" c="dimmed">{t('settings.roleMappingHint')}</Text>
          )}
          {roleMapping.map((entry, i) => (
            <Group key={i} gap="xs" align="flex-end">
              <TextInput
                placeholder={t('settings.claimValue')}
                value={entry.claim}
                onChange={(e) => updateRoleMappingEntry(i, 'claim', e.target.value)}
                style={{ flex: 1 }}
                size="xs"
              />
              <Select
                value={entry.role}
                onChange={(v) => updateRoleMappingEntry(i, 'role', v ?? 'MEMBER')}
                data={ROLE_OPTIONS}
                size="xs"
                style={{ width: 110 }}
              />
              <ActionIcon variant="subtle" color="red" size="sm" onClick={() => removeRoleMappingEntry(i)} type="button">
                <Trash2 size={14} />
              </ActionIcon>
            </Group>
          ))}
        </Stack>

        <Group justify="flex-end" mt="sm">
          <Button variant="default" onClick={onClose}>{t('common.cancel')}</Button>
          <Button onClick={handleSave} disabled={!name.trim()}>{t('common.save')}</Button>
        </Group>
      </Stack>
    </Modal>
  )
}

export default function SettingsPage() {
  const { wsId } = useParams<{ wsId: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { t } = useTranslation()
  const [saved, setSaved] = useState(false)
  const [macSaved, setMacSaved] = useState(false)
  const [modalOpened, { open: openModal, close: closeModal }] = useDisclosure(false)
  const [editingProvider, setEditingProvider] = useState<SSOProvider | null>(null)
  const [modalKey, setModalKey] = useState(0)

  // Rename
  const [newName, setNewName] = useState('')

  // Delete workspace
  const [deleteModalOpened, { open: openDeleteModal, close: closeDeleteModal }] = useDisclosure(false)
  const [deleteConfirmName, setDeleteConfirmName] = useState('')

  const { data: wsData } = useQuery({
    queryKey: ['workspace', wsId],
    queryFn: () => workspacesApi.get(wsId!),
    enabled: !!wsId,
  })

  const renameWorkspace = useMutation({
    mutationFn: (name: string) => workspacesApi.rename(wsId!, name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['workspaces'] })
      qc.invalidateQueries({ queryKey: ['workspace', wsId] })
      setNewName('')
      notifySuccess(t('common.saved'))
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const deleteWorkspace = useMutation({
    mutationFn: () => workspacesApi.delete(wsId!),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['workspaces'] })
      navigate('/workspaces')
    },
    onError: (err: unknown) => notifyError(err, t('common.error')),
  })

  const { data: ssoData, isLoading: ssoLoading } = useQuery({
    queryKey: ['sso-settings', wsId],
    queryFn: () => workspacesApi.getSsoSettings(wsId!),
    enabled: !!wsId,
  })

  const { data: macData, isLoading: macLoading } = useQuery({
    queryKey: ['member-auth-config', wsId],
    queryFn: () => workspacesApi.getMemberAuthConfig(wsId!),
    enabled: !!wsId,
  })

  const providers: SSOProvider[] = (ssoData?.providers ?? []) as SSOProvider[]

  const [passwordAuth, setPasswordAuth] = useState<boolean>(true)
  const [ssoAuth, setSsoAuth] = useState<boolean>(false)
  const [apiTokenAuth, setApiTokenAuth] = useState<boolean>(false)
  const [apiTokenMax, setApiTokenMax] = useState<number>(5)
  const [sessionDuration, setSessionDuration] = useState<number | string>('')

  // Sync MAC state once data is loaded (only on first load)
  const [macInitialized, setMacInitialized] = useState(false)
  if (macData && !macInitialized) {
    setPasswordAuth((macData.password as boolean) ?? true)
    setSsoAuth((macData.sso as boolean) ?? false)
    setApiTokenAuth((macData.api_token as boolean) ?? false)
    setApiTokenMax((macData.api_token_max as number) ?? 5)
    if (macData.session_duration != null) setSessionDuration(macData.session_duration as number)
    setMacInitialized(true)
  }

  const updateSSO = useMutation({
    mutationFn: (body: Record<string, unknown>) => workspacesApi.updateSsoSettings(wsId!, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['sso-settings', wsId] })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    },
  })

  const updateMAC = useMutation({
    mutationFn: (body: Record<string, unknown>) => workspacesApi.updateMemberAuthConfig(wsId!, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['member-auth-config', wsId] })
      setMacSaved(true)
      setTimeout(() => setMacSaved(false), 2000)
    },
  })

  function openAddProvider() {
    setEditingProvider(null)
    setModalKey((k) => k + 1)
    openModal()
  }

  function openEditProvider(p: SSOProvider) {
    setEditingProvider(p)
    setModalKey((k) => k + 1)
    openModal()
  }

  function handleSaveProvider(p: SSOProvider) {
    const updated = editingProvider
      ? providers.map((existing) => existing.id === editingProvider.id ? p : existing)
      : [...providers, p]
    updateSSO.mutate({ providers: updated })
  }

  function handleDeleteProvider(id: string) {
    updateSSO.mutate({ providers: providers.filter((p) => p.id !== id) })
  }

  function handleSaveMemberAuth() {
    const dur = typeof sessionDuration === 'number' ? sessionDuration : (sessionDuration === '' ? null : parseInt(String(sessionDuration), 10))
    updateMAC.mutate({
      password: passwordAuth,
      sso: ssoAuth,
      api_token: apiTokenAuth,
      api_token_max: apiTokenMax,
      session_duration: dur,
    })
  }

  if (ssoLoading || macLoading) {
    return (
      <Center py="xl">
        <Loader />
      </Center>
    )
  }

  return (
    <Container size="sm" py="xl">
      <Stack mb="xl" gap={4}>
        <Title order={2}>{t('settings.title')}</Title>
        <Text c="dimmed" size="sm">{t('settings.subtitle')}</Text>
      </Stack>

      {/* SSO Providers */}
      <Paper withBorder p="lg" radius="md" mb="md">
        <Group justify="space-between" mb="md">
          <Group gap="xs">
            <KeyRound size={18} opacity={0.6} />
            <Text fw={600}>{t('settings.sso')}</Text>
          </Group>
          <Button size="xs" variant="light" leftSection={<Plus size={14} />} onClick={openAddProvider}>
            {t('settings.addProvider')}
          </Button>
        </Group>

        {providers.length === 0 && (
          <Text size="sm" c="dimmed">{t('settings.noProviders')}</Text>
        )}

        <Stack gap="xs">
          {providers.map((p) => (
            <Paper key={p.id} withBorder p="sm" radius="sm">
              <Group justify="space-between">
                <Group gap="sm">
                  <Text size="sm" fw={500}>{p.name}</Text>
                  <Badge size="xs" variant="light">{p.type.toUpperCase()}</Badge>
                </Group>
                <Group gap={4}>
                  <ActionIcon variant="subtle" size="sm" onClick={() => openEditProvider(p)}>
                    <Pencil size={14} />
                  </ActionIcon>
                  <ActionIcon variant="subtle" color="red" size="sm" onClick={() => handleDeleteProvider(p.id)}>
                    <Trash2 size={14} />
                  </ActionIcon>
                </Group>
              </Group>
              {p.issuer && (
                <Text size="xs" c="dimmed" mt={4}>{p.issuer}</Text>
              )}
            </Paper>
          ))}
        </Stack>

        {saved && (
          <Alert icon={<CheckCircle2 size={16} />} color="green" variant="light" mt="md">
            {t('settings.saved')}
          </Alert>
        )}
      </Paper>

      {/* Member auth defaults */}
      <Paper withBorder p="lg" radius="md">
        <Stack gap="xs" mb="md">
          <Text fw={600}>{t('settings.memberAuthDefaults')}</Text>
          <Text size="xs" c="dimmed">{t('settings.memberAuthDefaultsHint')}</Text>
        </Stack>

        <Stack gap="md">
          <Switch
            label={t('settings.passwordAuth')}
            checked={passwordAuth}
            onChange={(e) => setPasswordAuth(e.currentTarget.checked)}
          />
          <Switch
            label={t('settings.ssoAuth')}
            checked={ssoAuth}
            onChange={(e) => setSsoAuth(e.currentTarget.checked)}
          />
          <Switch
            label={t('settings.apiTokenAuth')}
            checked={apiTokenAuth}
            onChange={(e) => setApiTokenAuth(e.currentTarget.checked)}
          />
          {apiTokenAuth && (
            <NumberInput
              label={t('settings.apiTokenMax')}
              value={apiTokenMax}
              onChange={(v) => setApiTokenMax(Number(v) || 5)}
              min={1}
              max={100}
              w={120}
            />
          )}
          <NumberInput
            label={t('settings.memberSessionDuration')}
            description={t('settings.memberSessionDurationHint')}
            value={sessionDuration}
            onChange={setSessionDuration}
            min={0}
            step={3600}
            placeholder={t('settings.memberSessionDurationPlaceholder')}
            w={200}
          />
        </Stack>

        {macSaved && (
          <Alert icon={<CheckCircle2 size={16} />} color="green" variant="light" mt="md">
            {t('settings.saved')}
          </Alert>
        )}

        <Group mt="md">
          <Button
            onClick={handleSaveMemberAuth}
            loading={updateMAC.isPending}
            leftSection={<Save size={16} />}
          >
            {t('settings.saveSso')}
          </Button>
        </Group>
      </Paper>

      {/* Danger zone */}
      <Paper withBorder p="lg" radius="md" mt="md" style={{ borderColor: 'var(--mantine-color-red-6)' }}>
        <Group gap="xs" mb="md">
          <AlertTriangle size={18} color="var(--mantine-color-red-6)" />
          <Text fw={600} c="red">{t('settings.dangerZone')}</Text>
        </Group>

        <Stack gap="lg">
          {/* Rename */}
          <div>
            <Text size="sm" fw={500} mb={4}>{t('settings.renameWorkspace')}</Text>
            <Text size="xs" c="dimmed" mb="xs">{t('settings.renameWorkspaceHint')}</Text>
            <Group gap="xs" align="flex-end">
              <TextInput
                placeholder={wsData?.name ?? ''}
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                size="sm"
                style={{ flex: 1 }}
                label={t('settings.newWorkspaceName')}
              />
              <Button
                size="sm"
                variant="light"
                leftSection={<Save size={14} />}
                loading={renameWorkspace.isPending}
                disabled={!newName.trim() || newName.trim() === wsData?.name}
                onClick={() => renameWorkspace.mutate(newName.trim())}
                mb={1}
              >
                {t('common.save')}
              </Button>
            </Group>
          </div>

          <Divider />

          {/* Delete */}
          <div>
            <Text size="sm" fw={500} mb={4}>{t('settings.deleteWorkspace')}</Text>
            <Text size="xs" c="dimmed" mb="xs">{t('settings.deleteWorkspaceHint')}</Text>
            <Button
              color="red"
              variant="light"
              leftSection={<Trash2 size={14} />}
              onClick={openDeleteModal}
            >
              {t('settings.deleteWorkspace')}
            </Button>
          </div>
        </Stack>
      </Paper>

      <ProviderModal
        key={modalKey}
        opened={modalOpened}
        onClose={closeModal}
        initial={editingProvider}
        onSave={handleSaveProvider}
      />

      {/* Delete confirmation modal */}
      <Modal
        opened={deleteModalOpened}
        onClose={() => { closeDeleteModal(); setDeleteConfirmName('') }}
        title={<Text fw={600} c="red">{t('settings.deleteWorkspaceConfirm')}</Text>}
        size="sm"
      >
        <Stack>
          <Text size="sm">{t('settings.deleteWorkspaceConfirmHint')}</Text>
          <Text size="sm" fw={600} ff="mono">{wsData?.name}</Text>
          <TextInput
            placeholder={wsData?.name ?? ''}
            value={deleteConfirmName}
            onChange={(e) => setDeleteConfirmName(e.target.value)}
          />
          <Group justify="flex-end">
            <Button variant="default" onClick={() => { closeDeleteModal(); setDeleteConfirmName('') }}>
              {t('common.cancel')}
            </Button>
            <Button
              color="red"
              loading={deleteWorkspace.isPending}
              disabled={deleteConfirmName !== wsData?.name}
              leftSection={<Trash2 size={14} />}
              onClick={() => deleteWorkspace.mutate()}
            >
              {t('settings.deleteWorkspaceConfirm')}
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Container>
  )
}
