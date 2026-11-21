// Small presentational components shared across pages.
import { Component, ErrorInfo, ReactNode } from 'react';
import type { TicketStatus } from '../api/types';
import { useLiveRefresh, REFRESH_INTERVAL_MS } from '../hooks/useLiveRefresh';
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

/** Live auto-refresh toggle, last-updated time and a manual refresh button. */
export function RefreshControl({ isFetching, updatedAt, onRefresh }: RefreshControlProps) {
  const { live, setLive } = useLiveRefresh();
  return (
    <div className="refresh-control">
      <label title={`Auto-refresh every ${REFRESH_INTERVAL_MS / 1000}s`}>
        <input type="checkbox" checked={live} onChange={(e) => setLive(e.target.checked)} />
        <span className={live ? 'live-dot on' : 'live-dot'} /> Live
      </label>
      {updatedAt > 0 && <span className="muted">Updated {formatTime(updatedAt)}</span>}
      <button className="btn btn-small" onClick={onRefresh} disabled={isFetching}>
        {isFetching ? 'Refreshing…' : '⟳ Refresh'}
      </button>
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
