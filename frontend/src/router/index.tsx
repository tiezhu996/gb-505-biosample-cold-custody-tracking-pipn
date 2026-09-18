import { Navigate, Outlet, createBrowserRouter, useLocation } from 'react-router-dom'
import { AppShell } from '../components/common/AppShell'
import { useAuth } from '../hooks/useAuth'
import { AuditPage } from '../pages/AuditPage'
import { SpecimenDetailPage } from '../pages/SpecimenDetailPage'
import { SpecimensPage } from '../pages/SpecimensPage'
import { TransfersPage } from '../pages/TransfersPage'
import { StoragePage } from '../pages/StoragePage'
import { LoginPage } from '../pages/LoginPage'
import { ProtocolsPage } from '../pages/ProtocolsPage'

function ProtectedRoute() {
  const { user } = useAuth()
  const location = useLocation()
  return user ? <Outlet /> : <Navigate to="/login" state={{ from: location.pathname }} replace />
}

function PermissionRoute({ permission }: { permission: string }) {
  const { can } = useAuth()
  return can(permission) ? <Outlet /> : <Navigate to="/specimens" replace />
}

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    element: <ProtectedRoute />,
    children: [{
      element: <AppShell />,
      children: [
        { index: true, element: <Navigate to="/specimens" replace /> },
        { path: '/specimens', element: <SpecimensPage /> },
        { path: '/specimens/:id', element: <SpecimenDetailPage /> },
        { path: '/storage', element: <StoragePage /> },
        { path: '/transfers', element: <TransfersPage /> },
        { path: '/protocols', element: <ProtocolsPage /> },
        {
          element: <PermissionRoute permission="audit:read" />,
          children: [{ path: '/audit', element: <AuditPage /> }],
        },
      ],
    }],
  },
  { path: '*', element: <Navigate to="/specimens" replace /> },
])
