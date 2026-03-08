import { StrictMode, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { MantineProvider, createTheme, ColorSchemeScript, Center, Paper, Stack, Group, Avatar, Text, LoadingOverlay } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { DoorOpen } from 'lucide-react'
import '@mantine/core/styles.css'
import '@mantine/notifications/styles.css'
import './i18n'
import './index.css'
import { router } from './router'
import { setupApi } from './api'
import { useAuthStore } from './store/auth'
import SetupPage from './pages/setup/SetupPage'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
})

const theme = createTheme({
  primaryColor: 'indigo',
  defaultRadius: 'md',
  fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif',
  colors: {
    // Dark grey palette — zinc-900-ish, not pure black
    dark: [
      '#C1C2C5',
      '#A6A7AB',
      '#909296',
      '#5c5f66',
      '#373A40',
      '#2C2E33',
      '#25262b',
      '#1A1B1E',
      '#141517',
      '#101113',
    ],
  },
})

function AppRoot() {
  const initializing = useAuthStore((s) => s.initializing)
  const [hydrated, setHydrated] = useState(false)

  useEffect(() => {
    useAuthStore.getState().hydrate().finally(() => setHydrated(true))
  }, [])

  const { data, isLoading } = useQuery({
    queryKey: ['setup-status'],
    queryFn: setupApi.status,
    staleTime: Infinity,
    retry: false,
    enabled: hydrated,
  })

  if (!hydrated || initializing || isLoading) {
    return <LoadingOverlay visible />
  }

  if (data?.setup_required) {
    return (
      <Center mih="100vh" p="md">
        <Stack w="100%" maw={400} gap="xl">
          <Group justify="center" gap="xs">
            <Avatar size={32} color="indigo" radius="md">
              <DoorOpen size={16} />
            </Avatar>
            <Text fw={700} size="lg" ff="mono">GATIE</Text>
          </Group>
          <Paper p="xl" radius="lg" withBorder>
            <SetupPage />
          </Paper>
        </Stack>
      </Center>
    )
  }

  return <RouterProvider router={router} />
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ColorSchemeScript defaultColorScheme="auto" />
    <MantineProvider theme={theme} defaultColorScheme="auto">
      <Notifications position="top-right" />
      <QueryClientProvider client={queryClient}>
        <AppRoot />
      </QueryClientProvider>
    </MantineProvider>
  </StrictMode>,
)
