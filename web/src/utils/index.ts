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

export const formatDate = (iso: string) => new Date(iso).toLocaleString();
export const formatTime = (ms: number) => new Date(ms).toLocaleTimeString();

/** "feature_request" -> "Feature request" */
export const humanize = (value: string) => {
  const text = value.replace(/_/g, ' ');
  return text.charAt(0).toUpperCase() + text.slice(1);
};

// Mirrors the backend state machine (api/internal/service/status_machine.go).
// The server is the source of truth; this only decides which buttons to show.
const transitions: Record<TicketStatus, TicketStatus[]> = {
  open: ['pending', 'solved'],
  pending: ['open', 'solved'],
  solved: ['open', 'closed'],
  closed: [],
};

export function nextStatuses(role: Role, current: TicketStatus): TicketStatus[] {
  const allowed = transitions[current];
  if (role === 'admin') return allowed;
  if (role === 'agent') return allowed.filter((s) => s !== 'closed');
  return current === 'solved' ? ['open'] : []; // customers can only reopen
}
