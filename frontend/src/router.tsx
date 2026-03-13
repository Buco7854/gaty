import { createBrowserRouter, Navigate, Outlet } from 'react-router'
import { useAuthStore } from '@/store/auth'
import AppLayout from '@/layouts/AppLayout'
import AuthLayout from '@/layouts/AuthLayout'
import LoginPage from '@/pages/auth/LoginPage'
import GatePage from '@/pages/gate/GatePage'
import MembersPage from '@/pages/workspace/MembersPage'
import SchedulesPage from '@/pages/workspace/SchedulesPage'
import SettingsPage from '@/pages/workspace/SettingsPage'
import DashboardPage from '@/pages/workspace/WorkspacePage'
import GatePortalPage from '@/pages/guest/GatePortalPage'
import PinPadPage from '@/pages/guest/PinPadPage'
import PasswordAccessPage from '@/pages/guest/PasswordAccessPage'
import MemberLoginPage from '@/pages/guest/MemberLoginPage'
import SsoCallbackPage from '@/pages/auth/SsoCallbackPage'

function RequireAuth() {
  const session = useAuthStore((s) => s.session)
  if (!session) {
    return <Navigate to="/login" replace />
  }
  return <Outlet />
}

function RequireGuest() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  if (isAuthenticated()) return <Navigate to="/gates" replace />
  return <Outlet />
}

export const router = createBrowserRouter([
  // Custom domain root — resolve by hostname; redirect to /gates if not a custom domain
  { path: '/', element: <GatePortalPage /> },
  // Public gate portal (no auth required)
  { path: '/gates/:gateId/public', element: <GatePortalPage /> },
  { path: '/gates/:gateId/public/pin', element: <PinPadPage /> },
  { path: '/gates/:gateId/public/password', element: <PasswordAccessPage /> },
  // Member login — gate_id optional for redirect context
  { path: '/member-login', element: <MemberLoginPage /> },
  // SSO callback — handles redirect from SSO provider after authentication
  { path: '/auth/sso/callback', element: <SsoCallbackPage /> },
  // Auth routes (only for unauthenticated users)
  {
    element: <RequireGuest />,
    children: [
      {
        element: <AuthLayout />,
        children: [
          { path: '/login', element: <LoginPage /> },
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
          { path: '/gates', element: <DashboardPage /> },
          { path: '/members', element: <MembersPage /> },
          { path: '/schedules', element: <SchedulesPage /> },
          { path: '/settings', element: <SettingsPage /> },
          { path: '/gates/:gateId', element: <GatePage /> },
        ],
      },
    ],
  },
])
