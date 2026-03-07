import { createBrowserRouter, Navigate, Outlet, useLocation } from 'react-router'
import { useAuthStore } from '@/store/auth'
import AppLayout from '@/layouts/AppLayout'
import AuthLayout from '@/layouts/AuthLayout'
import LoginPage from '@/pages/auth/LoginPage'
import RegisterPage from '@/pages/auth/RegisterPage'
import WorkspacesPage from '@/pages/workspaces/WorkspacesPage'
import WorkspacePage from '@/pages/workspace/WorkspacePage'
import GatePage from '@/pages/gate/GatePage'
import MembersPage from '@/pages/workspace/MembersPage'
import SchedulesPage from '@/pages/workspace/SchedulesPage'
import SettingsPage from '@/pages/workspace/SettingsPage'
import GatePortalPage from '@/pages/guest/GatePortalPage'
import PinPadPage from '@/pages/guest/PinPadPage'
import PasswordAccessPage from '@/pages/guest/PasswordAccessPage'
import MemberLoginPage from '@/pages/guest/MemberLoginPage'
import SsoCallbackPage from '@/pages/auth/SsoCallbackPage'

function hasLocalMemberSession(wsId: string): boolean {
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    if (!key?.startsWith('gatie_session_')) continue
    try {
      const s = JSON.parse(localStorage.getItem(key)!)
      if (s?.type === 'member' && s?.workspace_id === wsId && s?.access_token) return true
    } catch { /* ignore */ }
  }
  return false
}

function RequireAuth() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const { pathname } = useLocation()
  if (!isAuthenticated()) {
    // Allow local members to access all workspace sub-paths
    const m = pathname.match(/^\/workspaces\/([^/]+)/)
    if (m && hasLocalMemberSession(m[1])) return <Outlet />
    return <Navigate to="/login" replace />
  }
  return <Outlet />
}

function RequireGuest() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  if (isAuthenticated()) return <Navigate to="/workspaces" replace />
  return <Outlet />
}

export const router = createBrowserRouter([
  // Custom domain root — resolve by hostname; redirect to /workspaces if not a custom domain
  { path: '/', element: <GatePortalPage /> },
  // Public gate portal (no auth required)
  { path: '/workspaces/:wsId/gates/:gateId/public', element: <GatePortalPage /> },
  { path: '/workspaces/:wsId/gates/:gateId/public/pin', element: <PinPadPage /> },
  { path: '/workspaces/:wsId/gates/:gateId/public/password', element: <PasswordAccessPage /> },
  // Member login — workspace-scoped, gate_id optional for redirect context
  { path: '/workspaces/:wsId/login', element: <MemberLoginPage /> },
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
          { path: '/workspaces', element: <WorkspacesPage /> },
          { path: '/workspaces/:wsId', element: <WorkspacePage /> },
          { path: '/workspaces/:wsId/members', element: <MembersPage /> },
          { path: '/workspaces/:wsId/schedules', element: <SchedulesPage /> },
          { path: '/workspaces/:wsId/settings', element: <SettingsPage /> },
          { path: '/workspaces/:wsId/gates/:gateId', element: <GatePage /> },
        ],
      },
    ],
  },
])
