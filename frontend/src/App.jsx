import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { auth } from '@/lib/api'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { LocalOnly } from '@/components/LocalOnly'
import { LoginPage } from '@/pages/Login'
import { StreamerPage } from '@/pages/Streamer'
import { WatchPage } from '@/pages/Watch'
import { AccessDeniedPage } from '@/pages/AccessDenied'

function Home() {
  const user = auth.getUser()
  if (!user) return <Navigate to="/login" replace />
  return <Navigate to={user.role === 'admin' ? '/admin' : '/watch'} replace />
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/denied" element={<AccessDeniedPage />} />
        <Route
          path="/login"
          element={
            <LocalOnly>
              <LoginPage />
            </LocalOnly>
          }
        />
        <Route
          path="/admin"
          element={
            <LocalOnly>
              <ProtectedRoute role="admin">
                <StreamerPage />
              </ProtectedRoute>
            </LocalOnly>
          }
        />
        <Route path="/watch" element={<WatchPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
