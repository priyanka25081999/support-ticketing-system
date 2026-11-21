import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { HomePage, greeting } from '../pages/HomePage';

const mockUser = jest.fn();
jest.mock('../auth/AuthContext', () => ({ useAuth: () => ({ user: mockUser() }) }));
jest.mock('../hooks/notifications', () => ({ useNotifications: () => ({ data: { unreadCount: 2 } }) }));
jest.mock('../hooks/queries', () => ({ useTickets: () => ({ data: { items: [] }, isLoading: false }) }));

const renderHome = () => render(<MemoryRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}><HomePage /></MemoryRouter>);

test('greeting follows the time of day', () => {
  expect(greeting(9)).toBe('Good morning');
  expect(greeting(14)).toBe('Good afternoon');
  expect(greeting(20)).toBe('Good evening');
});

test('customer sees a welcome, create-ticket actions and unread updates', () => {
  mockUser.mockReturnValue({ fullName: 'Cathy Customer', role: 'customer' });
  renderHome();
  expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(/Cathy!/);
  expect(screen.getByText('Welcome to Support Desk ✨')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: '+ Create a ticket' })).toHaveAttribute('href', '/tickets/new');
  expect(screen.getByText(/2 new updates/)).toBeInTheDocument();
  expect(screen.getByText(/All caught up/)).toBeInTheDocument();
  expect(screen.queryByText('Manage agents')).not.toBeInTheDocument();
});

test('admin gets queue and agent management shortcuts', () => {
  mockUser.mockReturnValue({ fullName: 'Admin', role: 'admin' });
  renderHome();
  expect(screen.getByRole('link', { name: 'Open ticket queue' })).toHaveAttribute('href', '/tickets');
  expect(screen.getByText('Manage agents')).toBeInTheDocument();
});
