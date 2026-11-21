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

  it('is live by default and can be toggled off (persisted)', async () => {
    render(
      <LiveRefreshProvider>
        <RefreshControl isFetching={false} updatedAt={Date.now()} onRefresh={() => {}} />
      </LiveRefreshProvider>,
    );
    const toggle = screen.getByRole('checkbox');
    expect(toggle).toBeChecked();
    await userEvent.click(toggle);
    expect(toggle).not.toBeChecked();
    expect(localStorage.getItem('liveRefresh')).toBe('off');
  });

  it('calls onRefresh and disables the button while fetching', async () => {
    const onRefresh = jest.fn();
    const { rerender } = render(<RefreshControl isFetching={false} updatedAt={0} onRefresh={onRefresh} />);
    await userEvent.click(screen.getByRole('button', { name: /refresh/i }));
    expect(onRefresh).toHaveBeenCalledTimes(1);

    rerender(<RefreshControl isFetching updatedAt={0} onRefresh={onRefresh} />);
    expect(screen.getByRole('button', { name: /refreshing/i })).toBeDisabled();
  });
});

describe('common components', () => {
  it('renders a status badge with readable text', () => {
    render(<StatusBadge status="pending" />);
    expect(screen.getByText('Pending')).toHaveClass('status-pending');
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
