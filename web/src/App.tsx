import { Navigate, Route, Routes } from 'react-router-dom'
import { type ReactNode } from 'react'
import { AppLayout } from './components/AppLayout'
import { useAuthStore } from './store/auth'
import { Login } from './pages/Login'
import { Dashboard } from './pages/Dashboard'
import { Traces } from './pages/Traces'
import { TraceDetailPage } from './pages/TraceDetail'
import { Groups } from './pages/Groups'
import { GroupEdit } from './pages/GroupEdit'
import { Channels } from './pages/Channels'
import { ApiKeys } from './pages/ApiKeys'
import { Settings } from './pages/Settings'

/** 鉴权守卫：未登录跳转登录页。 */
function RequireAuth({ children }: { children: ReactNode }) {
  const token = useAuthStore((s) => s.token)
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route
        element={
          <RequireAuth>
            <AppLayout />
          </RequireAuth>
        }
      >
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/traces" element={<Traces />} />
        <Route path="/traces/:traceId" element={<TraceDetailPage />} />
        <Route path="/groups" element={<Groups />} />
        <Route path="/groups/:id" element={<GroupEdit />} />
        <Route path="/channels" element={<Channels />} />
        <Route path="/api-keys" element={<ApiKeys />} />
        <Route path="/settings" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  )
}
