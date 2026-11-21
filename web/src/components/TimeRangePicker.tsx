import { CSSProperties, FormEvent, useEffect, useLayoutEffect, useRef, useState } from 'react';
/** Compact label so the trigger keeps a stable width, e.g. "21 Nov 08:32 → 21 Nov 14:32". */
const shortDate = (iso: string) =>
  new Date(iso).toLocaleString('en-IN', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
import { RANGE_PRESETS, TimeRange, isPreset, rangeKey, shiftRange, toLocalInput } from '../utils/timeRange';

function labelFor(range: TimeRange): string {
  if (isPreset(range)) return RANGE_PRESETS.find((p) => p.key === range.preset)?.label ?? 'All time';
  return `${shortDate(range.from)} → ${shortDate(range.to)}`;
}

const PANEL_WIDTH = 470;

const ClockIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
    <circle cx="12" cy="12" r="9" />
    <polyline points="12 7 12 12 15 14" />
  </svg>
);

/** Grafana-style range picker: quick ranges, absolute from/to, and step back/forward. */
export function TimeRangePicker({ value, onChange }: { value: TimeRange; onChange: (r: TimeRange) => void }) {
  const [open, setOpen] = useState(false);
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [error, setError] = useState('');
  const ref = useRef<HTMLDivElement>(null);
  const [panelStyle, setPanelStyle] = useState<CSSProperties>({});

  // Keep the panel inside the viewport: align it with the picker's left edge,
  // and shift it left only as much as needed when it would overflow on the right.
  useLayoutEffect(() => {
    if (!open || !ref.current) return;
    const margin = 8;
    const rect = ref.current.getBoundingClientRect();
    const width = Math.min(PANEL_WIDTH, window.innerWidth - margin * 2);
    let left = 0;
    const overflowRight = rect.left + width - (window.innerWidth - margin);
    if (overflowRight > 0) left = -overflowRight;
    if (rect.left + left < margin) left = margin - rect.left;
    setPanelStyle({ left, width });
  }, [open]);

  // Pre-fill the absolute inputs when the panel opens. Depends on the range key (a string),
  // not the object, so live re-renders never reset what the user is typing.
  const key = rangeKey(value);
  useEffect(() => {
    if (!open) return;
    setError('');
    if (isPreset(value)) {
      const now = Date.now();
      const preset = RANGE_PRESETS.find((p) => p.key === value.preset);
      setFrom(toLocalInput(new Date(now - (preset?.ms || 86_400_000)).toISOString()));
      setTo(toLocalInput(new Date(now).toISOString()));
    } else {
      setFrom(toLocalInput(value.from));
      setTo(toLocalInput(value.to));
    }
  }, [open, key]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => ref.current && !ref.current.contains(e.target as Node) && setOpen(false);
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false);
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const apply = (e: FormEvent) => {
    e.preventDefault();
    const start = new Date(from);
    const end = new Date(to);
    if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return setError('Please enter both dates.');
    if (start >= end) return setError('"From" must be before "To".');
    onChange({ from: start.toISOString(), to: end.toISOString() });
    setOpen(false);
  };

  const pick = (preset: string) => {
    onChange({ preset });
    setOpen(false);
  };

  const shift = (direction: 1 | -1) => {
    const next = shiftRange(value, direction);
    if (next) onChange(next);
  };
  const canShift = !(isPreset(value) && value.preset === 'all');

  return (
    <div className="time-picker" ref={ref}>
      <div className="btn-group">
        <button type="button" className="btn btn-small" aria-label="Move time range back" title="Move back" disabled={!canShift} onClick={() => shift(-1)}>
          «
        </button>
        <button type="button" className="btn btn-small time-picker-trigger" aria-haspopup="dialog" aria-expanded={open} aria-label={`Time range: ${labelFor(value)}`} onClick={() => setOpen((o) => !o)}>
          <ClockIcon /> <span>{labelFor(value)}</span>
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" aria-hidden="true"><polyline points="6 9 12 15 18 9" /></svg>
        </button>
        <button type="button" className="btn btn-small" aria-label="Move time range forward" title="Move forward" disabled={!canShift} onClick={() => shift(1)}>
          »
        </button>
      </div>

      {open && (
        <div className="time-picker-panel" role="dialog" aria-label="Choose time range" style={panelStyle}>
          <form className="time-picker-abs" onSubmit={apply}>
            <strong>Absolute time range</strong>
            <label>
              From
              <input type="datetime-local" value={from} onChange={(e) => setFrom(e.target.value)} required />
            </label>
            <label>
              To
              <input type="datetime-local" value={to} onChange={(e) => setTo(e.target.value)} required />
            </label>
            {error && <span className="field-error">{error}</span>}
            <button className="btn btn-primary btn-small">Apply time range</button>
          </form>
          <div className="time-picker-quick">
            <strong>Quick ranges</strong>
            <ul>
              {RANGE_PRESETS.map((p) => (
                <li key={p.key}>
                  <button type="button" className={rangeKey(value) === p.key ? 'active' : ''} onClick={() => pick(p.key)}>
                    {p.label}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </div>
  );
}
