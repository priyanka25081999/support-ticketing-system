import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { AuthProvider, useAuth } from './auth/AuthContext';
import { ProtectedRoute } from './auth/ProtectedRoute';
import { ErrorBoundary } from './components/common';
import { Layout } from './components/Layout';
import { LiveRefreshProvider } from './hooks/useLiveRefresh';
import { AgentsPage, NotFoundPage } from './pages/AgentsPage';
import { LoginPage, RegisterPage } from './pages/AuthPages';
import { DashboardPage } from './pages/DashboardPage';
import { NewTicketPage } from './pages/NewTicketPage';
import { TicketDetailPage } from './pages/TicketDetailPage';
import { TicketListPage } from './pages/TicketListPage';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5_000,
      refetchOnWindowFocus: true,
      // Don't retry client errors (401/403/404/422); retry network/5xx once.
      retry: (count, error: unknown) => {
        const status = (error as { response?: { status?: number } })?.response?.status;
        return (!status || status >= 500) && count < 1;
      },
    },
  },
});

/** Login/Register redirect away if already signed in. */
function PublicOnly({ children }: { children: JSX.Element }) {
  const { user, status } = useAuth();
  if (status === 'loading') return <div className="center muted">Loading…</div>;
  return user ? <Navigate to="/dashboard" replace /> : children;
}

export default function App() {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <LiveRefreshProvider>
          <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
            <AuthProvider>
              <Routes>
                <Route path="/login" element={<PublicOnly><LoginPage /></PublicOnly>} />
                <Route path="/register" element={<PublicOnly><RegisterPage /></PublicOnly>} />

                <Route element={<ProtectedRoute />}>
                  <Route element={<Layout />}>
                    <Route path="/" element={<Navigate to="/dashboard" replace />} />
                    <Route path="/dashboard" element={<DashboardPage />} />
                    <Route path="/tickets" element={<TicketListPage />} />
                    <Route path="/tickets/:id" element={<TicketDetailPage />} />
                    <Route element={<ProtectedRoute roles={['customer']} />}>
                      <Route path="/tickets/new" element={<NewTicketPage />} />
                    </Route>
                    <Route element={<ProtectedRoute roles={['admin']} />}>
                      <Route path="/agents" element={<AgentsPage />} />
                    </Route>
                    <Route path="*" element={<NotFoundPage />} />
                  </Route>
                </Route>
              </Routes>
            </AuthProvider>
          </BrowserRouter>
        </LiveRefreshProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}
