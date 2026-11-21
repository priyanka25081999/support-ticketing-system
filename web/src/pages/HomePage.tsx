import { Link } from 'react-router-dom';
import type { Role } from '../api/types';
import { useAuth } from '../auth/AuthContext';
import { StatusBadge } from '../components/common';
import { useNotifications } from '../hooks/notifications';
import { useTickets } from '../hooks/queries';
import { timeAgo } from '../utils';

/** "Good morning" / "Good afternoon" / "Good evening" for the given hour. */
export function greeting(hour: number): string {
  if (hour < 12) return 'Good morning';
  if (hour < 17) return 'Good afternoon';
  return 'Good evening';
}

const TAGLINE: Record<Role, string> = {
  customer: "Need a hand? Raise a ticket and our team will jump right on it. We've got your back!",
  agent: "Ready to make someone's day? Your queue is waiting for its hero.",
  admin: 'Your command centre for the whole support team. Let\'s make today a great one!',
};

interface QuickLink {
  to: string;
  icon: string;
  title: string;
  text: string;
}

function quickLinks(role: Role): QuickLink[] {
  const dashboard = { to: '/dashboard', icon: '📊', title: 'Dashboard', text: 'Created vs resolved, trends and stats for any date range.' };
  if (role === 'customer') {
    return [
      { to: '/tickets/new', icon: '✍️', title: 'Create a ticket', text: 'Tell us what went wrong — attach files and link earlier tickets.' },
      { to: '/tickets', icon: '🎫', title: 'My tickets', text: 'Track progress, reply to the team and reopen solved tickets.' },
      dashboard,
    ];
  }
  const links: QuickLink[] = [
    { to: '/tickets', icon: '📥', title: 'Ticket queue', text: 'Filter, search and work on tickets assigned to you.' },
    { ...dashboard, text: 'Team stats, trends and CSV export for any date range.' },
  ];
  if (role === 'admin') {
    links.push({ to: '/agents', icon: '👥', title: 'Manage agents', text: 'Add agents, see their workload and remove access.' });
  }
  return links;
}

const FEATURES = [
  { icon: '💬', title: 'Live chat', text: 'Private 1:1 chat between the reporter and the assigned agent.' },
  { icon: '🔔', title: 'Notifications', text: 'The bell lights up whenever something changes on your tickets.' },
  { icon: '📎', title: 'Attachments', text: 'Drag & drop screenshots and files (up to 5 MB each).' },
  { icon: '🔍', title: 'Smart filters', text: 'Multi-select filters, search and date ranges on every list.' },
];

/** Landing page shown right after login. */
export function HomePage() {
  const { user } = useAuth();
  const { data: notifications } = useNotifications();
  const { data: active, isLoading } = useTickets({ status: 'open,in_progress,on_hold', sort: 'updated', pageSize: 5 });
  if (!user) return null;

  const firstName = user.fullName.split(' ')[0];
  const unread = notifications?.unreadCount ?? 0;
  const isCustomer = user.role === 'customer';

  return (
    <div className="home">
      <section className="hero">
        <div className="hero-text">
          <span className="hero-wave" aria-hidden="true">👋</span>
          <h1>
            {greeting(new Date().getHours())}, {firstName}!
          </h1>
          <p className="hero-title">Welcome to Support Desk ✨</p>
          <p className="hero-sub">{TAGLINE[user.role]}</p>
          <div className="hero-actions">
            {isCustomer ? (
              <Link to="/tickets/new" className="btn btn-hero">+ Create a ticket</Link>
            ) : (
              <Link to="/tickets" className="btn btn-hero">Open ticket queue</Link>
            )}
            <Link to="/dashboard" className="btn btn-hero-outline">View dashboard</Link>
          </div>
          {unread > 0 && (
            <p className="hero-chip">🔔 You have {unread} new update{unread > 1 ? 's' : ''} — check the bell at the top right.</p>
          )}
        </div>
        <div className="hero-art" aria-hidden="true">🎧</div>
      </section>

      <div className="quick-grid">
        {quickLinks(user.role).map((q) => (
          <Link key={q.to} to={q.to} className="card quick-card">
            <span className="quick-icon" aria-hidden="true">{q.icon}</span>
            <strong>{q.title}</strong>
            <span className="muted">{q.text}</span>
            <span className="quick-go" aria-hidden="true">→</span>
          </Link>
        ))}
      </div>

      <div className="grid-2">
        <section className="card">
          <div className="row space-between">
            <h3>{isCustomer ? 'Your active tickets' : 'Active in your queue'}</h3>
            <Link to="/tickets?status=open,in_progress,on_hold">View all</Link>
          </div>
          {isLoading && <p className="muted">Loading…</p>}
          {active && active.items.length === 0 && (
            <p className="empty-happy">🎉 All caught up — nothing waiting right now.</p>
          )}
          <ul className="active-list">
            {active?.items.map((t) => (
              <li key={t.id}>
                <Link to={`/tickets/${t.ticketNumber}`}>
                  <strong>{t.ticketNumber}</strong> {t.subject}
                </Link>
                <span className="row">
                  <StatusBadge status={t.status} />
                  <span className="muted">{timeAgo(t.updatedAt)}</span>
                </span>
              </li>
            ))}
          </ul>
        </section>

        <section className="card">
          <h3>Things you can do</h3>
          <ul className="feature-list">
            {FEATURES.map((f) => (
              <li key={f.title}>
                <span className="feature-icon" aria-hidden="true">{f.icon}</span>
                <div>
                  <strong>{f.title}</strong>
                  <p className="muted">{f.text}</p>
                </div>
              </li>
            ))}
          </ul>
        </section>
      </div>
    </div>
  );
}
