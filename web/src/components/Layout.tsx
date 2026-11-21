import { NavLink, Outlet } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import { humanize } from '../utils';

/** App shell: top navigation (role-aware) + page content. */
export function Layout() {
  const { user, logout } = useAuth();
  if (!user) return null;
  const isStaff = user.role !== 'customer';

  return (
    <>
      <header className="topbar">
        <span className="brand">
          <img src="/logo.svg" alt="" width={28} height={28} /> Support Desk
        </span>
        <nav>
          <NavLink to="/dashboard">Dashboard</NavLink>
          <NavLink to="/tickets">{isStaff ? 'Ticket Queue' : 'My Tickets'}</NavLink>
          {user.role === 'customer' && <NavLink to="/tickets/new">New Ticket</NavLink>}
          {user.role === 'admin' && <NavLink to="/agents">Agents</NavLink>}
        </nav>
        <div className="user-box">
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
