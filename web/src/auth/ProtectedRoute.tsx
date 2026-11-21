import { Navigate, Outlet, useLocation } from 'react-router-dom';
import type { Role } from '../api/types';
import { useAuth } from './AuthContext';

/** Renders child routes only for logged-in users (optionally with specific roles). */
export function ProtectedRoute({ roles }: { roles?: Role[] }) {
  const { user, status } = useAuth();
  const location = useLocation();

  if (status === 'loading') return <div className="center muted">Loading…</div>;
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />;
  if (roles && !roles.includes(user.role)) {
    return <div className="alert alert-error">You do not have permission to view this page.</div>;
  }
  return <Outlet />;
}
