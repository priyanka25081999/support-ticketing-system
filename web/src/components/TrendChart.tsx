import type { TrendPoint } from '../api/types';

const WIDTH = 720;
const HEIGHT = 200;
const PAD = { top: 10, right: 10, bottom: 28, left: 32 };

function bucketLabel(iso: string, bucket: string): string {
  const d = new Date(iso);
  if (bucket === 'hour') return d.toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
  if (bucket === 'month') return d.toLocaleDateString('en-IN', { month: 'short', year: 'numeric' });
  return d.toLocaleDateString('en-IN', { day: '2-digit', month: 'short' });
}

/** Dependency-free SVG bar chart: created vs resolved per bucket. */
export function TrendChart({ points, bucket }: { points: TrendPoint[]; bucket: string }) {
  if (points.length === 0) return <p className="muted">No activity in this range.</p>;

  const max = Math.max(1, ...points.map((p) => Math.max(p.created, p.resolved)));
  const innerW = WIDTH - PAD.left - PAD.right;
  const innerH = HEIGHT - PAD.top - PAD.bottom;
  const slot = innerW / points.length;
  const barW = Math.max(2, Math.min(18, slot / 2 - 2));
  const y = (v: number) => PAD.top + innerH - (v / max) * innerH;
  const labelEvery = Math.ceil(points.length / 10); // at most ~10 x-axis labels
  const ticks = [0, Math.ceil(max / 2), max];

  return (
    <figure className="trend">
      <svg viewBox={`0 0 ${WIDTH} ${HEIGHT}`} role="img" aria-label="Tickets created and resolved over time">
        {ticks.map((t) => (
          <g key={t}>
            <line x1={PAD.left} x2={WIDTH - PAD.right} y1={y(t)} y2={y(t)} className="trend-grid" />
            <text x={PAD.left - 6} y={y(t) + 4} textAnchor="end" className="trend-axis">{t}</text>
          </g>
        ))}
        {points.map((p, i) => {
          const x = PAD.left + i * slot + slot / 2;
          const label = bucketLabel(p.bucket, bucket);
          return (
            <g key={p.bucket}>
              <rect x={x - barW - 1} y={y(p.created)} width={barW} height={y(0) - y(p.created)} className="bar-created" rx="2">
                <title>{`${label}: ${p.created} created`}</title>
              </rect>
              <rect x={x + 1} y={y(p.resolved)} width={barW} height={y(0) - y(p.resolved)} className="bar-resolved" rx="2">
                <title>{`${label}: ${p.resolved} resolved`}</title>
              </rect>
              {i % labelEvery === 0 && (
                <text x={x} y={HEIGHT - 8} textAnchor="middle" className="trend-axis">{label}</text>
              )}
            </g>
          );
        })}
      </svg>
      <figcaption className="trend-legend">
        <span><i className="swatch created" /> Created</span>
        <span><i className="swatch resolved" /> Resolved</span>
      </figcaption>
    </figure>
  );
}
