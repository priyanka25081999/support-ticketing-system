import { Link } from 'react-router-dom';
import { useSummary } from '../hooks/queries';
import { ErrorMessage, RefreshControl, SeverityBadge, StatusBadge } from '../components/common';
import { formatDate, humanize } from '../utils';
import { useAuth } from '../auth/AuthContext';

/** Horizontal bar list, e.g. tickets by severity. */
function BreakdownCard({ title, counts, filterKey }: { title: string; counts: Record<string, number>; filterKey: string }) {
  const max = Math.max(1, ...Object.values(counts));
  return (
    <section className="card">
      <h3>{title}</h3>
      {Object.entries(counts).map(([key, count]) => (
        <Link key={key} to={`/tickets?${filterKey}=${key}`} className="bar-row">
          <span className="bar-label">{humanize(key)}</span>
          <span className="bar">
            <span className={`bar-fill ${filterKey}-${key}`} style={{ width: `${(count / max) * 100}%` }} />
          </span>
          <span className="bar-count">{count}</span>
        </Link>
      ))}
    </section>
  );
}

export function DashboardPage() {
  const { user } = useAuth();
  const { data, error, isLoading, isFetching, dataUpdatedAt, refetch } = useSummary();
  const scope = user?.role === 'admin' ? 'All tickets' : user?.role === 'agent' ? 'Tickets assigned to you' : 'Your tickets';

  return (
    <>
      <div className="page-header">
        <div>
          <h1>Dashboard</h1>
          <p className="muted">{scope}</p>
        </div>
        <RefreshControl isFetching={isFetching} updatedAt={dataUpdatedAt} onRefresh={() => refetch()} />
      </div>

      {error && <ErrorMessage error={error} onRetry={() => refetch()} />}
      {isLoading && <p className="muted">Loading dashboard…</p>}

      {data && (
        <>
          <div className="stats">
            <Link to="/tickets" className="stat card">
              <span className="stat-value">{data.total}</span>
              <span className="muted">Total</span>
            </Link>
            {Object.entries(data.byStatus).map(([status, count]) => (
              <Link key={status} to={`/tickets?status=${status}`} className={`stat card stat-${status}`}>
                <span className="stat-value">{count}</span>
                <span className="muted">{humanize(status)}</span>
              </Link>
            ))}
          </div>

          <div className="grid-2">
            <BreakdownCard title="By severity" counts={data.bySeverity} filterKey="severity" />
            <BreakdownCard title="By type" counts={data.byType} filterKey="type" />
          </div>

          <section className="card">
            <h3>Recently updated</h3>
            {data.recent.length === 0 ? (
              <p className="muted">No tickets yet.</p>
            ) : (
              <table className="table">
                <tbody>
                  {data.recent.map((t) => (
                    <tr key={t.id}>
                      <td>
                        <Link to={`/tickets/${t.ticketNumber}`}>{t.ticketNumber}</Link>
                      </td>
                      <td>{t.subject}</td>
                      <td><StatusBadge status={t.status} /></td>
                      <td><SeverityBadge severity={t.severity} /></td>
                      <td className="muted">{formatDate(t.updatedAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </section>
        </>
      )}
    </>
  );
}
