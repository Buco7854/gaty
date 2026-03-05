import { useState } from 'react'
import { useNavigate } from 'react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { workspacesApi } from '@/api'
import type { WorkspaceWithRole } from '@/types'
import { useTranslation } from 'react-i18next'
import {
  Container, Title, Text, Group, Button, Modal, TextInput, Stack, Alert,
  Card, Avatar, Badge, Skeleton, Center,
} from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { Plus, Building2, ChevronRight, AlertCircle } from 'lucide-react'
import { extractApiError, notifySuccess } from '@/lib/notify'

export default function WorkspacesPage() {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const { t } = useTranslation()
  const [opened, { open, close }] = useDisclosure(false)
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)

  const { data: workspaces, isLoading } = useQuery<WorkspaceWithRole[]>({
    queryKey: ['workspaces'],
    queryFn: workspacesApi.list,
  })

  const create = useMutation({
    mutationFn: () => workspacesApi.create(name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['workspaces'] })
      close()
      setName('')
      setError(null)
      notifySuccess(t('common.created'))
    },
    onError: (err: unknown) => {
      setError(extractApiError(err, t('common.error')))
    },
  })

  function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    create.mutate()
  }

  return (
    <Container size="sm" py="xl">
      <Group justify="space-between" mb="xl">
        <div>
          <Title order={2}>{t('workspaces.title')}</Title>
          <Text c="dimmed" size="sm">{t('workspaces.subtitle')}</Text>
        </div>
        <Button leftSection={<Plus size={16} />} onClick={open}>
          {t('workspaces.new')}
        </Button>
      </Group>

      <Modal opened={opened} onClose={close} title={t('workspaces.create')}>
        <form onSubmit={handleCreate}>
          <Stack>
            <TextInput
              label={t('common.name')}
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              placeholder="My Building"
            />
            {error && (
              <Alert icon={<AlertCircle size={16} />} color="red" variant="light">
                {error}
              </Alert>
            )}
            <Group justify="flex-end">
              <Button variant="default" onClick={close}>{t('common.cancel')}</Button>
              <Button type="submit" loading={create.isPending}>{t('common.create')}</Button>
            </Group>
          </Stack>
        </form>
      </Modal>

      {isLoading ? (
        <Stack>
          {[0, 1, 2].map((i) => <Skeleton key={i} height={72} radius="md" />)}
        </Stack>
      ) : workspaces?.length === 0 ? (
        <Center py={80}>
          <Stack align="center" gap="xs">
            <Building2 size={40} opacity={0.3} />
            <Text fw={500}>{t('workspaces.noWorkspaces')}</Text>
            <Text size="sm" c="dimmed">{t('workspaces.noWorkspacesHint')}</Text>
          </Stack>
        </Center>
      ) : (
        <Stack gap="sm">
          {workspaces?.map((ws) => (
            <Card
              key={ws.id}
              withBorder
              padding="md"
              radius="md"
              style={{ cursor: 'pointer' }}
              onClick={() => navigate(`/workspaces/${ws.id}`)}
            >
              <Group justify="space-between">
                <Group>
                  <Avatar color="indigo" radius="md" size={40}>
                    {ws.name[0].toUpperCase()}
                  </Avatar>
                  <div>
                    <Text fw={600}>{ws.name}</Text>
                    <Group gap={4}>
                        <Badge size="xs" variant="light">{ws.role}</Badge>
                    </Group>
                  </div>
                </Group>
                <ChevronRight size={16} opacity={0.4} />
              </Group>
            </Card>
          ))}
        </Stack>
      )}
    </Container>
  )
}
