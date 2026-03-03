import { Outlet } from 'react-router'

export default function AuthLayout() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-sm">
        <div className="flex items-center justify-center gap-2 mb-8">
          <div className="w-8 h-8 bg-primary rounded-lg flex items-center justify-center">
            <span className="text-primary-foreground font-bold text-sm">G</span>
          </div>
          <span className="text-xl font-bold tracking-tight">GATY</span>
        </div>
        <Outlet />
      </div>
    </div>
  )
}
