import { createBrowserRouter, Navigate, Outlet } from 'react-router'
import { useAuthStore } from '@/store/auth'
import AppLayout from '@/layouts/AppLayout'
import AuthLayout from '@/layouts/AuthLayout'
import LoginPage from '@/pages/auth/LoginPage'
import RegisterPage from '@/pages/auth/RegisterPage'
import WorkspacesPage from '@/pages/workspaces/WorkspacesPage'
import WorkspacePage from '@/pages/workspace/WorkspacePage'
import GatePage from '@/pages/gate/GatePage'
import MembersPage from '@/pages/workspace/MembersPage'
import SettingsPage from '@/pages/workspace/SettingsPage'
import PinPadPage from '@/pages/guest/PinPadPage'

function RequireAuth() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  if (!isAuthenticated()) return <Navigate to="/login" replace />
  return <Outlet />
}

function RequireGuest() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  if (isAuthenticated()) return <Navigate to="/workspaces" replace />
  return <Outlet />
}

export const router = createBrowserRouter([
  // Auth routes (only for unauthenticated users)
  {
    element: <RequireGuest />,
    children: [
      {
        element: <AuthLayout />,
        children: [
          { path: '/login', element: <LoginPage /> },
          { path: '/register', element: <RegisterPage /> },
        ],
      },
    ],
  },
  // Protected app routes
  {
    element: <RequireAuth />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { path: '/', element: <Navigate to="/workspaces" replace /> },
          { path: '/workspaces', element: <WorkspacesPage /> },
          { path: '/workspaces/:wsId', element: <WorkspacePage /> },
          { path: '/workspaces/:wsId/members', element: <MembersPage /> },
          { path: '/workspaces/:wsId/settings', element: <SettingsPage /> },
          { path: '/workspaces/:wsId/gates/:gateId', element: <GatePage /> },
        ],
      },
    ],
  },
  // Public guest PIN pad (no auth required)
  { path: '/unlock', element: <PinPadPage /> },
  { path: '/unlock/:gateId', element: <PinPadPage /> },
])
