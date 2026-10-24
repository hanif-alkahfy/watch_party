import { Navigate, useLocation } from 'react-router-dom'
import { auth } from '@/lib/api'

export function ProtectedRoute({ role, children }) {
  const user = auth.getUser()
  const location = useLocation()

  if (!user) return <Navigate to="/login" replace state={{ from: location }} />
  if (role && user.role !== role) return <Navigate to="/watch" replace />
  return children
}
