import { useState } from 'react';
import type { TimelineItem } from '../api/types';
import { formatDate, humanize } from '../utils';

/** One-line description of a ticket event. */
function describeEvent(item: TimelineItem): string {
  const e = item.event!;
  const who = e.actor?.name ?? 'System';
  switch (e.type) {
    case 'created':
      return `${who} created the ticket`;
    case 'assigned':
      return `Assigned to ${e.data.assigneeName ?? 'someone'}`;
    case 'status_changed':
      return `${who} changed status ${humanize(String(e.data.from))} → ${humanize(String(e.data.to))}${e.data.auto ? ' (auto)' : ''}`;
    case 'linked':
      return `${who} linked ${e.data.linkedTicketNumber ?? 'a ticket'}`;
    default:
      return humanize(e.type);
  }
}

/** Messages and status events in time order; events can be hidden. */
export function Timeline({ items }: { items: TimelineItem[] }) {
  const [showEvents, setShowEvents] = useState(true);
  const visible = showEvents ? items : items.filter((i) => i.kind === 'message');

  return (
    <section className="card">
      <div className="row space-between">
        <h3>Timeline</h3>
        <label className="muted">
          <input type="checkbox" checked={showEvents} onChange={(e) => setShowEvents(e.target.checked)} /> Show
          activity
        </label>
      </div>
      {visible.length === 0 && <p className="muted">No activity yet.</p>}
      <ul className="timeline">
        {visible.map((item) =>
          item.kind === 'message' && item.message ? (
            <li
              key={`m-${item.message.id}`}
              className={`message ${item.message.authorRole === 'customer' ? 'from-customer' : 'from-staff'} ${item.message.isInternal ? 'internal' : ''}`}
            >
              <div className="meta">
                <strong>{item.message.author.name}</strong>
                {item.message.isInternal && <span className="badge">Internal note</span>}
                <span className="muted">{formatDate(item.createdAt)}</span>
              </div>
              <p className="body">{item.message.body}</p>
            </li>
          ) : (
            <li key={`e-${item.event?.id}`} className="event muted">
              • {describeEvent(item)} <span>· {formatDate(item.createdAt)}</span>
            </li>
          ),
        )}
      </ul>
    </section>
  );
}
