import { NavLink, Outlet, useNavigate, useParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useAuthStore } from '@/store/auth'
import type { WorkspaceWithRole } from '@/types'
import {
  LayoutGrid,
  Users,
  Settings,
  LogOut,
  ChevronDown,
  Home,
} from 'lucide-react'
import { useState } from 'react'

export default function AppLayout() {
  const { wsId } = useParams<{ wsId?: string }>()
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)

  const { data: workspaces } = useQuery<WorkspaceWithRole[]>({
    queryKey: ['workspaces'],
    queryFn: () => api.get<{ workspaces: WorkspaceWithRole[] }>('/workspaces').then((r) => {
      // Huma wraps arrays — handle both formats
      const d = r.data as unknown as { workspaces?: WorkspaceWithRole[] } | WorkspaceWithRole[]
      return Array.isArray(d) ? d : (d as { workspaces: WorkspaceWithRole[] }).workspaces ?? []
    }),
  })

  const currentWs = workspaces?.find((w) => w.id === wsId)

  function handleLogout() {
    logout()
    navigate('/login')
  }

  return (
    <div className="flex h-screen bg-background overflow-hidden">
      {/* Sidebar */}
      <aside className="w-60 shrink-0 border-r border-border flex flex-col bg-sidebar">
        {/* Logo */}
        <div className="h-14 flex items-center px-4 border-b border-border">
          <NavLink to="/workspaces" className="flex items-center gap-2">
            <div className="w-7 h-7 bg-primary rounded-md flex items-center justify-center">
              <span className="text-primary-foreground font-bold text-xs">G</span>
            </div>
            <span className="font-bold text-sidebar-foreground">GATY</span>
          </NavLink>
        </div>

        {/* Workspace switcher */}
        {currentWs ? (
          <div className="px-3 py-3 border-b border-border">
            <button
              className="w-full flex items-center justify-between text-left px-2 py-1.5 rounded-md hover:bg-sidebar-accent transition-colors text-sm"
              onClick={() => setMenuOpen(!menuOpen)}
            >
              <div className="flex items-center gap-2 min-w-0">
                <div className="w-5 h-5 rounded bg-primary/15 flex items-center justify-center shrink-0">
                  <span className="text-primary font-bold text-[10px] uppercase">
                    {currentWs.name[0]}
                  </span>
                </div>
                <span className="truncate font-medium text-sidebar-foreground">{currentWs.name}</span>
              </div>
              <ChevronDown className="w-3.5 h-3.5 shrink-0 text-muted-foreground" />
            </button>
            {menuOpen && (
              <div className="mt-1 rounded-md border border-border bg-popover shadow-md p-1 z-10">
                {workspaces?.map((w) => (
                  <button
                    key={w.id}
                    className="w-full text-left text-sm px-2 py-1.5 rounded hover:bg-accent transition-colors truncate"
                    onClick={() => { navigate(`/workspaces/${w.id}`); setMenuOpen(false) }}
                  >
                    {w.name}
                  </button>
                ))}
                <div className="h-px bg-border my-1" />
                <button
                  className="w-full text-left text-sm px-2 py-1.5 rounded hover:bg-accent transition-colors text-muted-foreground"
                  onClick={() => { navigate('/workspaces'); setMenuOpen(false) }}
                >
                  All workspaces
                </button>
              </div>
            )}
          </div>
        ) : (
          <div className="px-3 py-3 border-b border-border">
            <NavLink
              to="/workspaces"
              className="flex items-center gap-2 px-2 py-1.5 rounded-md hover:bg-sidebar-accent text-sm text-sidebar-foreground"
            >
              <Home className="w-4 h-4" />
              Workspaces
            </NavLink>
          </div>
        )}

        {/* Nav links (workspace context) */}
        {wsId && (
          <nav className="flex-1 p-3 space-y-0.5">
            <NavItem to={`/workspaces/${wsId}`} end icon={<LayoutGrid className="w-4 h-4" />} label="Gates" />
            <NavItem to={`/workspaces/${wsId}/members`} icon={<Users className="w-4 h-4" />} label="Members" />
            <NavItem to={`/workspaces/${wsId}/settings`} icon={<Settings className="w-4 h-4" />} label="Settings" />
          </nav>
        )}

        {/* User footer */}
        <div className="p-3 border-t border-border">
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0">
              <p className="text-xs font-medium text-sidebar-foreground truncate">{user?.email}</p>
            </div>
            <button
              onClick={handleLogout}
              className="p-1.5 rounded-md hover:bg-sidebar-accent text-muted-foreground hover:text-foreground transition-colors shrink-0"
              title="Sign out"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  )
}

function NavItem({
  to,
  icon,
  label,
  end,
}: {
  to: string
  icon: React.ReactNode
  label: string
  end?: boolean
}) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        `flex items-center gap-2.5 px-2 py-1.5 rounded-md text-sm transition-colors ${
          isActive
            ? 'bg-sidebar-accent text-sidebar-primary font-medium'
            : 'text-sidebar-foreground hover:bg-sidebar-accent/60'
        }`
      }
    >
      {icon}
      {label}
    </NavLink>
  )
}
