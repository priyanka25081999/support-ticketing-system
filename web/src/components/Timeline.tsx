import type { TimelineItem } from '../api/types';
import { formatDate, humanize } from '../utils';

/** One-line description of a ticket event. requesterId identifies the automatic assignment on creation. */
export function describeEvent(item: TimelineItem, requesterId?: string): string {
  const e = item.event!;
  const who = e.actor?.name ?? 'System';
  switch (e.type) {
    case 'created':
      return `${who} created the ticket`;
    case 'assigned':
      if (!e.actor || e.actor.id === requesterId) return `Auto-assigned to ${e.data.assigneeName ?? 'someone'}`;
      return `${who} assigned the ticket to ${e.data.assigneeName ?? 'someone'}`;
    case 'status_changed':
      return `${who} changed status ${humanize(String(e.data.from))} → ${humanize(String(e.data.to))}${e.data.auto ? ' (auto)' : ''}`;
    case 'linked':
      return `${who} linked ${e.data.linkedTicketNumber ?? 'a ticket'}`;
    default:
      return humanize(e.type);
  }
}

/** Conversation between the customer and support (oldest first). */
export function CommentList({ items }: { items: TimelineItem[] }) {
  const comments = items.filter((i) => i.kind === 'message' && i.message);
  if (comments.length === 0) return <p className="muted">There are no comments yet on this ticket.</p>;
  return (
    <ul className="timeline">
      {comments.map(({ message: m }) => (
        <li
          key={m!.id}
          className={`message ${m!.authorRole === 'customer' ? 'from-customer' : 'from-staff'} ${m!.isInternal ? 'internal' : ''}`}
        >
          <div className="meta">
            <span className="avatar" aria-hidden="true">{m!.author.name.charAt(0).toUpperCase()}</span>
            <strong>{m!.author.name}</strong>
            {m!.authorRole !== 'customer' && <span className="badge">{humanize(m!.authorRole)}</span>}
            {m!.isInternal && <span className="badge status-pending">Internal note</span>}
            <span className="muted">{formatDate(m!.createdAt)}</span>
          </div>
          <p className="body">{m!.body}</p>
        </li>
      ))}
    </ul>
  );
}

/** Status/assignment/link history (newest first). */
export function HistoryList({ items, requesterId }: { items: TimelineItem[]; requesterId: string }) {
  const events = items.filter((i) => i.kind === 'event' && i.event).reverse();
  if (events.length === 0) return <p className="muted">No history yet.</p>;
  return (
    <ol className="history">
      {events.map((item) => (
        <li key={item.event!.id}>
          <span>{describeEvent(item, requesterId)}</span>
          <span className="muted">{formatDate(item.createdAt)}</span>
        </li>
      ))}
    </ol>
  );
}
