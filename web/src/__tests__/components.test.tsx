import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ErrorMessage, Pagination, RefreshControl, StatusBadge } from '../components/common';
import { LiveRefreshProvider } from '../hooks/useLiveRefresh';
import { ProtectedRoute } from '../auth/ProtectedRoute';

// Mock the auth hook so ProtectedRoute can be tested without a backend.
const mockUseAuth = jest.fn();
jest.mock('../auth/AuthContext', () => ({ useAuth: () => mockUseAuth() }));

describe('RefreshControl', () => {
  beforeEach(() => localStorage.clear());

  it('auto-refreshes every 10s by default and can be switched off (persisted)', async () => {
    render(
      <LiveRefreshProvider>
        <RefreshControl isFetching={false} updatedAt={Date.now()} onRefresh={() => {}} />
      </LiveRefreshProvider>,
    );
    expect(screen.getByText('Live')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Auto refresh: 10s' }));
    await userEvent.click(screen.getByRole('option', { name: 'Off' }));
    expect(screen.getByText('Paused')).toBeInTheDocument();
    expect(localStorage.getItem('refreshIntervalMs')).toBe('0');
  });

  it('calls onRefresh and disables the button while fetching', async () => {
    const onRefresh = jest.fn();
    const { rerender } = render(<RefreshControl isFetching={false} updatedAt={0} onRefresh={onRefresh} />);
    await userEvent.click(screen.getByRole('button', { name: 'Refresh now' }));
    expect(onRefresh).toHaveBeenCalledTimes(1);

    rerender(<RefreshControl isFetching updatedAt={0} onRefresh={onRefresh} />);
    expect(screen.getByRole('button', { name: 'Refresh now' })).toBeDisabled();
  });
});

describe('common components', () => {
  it('renders a status badge with readable text', () => {
    render(<StatusBadge status="in_progress" />);
    expect(screen.getByText('In progress')).toHaveClass('status-in_progress');
  });

  it('shows API errors with a retry button', async () => {
    const onRetry = jest.fn();
    render(<ErrorMessage error={{ response: { data: { error: { message: 'Ticket not found' } } } }} onRetry={onRetry} />);
    expect(screen.getByRole('alert')).toHaveTextContent('Ticket not found');
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalled();
  });

  it('disables pagination buttons at the edges', () => {
    render(<Pagination page={1} pageSize={20} total={25} onChange={() => {}} />);
    expect(screen.getByRole('button', { name: /prev/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /next/i })).toBeEnabled();
    expect(screen.getByText(/Page 1 of 2/)).toBeInTheDocument();
  });
});

describe('ProtectedRoute', () => {
  const renderAt = (roles?: ('admin' | 'customer')[]) =>
    render(
      <MemoryRouter initialEntries={['/secret']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route path="/login" element={<p>Login page</p>} />
          <Route element={<ProtectedRoute roles={roles} />}>
            <Route path="/secret" element={<p>Secret content</p>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

  it('redirects anonymous users to login', () => {
    mockUseAuth.mockReturnValue({ user: null, status: 'anonymous' });
    renderAt();
    expect(screen.getByText('Login page')).toBeInTheDocument();
  });

  it('blocks users without the required role', () => {
    mockUseAuth.mockReturnValue({ user: { role: 'customer' }, status: 'authenticated' });
    renderAt(['admin']);
    expect(screen.getByText(/do not have permission/)).toBeInTheDocument();
  });

  it('renders content for allowed users', () => {
    mockUseAuth.mockReturnValue({ user: { role: 'admin' }, status: 'authenticated' });
    renderAt(['admin']);
    expect(screen.getByText('Secret content')).toBeInTheDocument();
  });
});
