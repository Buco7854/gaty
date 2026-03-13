import { StrictMode, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router'
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query'
import { Toaster } from 'sonner'
import { DoorOpen, Loader2 } from 'lucide-react'
import './i18n'
import './index.css'
import { router } from './router'
import { setupApi } from './api'
import { useAuthStore } from './store/auth'
import { ErrorBoundary } from './components/ErrorBoundary'
import { ThemeProvider } from './components/ThemeProvider'
import { TooltipProvider } from './components/ui/tooltip'
import SetupPage from './pages/setup/SetupPage'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
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
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (data?.setup_required) {
    return (
      <div className="flex items-center justify-center min-h-screen p-4">
        <div className="w-full max-w-md space-y-8">
          <div className="flex items-center justify-center gap-2">
            <div className="flex items-center justify-center h-8 w-8 rounded-md bg-primary/10 text-primary">
              <DoorOpen className="h-4 w-4" />
            </div>
            <span className="font-bold text-lg font-mono">GATIE</span>
          </div>
          <div className="border rounded-lg p-6">
            <SetupPage />
          </div>
        </div>
      </div>
    )
  }

  return <RouterProvider router={router} />
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider>
      <TooltipProvider>
        <Toaster position="top-right" richColors closeButton />
        <ErrorBoundary>
          <QueryClientProvider client={queryClient}>
            <AppRoot />
          </QueryClientProvider>
        </ErrorBoundary>
      </TooltipProvider>
    </ThemeProvider>
  </StrictMode>,
)
