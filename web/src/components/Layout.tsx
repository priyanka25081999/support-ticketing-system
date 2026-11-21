import { NavLink, Outlet, useLocation } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import { humanize } from '../utils';
import { NotificationBell } from './NotificationBell';

/** App shell: top navigation (role-aware) + page content. */
export function Layout() {
  const { user, logout } = useAuth();
  const { pathname } = useLocation();
  if (!user) return null;
  const isStaff = user.role !== 'customer';
  // "My Tickets" covers the list and ticket details, but not the New Ticket form.
  const ticketsActive = pathname.startsWith('/tickets') && pathname !== '/tickets/new';

  return (
    <>
      <header className="topbar">
        <NavLink to="/home" className="brand">
          <img src="/logo.svg" alt="" width={28} height={28} /> Support Desk
        </NavLink>
        <nav>
          <NavLink to="/home">Home</NavLink>
          <NavLink to="/dashboard">Dashboard</NavLink>
          <NavLink to="/tickets" className={() => (ticketsActive ? 'active' : '')} aria-current={ticketsActive ? 'page' : undefined}>
            {isStaff ? 'Ticket Queue' : 'My Tickets'}
          </NavLink>
          {user.role === 'customer' && <NavLink to="/tickets/new">New Ticket</NavLink>}
          {user.role === 'admin' && <NavLink to="/agents">Agents</NavLink>}
        </nav>
        <div className="user-box">
          <span className="powered-by">⚡ Powered by Zendesk</span>
          <NotificationBell />
          <span>
            {user.fullName} <span className="badge">{humanize(user.role)}</span>
          </span>
          <button className="btn btn-small" onClick={logout}>
            Logout
          </button>
        </div>
      </header>
      <main className="container">
        <Outlet />
      </main>
    </>
  );
}
