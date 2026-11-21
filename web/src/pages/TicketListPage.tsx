import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import type { Ticket, TicketFilters } from '../api/types';
import { useAuth } from '../auth/AuthContext';
import { ErrorMessage, Pagination, RefreshControl, SeverityBadge, StatusBadge } from '../components/common';
import { useEnums, useTickets } from '../hooks/queries';
import { useNotifications } from '../hooks/notifications';
import { Select } from '../components/Select';
import { TimeRangePicker } from '../components/TimeRangePicker';
import { TimeRange, readRange, writeRange } from '../utils/timeRange';
import { formatDate, humanize } from '../utils';

type ViewMode = 'table' | 'cards';
const VIEW_KEY = 'ticketViewMode';

/** Ticket list for all roles. Filters live in the URL so views are shareable and survive reloads. */
export function TicketListPage() {
  const { user } = useAuth();
  const isStaff = user?.role !== 'customer';
  const { data: enums } = useEnums();
  const [params, setParams] = useSearchParams();
  const [view, setView] = useState<ViewMode>(() => (localStorage.getItem(VIEW_KEY) as ViewMode) || 'table');
  const [searchInput, setSearchInput] = useState(params.get('search') ?? '');

  const filters: TicketFilters = {
    status: params.get('status') ?? '',
    type: params.get('type') ?? '',
    severity: params.get('severity') ?? '',
    search: params.get('search') ?? '',
    sort: params.get('sort') ?? 'newest',
    page: Number(params.get('page')) || 1,
    pageSize: Number(params.get('pageSize')) || 20,
  };
  const range = readRange(params, 'all');
  const { data, error, isLoading, isFetching, dataUpdatedAt, refetch } = useTickets(filters, range);
  const { data: notifications } = useNotifications();
  const unreadIds = new Set(notifications?.unreadTicketIds ?? []);

  /** Updates one filter and resets to page 1 (unless changing the page itself). */
  const setFilter = (key: string, value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    if (key !== 'page') next.delete('page');
    setParams(next, { replace: true });
  };

  // Debounce the search box so we don't hit the API on every keystroke.
  useEffect(() => {
    const id = setTimeout(() => {
      if (searchInput !== (params.get('search') ?? '')) setFilter('search', searchInput.trim());
    }, 300);
    return () => clearTimeout(id);
  }, [searchInput]);

  useEffect(() => localStorage.setItem(VIEW_KEY, view), [view]);

  const setRange = (r: TimeRange) => {
    const next = writeRange(params, r, 'all');
    next.delete('page');
    setParams(next, { replace: true });
  };

  const hasFilters = ['status', 'type', 'severity', 'search', 'range', 'from'].some((k) => params.get(k));

  return (
    <>
      <div className="page-header">
        <h1>{isStaff ? 'Ticket Queue' : 'My Tickets'}</h1>
        <RefreshControl isFetching={isFetching} updatedAt={dataUpdatedAt} onRefresh={() => refetch()} />
      </div>

      <div className="card filters">
        <input
          placeholder="Search subject or TKT-…"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          aria-label="Search"
        />
        <FilterSelect label="Status" allLabel="All statuses" value={filters.status!} options={enums?.statuses ?? []} onChange={(v) => setFilter('status', v)} />
        <FilterSelect label="Type" allLabel="All types" value={filters.type!} options={enums?.types ?? []} onChange={(v) => setFilter('type', v)} />
        <FilterSelect label="Severity" allLabel="All severities" value={filters.severity!} options={enums?.severities ?? []} onChange={(v) => setFilter('severity', v)} />
        <Select label="Sort" value={filters.sort!} options={SORT_OPTIONS} onChange={(v) => setFilter('sort', v)} />
        <Select label="Page size" value={String(filters.pageSize)} options={PAGE_SIZE_OPTIONS} onChange={(v) => setFilter('pageSize', v)} />
        <TimeRangePicker value={range} onChange={setRange} />
        <div className="view-toggle">
          <button className={`btn btn-small ${view === 'table' ? 'active' : ''}`} onClick={() => setView('table')}>Table</button>
          <button className={`btn btn-small ${view === 'cards' ? 'active' : ''}`} onClick={() => setView('cards')}>Cards</button>
        </div>
        {hasFilters && (
          <button
            className="btn btn-small btn-link"
            onClick={() => {
              setSearchInput('');
              setParams({}, { replace: true });
            }}
          >
            Clear filters
          </button>
        )}
      </div>

      {error && <ErrorMessage error={error} onRetry={() => refetch()} />}
      {isLoading && <p className="muted">Loading tickets…</p>}

      {data && data.items.length === 0 && (
        <div className="card center muted">
          {hasFilters ? 'No tickets match these filters.' : 'No tickets yet.'}
          {!isStaff && !hasFilters && (
            <p>
              <Link to="/tickets/new" className="btn btn-primary">Create your first ticket</Link>
            </p>
          )}
        </div>
      )}

      {data && data.items.length > 0 && (
        <>
          {view === 'table' ? (
            <TicketTable tickets={data.items} showRequester={isStaff} unreadIds={unreadIds} />
          ) : (
            <TicketCards tickets={data.items} unreadIds={unreadIds} />
          )}
          <Pagination page={data.page} pageSize={data.pageSize} total={data.total} onChange={(p) => setFilter('page', String(p))} />
        </>
      )}
    </>
  );
}

