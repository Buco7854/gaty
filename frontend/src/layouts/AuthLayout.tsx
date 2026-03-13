import { Outlet } from 'react-router'
import { DoorOpen } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'
import { LangToggle } from '@/components/LangToggle'

export default function AuthLayout() {
  return (
    <div className="flex items-center justify-center min-h-screen p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="flex items-center justify-center h-8 w-8 rounded-md bg-primary/10 text-primary">
              <DoorOpen className="h-4 w-4" />
            </div>
            <span className="font-bold text-lg font-mono">GATIE</span>
          </div>
          <div className="flex items-center gap-1">
            <LangToggle />
            <ThemeToggle />
          </div>
        </div>
        <div className="border rounded-lg p-6">
          <Outlet />
        </div>
      </div>
    </div>
  )
}
