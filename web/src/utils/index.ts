// Shared helpers: error messages, formatting and status rules.
import type { Role, TicketStatus } from '../api/types';

interface ApiErrorShape {
  response?: { status?: number; data?: { error?: { message?: string } } };
  request?: unknown;
  message?: string;
}

/** Turns any thrown value (usually an Axios error) into a user-friendly message. */
export function getErrorMessage(error: unknown): string {
  const err = error as ApiErrorShape | null;
  const apiMessage = err?.response?.data?.error?.message;
  if (apiMessage) return apiMessage;
  if (err?.response?.status && err.response.status >= 500) return 'Server error. Please try again shortly.';
  if (err?.request && !err.response) return 'Cannot reach the server. Check your connection and try again.';
  return err?.message || 'Something went wrong.';
}

export const MAX_FILE_BYTES = 5 * 1024 * 1024; // must match the API limit
export const MAX_FILES = 10;

export const formatBytes = (n: number) =>
  n < 1024 ? `${n} B` : n < 1024 * 1024 ? `${(n / 1024).toFixed(1)} KB` : `${(n / 1024 / 1024).toFixed(1)} MB`;

const EMAIL_RE = /^[^\s@;]+@[^\s@;]+\.[^\s@;]+$/;

/** Validates a ";"-separated email list; returns an error message or "" if valid. */
export function validateEmailList(raw: string): string {
  const emails = raw.split(';').map((e) => e.trim()).filter(Boolean);
  if (emails.length === 0) return 'At least one email is required';
  const bad = emails.find((e) => !EMAIL_RE.test(e));
  if (bad) return `Invalid email address: ${bad}`;
  if (emails.length > 10) return 'At most 10 emails are allowed';
  return '';
}

// en-IN gives short zone names such as "IST" and "UTC"; timeZone is only passed in tests.
const DATE_OPTIONS: Intl.DateTimeFormatOptions = {
  day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZoneName: 'short',
};
const TIME_OPTIONS: Intl.DateTimeFormatOptions = {
  hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23', timeZoneName: 'short',
};

/** e.g. "21 Nov 2026, 12:41 IST" */
export const formatDate = (iso: string, timeZone?: string) =>
  new Date(iso).toLocaleString('en-IN', { ...DATE_OPTIONS, timeZone });

/** e.g. "13:52:08 IST" */
export const formatTime = (ms: number, timeZone?: string) =>
  new Date(ms).toLocaleTimeString('en-IN', { ...TIME_OPTIONS, timeZone });

/** Short relative time, e.g. "5m ago". */
export function timeAgo(iso: string, now = Date.now()): string {
  const seconds = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000));
  if (seconds < 60) return 'just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

/** "feature_request" -> "Feature request" */
export const humanize = (value: string) => {
  const text = value.replace(/_/g, ' ');
  return text.charAt(0).toUpperCase() + text.slice(1);
};

// Mirrors the backend state machine (api/internal/service/status_machine.go).
// The server is the source of truth; this only decides which buttons to show.
const transitions: Record<TicketStatus, TicketStatus[]> = {
  open: ['in_progress', 'on_hold', 'solved'],
  in_progress: ['open', 'on_hold', 'solved'],
  on_hold: ['in_progress', 'open', 'solved'],
  solved: ['open', 'closed'],
  closed: [],
};

export function nextStatuses(role: Role, current: TicketStatus): TicketStatus[] {
  const allowed = transitions[current];
  if (role === 'admin') return allowed;
  if (role === 'agent') return allowed.filter((s) => s !== 'closed');
  return current === 'solved' ? ['open'] : []; // customers can only reopen
}
