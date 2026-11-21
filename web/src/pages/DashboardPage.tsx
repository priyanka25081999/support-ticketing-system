import { useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { metaApi } from '../api/endpoints';
import { useAuth } from '../auth/AuthContext';
import { ErrorMessage, RefreshControl, SeverityBadge, StatusBadge } from '../components/common';
import { TimeRangePicker } from '../components/TimeRangePicker';
import { TrendChart } from '../components/TrendChart';
import { useSummary } from '../hooks/queries';
import { formatDate, getErrorMessage, humanize } from '../utils';
import { TimeRange, browserTimeZone, readRange, resolveRange, writeRange } from '../utils/timeRange';

const DEFAULT_RANGE = '30d';

/** "5.2 h" or "2.1 d" */
function formatDuration(hours: number | null): string {
  if (hours == null) return '—';
  return hours < 48 ? `${hours.toFixed(1)} h` : `${(hours / 24).toFixed(1)} d`;
}

/** Horizontal bar list, e.g. tickets by severity. Links to the filtered list. */
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

function Kpi({ label, value, tone, hint }: { label: string; value: string | number; tone: string; hint?: string }) {
  return (
    <div className={`card kpi kpi-${tone}`} title={hint}>
      <span className="muted">{label}</span>
      <span className="kpi-value">{value}</span>
    </div>
  );
}

export function DashboardPage() {
  const { user } = useAuth();
  const [params, setParams] = useSearchParams();
  const range = readRange(params, DEFAULT_RANGE);
  const { data, error, isLoading, isFetching, dataUpdatedAt, refetch } = useSummary(range);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState('');
  const isStaff = user?.role === 'admin' || user?.role === 'agent';

  const setRange = (r: TimeRange) => setParams(writeRange(params, r, DEFAULT_RANGE), { replace: true });

  const exportCsv = async () => {
    setExporting(true);
    setExportError('');
    try {
      await metaApi.exportCsv({ ...resolveRange(range), tz: browserTimeZone() });
    } catch (err) {
      setExportError(getErrorMessage(err));
    } finally {
      setExporting(false);
    }
  };

  return (
    <>
      <div className="page-header">
        <h1>Dashboard</h1>
        <div className="toolbar">
          <TimeRangePicker value={range} onChange={setRange} />
          <RefreshControl isFetching={isFetching} updatedAt={dataUpdatedAt} onRefresh={() => refetch()} />
          {isStaff && (
            <button className="btn btn-small" onClick={exportCsv} disabled={exporting}>
              ⬇ {exporting ? 'Exporting…' : 'Export CSV'}
            </button>
          )}
        </div>
      </div>

      {exportError && <div className="alert alert-error">{exportError}</div>}
      {error && <ErrorMessage error={error} onRetry={() => refetch()} />}
      {isLoading && <p className="muted">Loading dashboard…</p>}

      {data && (
        <>
          <div className="kpis">
            <Kpi label="Created" value={data.total} tone="created" hint="Tickets created in this range" />
            <Kpi label="Resolved" value={data.resolved} tone="resolved" hint="Tickets moved to Solved in this range" />
            <Kpi label="Closed" value={data.closed} tone="closed" hint="Tickets closed in this range" />
            <Kpi label="Reopened" value={data.reopened} tone="reopened" hint="Solved tickets reopened in this range" />
            <Kpi label="Avg. resolution" value={formatDuration(data.avgResolutionHours)} tone="time" hint="Average time from creation to Solved" />
          </div>



          <div className="grid-2">
            <BreakdownCard title="By severity" counts={data.bySeverity} filterKey="severity" />
            <BreakdownCard title="By type" counts={data.byType} filterKey="type" />
          </div>

          <section className="card">
            <div className="row space-between">
              <h3>Created vs resolved</h3>
              <span className="muted">per {data.bucket}</span>
            </div>
            <TrendChart points={data.trend} bucket={data.bucket} />
          </section>

          <section className="card">
            <h3>Recently updated</h3>
            {data.recent.length === 0 ? (
              <p className="muted">No tickets yet.</p>
            ) : (
              <table className="table">
                <tbody>
                  {data.recent.map((t) => (
                    <tr key={t.id}>
                      <td><Link to={`/tickets/${t.ticketNumber}`}>{t.ticketNumber}</Link></td>
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