const SORT_OPTIONS = [
  { value: 'newest', label: 'Newest first' },
  { value: 'oldest', label: 'Oldest first' },
  { value: 'updated', label: 'Recently updated' },
  { value: 'severity', label: 'Highest severity' },
];
const PAGE_SIZE_OPTIONS = [10, 20, 50].map((n) => ({ value: String(n), label: `${n} / page` }));

/** Enum filter with an "All …" option. */
function FilterSelect({ label, allLabel, value, options, onChange }: { label: string; allLabel: string; value: string; options: string[]; onChange: (v: string) => void }) {
  return (
    <Select
      label={label}
      value={value}
      options={[{ value: '', label: allLabel }, ...options.map((o) => ({ value: o, label: humanize(o) }))]}
      onChange={onChange}
    />
  );
}

/** Blue dot for tickets with activity the user hasn't seen yet. */
const UnreadDot = () => <span className="unread-dot" title="New activity" aria-label="New activity" />;

function TicketTable({ tickets, showRequester, unreadIds }: { tickets: Ticket[]; showRequester: boolean; unreadIds: Set<string> }) {
  return (
    <div className="card table-wrap">
      <table className="table">
        <thead>
          <tr>
            <th>Ticket</th>
            <th>Subject</th>
            <th>Status</th>
            <th>Severity</th>
            <th>Type</th>
            {showRequester && <th>Requester</th>}
            <th>Assignee</th>
            <th>Updated</th>
          </tr>
        </thead>
        <tbody>
          {tickets.map((t) => (
            <tr key={t.id} className={unreadIds.has(t.id) ? 'row-unread' : ''}>
              <td>
                {unreadIds.has(t.id) && <UnreadDot />}
                <Link to={`/tickets/${t.ticketNumber}`}>{t.ticketNumber}</Link>
              </td>
              <td>{t.subject}</td>
              <td><StatusBadge status={t.status} /></td>
              <td><SeverityBadge severity={t.severity} /></td>
              <td>{humanize(t.type)}</td>
              {showRequester && <td>{t.requester.name}</td>}
              <td>{t.assignee?.name ?? '—'}</td>
              <td className="muted">{formatDate(t.updatedAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function TicketCards({ tickets, unreadIds }: { tickets: Ticket[]; unreadIds: Set<string> }) {
  return (
    <div className="cards">
      {tickets.map((t) => (
        <Link key={t.id} to={`/tickets/${t.ticketNumber}`} className="card ticket-card">
          <div className="row space-between">
            <strong>
              {unreadIds.has(t.id) && <UnreadDot />}
              {t.ticketNumber}
            </strong>
            <StatusBadge status={t.status} />
          </div>
          <p>{t.subject}</p>
          <div className="row space-between muted">
            <SeverityBadge severity={t.severity} />
            <span>{formatDate(t.updatedAt)}</span>
          </div>
        </Link>
      ))}
    </div>
  );
}
