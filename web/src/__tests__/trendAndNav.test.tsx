import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Layout } from '../components/Layout';
import { TrendChart, monotonePath } from '../components/TrendChart';

jest.mock('../auth/AuthContext', () => ({ useAuth: () => ({ user: { fullName: 'Cathy', role: 'customer' }, logout: jest.fn() }) }));
jest.mock('../components/NotificationBell', () => ({ NotificationBell: () => null }));

const nav = (path: string) =>
  render(
    <MemoryRouter initialEntries={[path]} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
      <Layout />
    </MemoryRouter>,
  );

describe('top navigation', () => {
  it('highlights only New Ticket on the new-ticket page', () => {
    nav('/tickets/new');
    expect(screen.getByRole('link', { name: 'New Ticket' })).toHaveClass('active');
    expect(screen.getByRole('link', { name: 'My Tickets' })).not.toHaveClass('active');
  });

  it('highlights My Tickets on the list and on ticket details', () => {
    nav('/tickets/TKT-000001');
    expect(screen.getByRole('link', { name: 'My Tickets' })).toHaveClass('active');
    expect(screen.getByRole('link', { name: 'New Ticket' })).not.toHaveClass('active');
  });
});

describe('TrendChart', () => {
  const points = [
    { bucket: '2026-11-19T00:00:00+05:30', created: 0, resolved: 0 },
    { bucket: '2026-11-20T00:00:00+05:30', created: 2, resolved: 1 },
    { bucket: '2026-11-21T00:00:00+05:30', created: 6, resolved: 3 },
  ];

  it('draws dots only where tickets were created/resolved and shows counts on hover', () => {
    const { container } = render(<TrendChart points={points} bucket="day" />);
    expect(container.querySelectorAll('.trend-line')).toHaveLength(2);
    expect(container.querySelectorAll('.trend-dot')).toHaveLength(4); // first day has no activity
    fireEvent.mouseEnter(screen.getAllByTestId('trend-hit')[2]);
    const tip = screen.getByRole('tooltip');
    expect(tip).toHaveTextContent('21 Nov 2026');
    expect(tip).toHaveTextContent('6 created');
    expect(tip).toHaveTextContent('3 resolved');
  });

  it('builds a smooth path that never overshoots below the baseline', () => {
    const d = monotonePath([{ x: 0, y: 100 }, { x: 10, y: 0 }, { x: 20, y: 100 }]);
    const ys = [...d.matchAll(/[\d.]+,([\d.-]+)/g)].map((m) => Number(m[1]));
    expect(Math.max(...ys)).toBeLessThanOrEqual(100);
  });
});
