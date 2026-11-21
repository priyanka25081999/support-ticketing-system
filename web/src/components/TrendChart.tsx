import { useState } from 'react';
import type { TrendPoint } from '../api/types';

const WIDTH = 720;
const HEIGHT = 220;
const PAD = { top: 16, right: 16, bottom: 28, left: 34 };

type Series = 'created' | 'resolved';
const SERIES: { key: Series; label: string }[] = [
  { key: 'created', label: 'Created' },
  { key: 'resolved', label: 'Resolved' },
];

function axisLabel(iso: string, bucket: string): string {
  const d = new Date(iso);
  if (bucket === 'hour') return d.toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
  if (bucket === 'month') return d.toLocaleDateString('en-IN', { month: 'short', year: 'numeric' });
  return d.toLocaleDateString('en-IN', { day: '2-digit', month: 'short' });
}

/** Longer label for the tooltip, e.g. "21 Nov 2026" or "21 Nov, 14:00". */
function tooltipLabel(iso: string, bucket: string): string {
  const d = new Date(iso);
  if (bucket === 'hour') return d.toLocaleString('en-IN', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
  if (bucket === 'month') return d.toLocaleDateString('en-IN', { month: 'long', year: 'numeric' });
  return d.toLocaleDateString('en-IN', { day: '2-digit', month: 'short', year: 'numeric' });
}

/**
 * Smooth path through points using monotone cubic interpolation
 * (Fritsch–Carlson), so the curve never dips below zero between counts.
 */
export function monotonePath(pts: { x: number; y: number }[]): string {
  if (pts.length === 0) return '';
  if (pts.length === 1) return `M${pts[0].x},${pts[0].y}`;
  const n = pts.length;
  const dx = pts.slice(1).map((p, i) => p.x - pts[i].x);
  const slope = pts.slice(1).map((p, i) => (p.y - pts[i].y) / dx[i]);
  const tangent = pts.map((_, i) => {
    if (i === 0) return slope[0];
    if (i === n - 1) return slope[n - 2];
    return slope[i - 1] * slope[i] <= 0 ? 0 : (slope[i - 1] + slope[i]) / 2;
  });
  // Limit tangents to keep the curve monotone between points.
  for (let i = 0; i < n - 1; i++) {
    if (slope[i] === 0) {
      tangent[i] = tangent[i + 1] = 0;
      continue;
    }
    const a = tangent[i] / slope[i];
    const b = tangent[i + 1] / slope[i];
    const h = a * a + b * b;
    if (h > 9) {
      const t = 3 / Math.sqrt(h);
      tangent[i] = t * a * slope[i];
      tangent[i + 1] = t * b * slope[i];
    }
  }
  let d = `M${pts[0].x},${pts[0].y}`;
  for (let i = 0; i < n - 1; i++) {
    const h = dx[i] / 3;
    d += ` C${pts[i].x + h},${pts[i].y + tangent[i] * h} ${pts[i + 1].x - h},${pts[i + 1].y - tangent[i + 1] * h} ${pts[i + 1].x},${pts[i + 1].y}`;
  }
  return d;
}

/** Line/area trend of tickets created vs resolved with hover tooltips. */
export function TrendChart({ points, bucket }: { points: TrendPoint[]; bucket: string }) {
  const [active, setActive] = useState<number | null>(null);
  if (points.length === 0) return <p className="muted">No activity in this range.</p>;

  const max = Math.max(1, ...points.map((p) => Math.max(p.created, p.resolved)));
  const innerW = WIDTH - PAD.left - PAD.right;
  const innerH = HEIGHT - PAD.top - PAD.bottom;
  const x = (i: number) => PAD.left + (points.length === 1 ? innerW / 2 : (i * innerW) / (points.length - 1));
  const y = (v: number) => PAD.top + innerH - (v / max) * innerH;
  const baseline = y(0);
  const ticks = [...new Set([0, Math.round(max / 2), max])];
  const labelEvery = Math.ceil(points.length / 8); // at most ~8 x-axis labels
  const slot = points.length === 1 ? innerW : innerW / (points.length - 1);

  const lines = SERIES.map(({ key }) => {
    const pts = points.map((p, i) => ({ x: x(i), y: y(p[key]) }));
    const line = monotonePath(pts);
    const area = `${line} L${pts[pts.length - 1].x},${baseline} L${pts[0].x},${baseline} Z`;
    return { key, line, area };
  });

  const hovered = active === null ? null : points[active];
  const hoverX = active === null ? 0 : x(active);
  const tooltipLeft = (hoverX / WIDTH) * 100;

  return (
    <figure className="trend" onMouseLeave={() => setActive(null)}>
      <svg viewBox={`0 0 ${WIDTH} ${HEIGHT}`} role="img" aria-label="Tickets created and resolved over time">
        <defs>
          <linearGradient id="trend-created" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#3b82f6" stopOpacity="0.25" />
            <stop offset="100%" stopColor="#3b82f6" stopOpacity="0" />
          </linearGradient>
          <linearGradient id="trend-resolved" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#22c55e" stopOpacity="0.2" />
            <stop offset="100%" stopColor="#22c55e" stopOpacity="0" />
          </linearGradient>
        </defs>

        {ticks.map((t) => (
          <g key={t}>
            <line x1={PAD.left} x2={WIDTH - PAD.right} y1={y(t)} y2={y(t)} className="trend-grid" />
            <text x={PAD.left - 8} y={y(t) + 4} textAnchor="end" className="trend-axis">{t}</text>
          </g>
        ))}
        {points.map((p, i) =>
          i % labelEvery === 0 ? (
            <text key={p.bucket} x={x(i)} y={HEIGHT - 8} textAnchor="middle" className="trend-axis">
              {axisLabel(p.bucket, bucket)}
            </text>
          ) : null,
        )}

        {lines.map((l) => (
          <g key={l.key}>
            <path d={l.area} fill={`url(#trend-${l.key})`} />
            <path d={l.line} className={`trend-line ${l.key}`} />
          </g>
        ))}

        {/* Dots only where something happened, so busy days stand out. */}
        {points.map((p, i) =>
          SERIES.map(({ key }) =>
            p[key] > 0 ? (
              <circle key={`${key}-${p.bucket}`} cx={x(i)} cy={y(p[key])} r={active === i ? 6 : 4} className={`trend-dot ${key}`} />
            ) : null,
          ),
        )}

        {active !== null && <line x1={hoverX} x2={hoverX} y1={PAD.top} y2={baseline} className="trend-guide" />}

        {/* Invisible hover/focus targets, one per bucket. */}
        {points.map((p, i) => (
          <rect
            key={`hit-${p.bucket}`}
            data-testid="trend-hit"
            x={x(i) - slot / 2}
            y={PAD.top}
            width={Math.max(slot, 6)}
            height={innerH}
            fill="transparent"
            tabIndex={0}
            aria-label={`${tooltipLabel(p.bucket, bucket)}: ${p.created} created, ${p.resolved} resolved`}
            onMouseEnter={() => setActive(i)}
            onFocus={() => setActive(i)}
            onBlur={() => setActive(null)}
          />
        ))}
      </svg>

      {hovered && (
        <div
          className={`trend-tooltip ${tooltipLeft > 75 ? 'left' : tooltipLeft < 25 ? 'right' : ''}`}
          style={{ left: `${tooltipLeft}%` }}
          role="tooltip"
        >
          <strong>{tooltipLabel(hovered.bucket, bucket)}</strong>
          <span><i className="swatch created" /> {hovered.created} created</span>
          <span><i className="swatch resolved" /> {hovered.resolved} resolved</span>
        </div>
      )}

      <figcaption className="trend-legend">
        <span><i className="swatch created" /> Created</span>
        <span><i className="swatch resolved" /> Resolved</span>
      </figcaption>
    </figure>
  );
}
