// Small presentational components shared across pages.
import { Component, ErrorInfo, ReactNode } from 'react';
import type { TicketStatus } from '../api/types';
import { REFRESH_OPTIONS, useLiveRefresh } from '../hooks/useLiveRefresh';
import { Select } from './Select';
import { formatTime, getErrorMessage, humanize } from '../utils';

export function StatusBadge({ status }: { status: TicketStatus }) {
  return <span className={`badge status-${status}`}>{humanize(status)}</span>;
}

export function SeverityBadge({ severity }: { severity: string }) {
  return <span className={`badge severity-${severity}`}>{humanize(severity)}</span>;
}

/** Inline error box with an optional retry action. */
export function ErrorMessage({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  return (
    <div className="alert alert-error" role="alert">
      <span>{getErrorMessage(error)}</span>
      {onRetry && (
        <button className="btn btn-small" onClick={onRetry}>
          Retry
        </button>
      )}
    </div>
  );
}

interface RefreshControlProps {
  isFetching: boolean;
  updatedAt: number;
  onRefresh: () => void;
}

const RefreshIcon = ({ spinning }: { spinning: boolean }) => (
  <svg className={spinning ? 'spin' : ''} width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
    <polyline points="23 4 23 10 17 10" />
    <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
  </svg>
);

/** Live status, last update time, manual refresh and auto-refresh interval (Off, 5s … 1h). */
export function RefreshControl({ isFetching, updatedAt, onRefresh }: RefreshControlProps) {
  const { intervalMs, setIntervalMs, live } = useLiveRefresh();
  const current = REFRESH_OPTIONS.find((o) => o.ms === intervalMs);

  return (
    <div className="refresh-control">
      <span className={`live-pill ${live ? 'on' : ''}`} title={live ? `Auto-refresh every ${current?.label}` : 'Auto-refresh is off'}>
        <span className={live ? 'live-dot on' : 'live-dot'} /> {live ? 'Live' : 'Paused'}
      </span>
      <span className="muted updated-at">{updatedAt > 0 ? `Updated ${formatTime(updatedAt)}` : ''}</span>
      <div className="btn-group">
        <button className="btn btn-small" onClick={onRefresh} disabled={isFetching} aria-label="Refresh now" title="Refresh now">
          <RefreshIcon spinning={isFetching} /> Refresh
        </button>
        <Select
          className="select-compact"
          label="Auto refresh"
          alignRight
          value={String(intervalMs)}
          options={REFRESH_OPTIONS.map((o) => ({ value: String(o.ms), label: o.label }))}
          onChange={(v) => setIntervalMs(Number(v))}
        />
      </div>
    </div>
  );
}

interface PaginationProps {
  page: number;
  pageSize: number;
  total: number;
  onChange: (page: number) => void;
}

export function Pagination({ page, pageSize, total, onChange }: PaginationProps) {
  const pages = Math.max(1, Math.ceil(total / pageSize));
  return (
    <div className="pagination">
      <button className="btn btn-small" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        ‹ Prev
      </button>
      <span>
        Page {page} of {pages} · {total} tickets
      </span>
      <button className="btn btn-small" disabled={page >= pages} onClick={() => onChange(page + 1)}>
        Next ›
      </button>
    </div>
  );
}

/** Catches render errors anywhere below it and shows a friendly fallback. */
export class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('UI error:', error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="center">
          <h2>Something went wrong</h2>
          <p className="muted">{this.state.error.message}</p>
          <button className="btn btn-primary" onClick={() => window.location.reload()}>
            Reload page
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
