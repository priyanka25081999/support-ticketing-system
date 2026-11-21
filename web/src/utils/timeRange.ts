// Time range model shared by the dashboard and ticket list.
// Relative presets are resolved at fetch time so live refresh always uses "now".

export type TimeRange = { preset: string } | { from: string; to: string }; // ISO strings for absolute ranges

export const RANGE_PRESETS = [
  { key: '1h', label: 'Last 1 hour', ms: 3_600_000 },
  { key: '6h', label: 'Last 6 hours', ms: 6 * 3_600_000 },
  { key: '24h', label: 'Last 24 hours', ms: 24 * 3_600_000 },
  { key: '7d', label: 'Last 7 days', ms: 7 * 86_400_000 },
  { key: '30d', label: 'Last 30 days', ms: 30 * 86_400_000 },
  { key: '90d', label: 'Last 90 days', ms: 90 * 86_400_000 },
  { key: '1y', label: 'Last 1 year', ms: 365 * 86_400_000 },
  { key: 'all', label: 'All time', ms: 0 },
];

export const isPreset = (r: TimeRange): r is { preset: string } => 'preset' in r;

/** Concrete ISO bounds for the API; "all" has none. */
export function resolveRange(range: TimeRange, now = Date.now()): { from?: string; to?: string } {
  if (!isPreset(range)) return { from: range.from, to: range.to };
  const preset = RANGE_PRESETS.find((p) => p.key === range.preset);
  if (!preset || preset.ms === 0) return {};
  return { from: new Date(now - preset.ms).toISOString() };
}

/** Stable string for query keys and URLs. */
export const rangeKey = (r: TimeRange) => (isPreset(r) ? r.preset : `${r.from}_${r.to}`);

export function readRange(params: URLSearchParams, fallback: string): TimeRange {
  const from = params.get('from');
  const to = params.get('to');
  if (from && to && !Number.isNaN(Date.parse(from)) && !Number.isNaN(Date.parse(to)) && Date.parse(from) < Date.parse(to)) {
    return { from, to };
  }
  const preset = params.get('range');
  return { preset: RANGE_PRESETS.some((p) => p.key === preset) ? preset! : fallback };
}

/** Writes the range into URL params (removes the keys when it equals the default preset). */
export function writeRange(params: URLSearchParams, range: TimeRange, fallback: string): URLSearchParams {
  const next = new URLSearchParams(params);
  next.delete('range');
  next.delete('from');
  next.delete('to');
  if (isPreset(range)) {
    if (range.preset !== fallback) next.set('range', range.preset);
  } else {
    next.set('from', range.from);
    next.set('to', range.to);
  }
  return next;
}

/** Moves a range back (-1) or forward (+1) by its own length. */
export function shiftRange(range: TimeRange, direction: 1 | -1, now = Date.now()): TimeRange | null {
  const { from, to } = resolveRange(range, now);
  if (!from) return null; // "All time" can't shift
  const start = Date.parse(from);
  const end = to ? Date.parse(to) : now;
  const span = end - start;
  const nextEnd = Math.min(end + direction * span, now);
  return { from: new Date(nextEnd - span).toISOString(), to: new Date(nextEnd).toISOString() };
}

/** Browser timezone, e.g. "Asia/Kolkata" (used for trend buckets). */
export const browserTimeZone = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';

/** ISO -> value for <input type="datetime-local"> in local time. */
export function toLocalInput(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
