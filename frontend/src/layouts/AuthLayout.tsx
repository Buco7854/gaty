import { Outlet } from 'react-router'
import { Center, Paper, Stack, Group, Avatar, Text } from '@mantine/core'
import { DoorOpen } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'

export default function AuthLayout() {
  return (
    <Center mih="100vh" p="md">
      <Stack w="100%" maw={400} gap="xl">
        <Group justify="space-between" align="center">
          <Group gap="xs">
            <Avatar size={32} color="indigo" radius="md">
              <DoorOpen size={16} />
            </Avatar>
            <Text fw={700} size="lg" ff="mono">GATIE</Text>
          </Group>
          <Group gap={4}>
            <LangToggle />
            <ThemeToggle />
          </Group>
        </Group>
        <Paper p="xl" radius="lg" withBorder>
          <Outlet />
        </Paper>
      </Stack>
    </Center>
  )
}
