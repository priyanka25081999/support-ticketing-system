import type { AppNotification } from '../api/types';
import { describeNotification } from '../hooks/notifications';
import { formatDate, formatTime, timeAgo } from '../utils';

describe('date formatting', () => {
  const iso = '2026-11-21T07:11:02Z';

  it('shows the timezone abbreviation', () => {
    expect(formatDate(iso, 'Asia/Kolkata')).toBe('21 Nov 2026, 12:41 IST');
    expect(formatDate(iso, 'UTC')).toBe('21 Nov 2026, 07:11 UTC');
    expect(formatTime(Date.parse(iso), 'Asia/Kolkata')).toBe('12:41:02 IST');
  });

  it('formats relative time', () => {
    const now = Date.parse(iso);
    expect(timeAgo(iso, now + 10_000)).toBe('just now');
    expect(timeAgo(iso, now + 5 * 60_000)).toBe('5m ago');
    expect(timeAgo(iso, now + 3 * 3_600_000)).toBe('3h ago');
    expect(timeAgo(iso, now + 2 * 86_400_000)).toBe('2d ago');
  });
});

describe('describeNotification', () => {
  const base: AppNotification = {
    id: '1', kind: 'event', type: 'created', ticketId: 't', ticketNumber: 'TKT-000001', subject: 's',
    actorName: 'Ann', data: {}, preview: '', createdAt: '2026-11-21T07:11:02Z', unread: true,
  };

  it('describes each activity type', () => {
    expect(describeNotification(base)).toBe('created a new ticket');
    expect(describeNotification({ ...base, type: 'status_changed', data: { from: 'open', to: 'solved' } })).toBe('changed status to Solved');
    expect(describeNotification({ ...base, type: 'assigned', data: { assigneeName: 'Bella' } })).toBe('assigned the ticket to Bella');
    expect(describeNotification({ ...base, kind: 'message', type: 'comment', data: null })).toBe('commented');
    expect(describeNotification({ ...base, kind: 'attachment', type: 'attachment', data: { fileName: 'a.pdf' } })).toBe('attached a.pdf');
  });
});
