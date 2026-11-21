import { useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { notificationsApi } from '../api/endpoints';
import type { AppNotification } from '../api/types';
import { humanize } from '../utils';

export const NOTIFICATION_POLL_MS = 10_000;
const KEY = ['notifications'];

/** Notifications are always polled (independent of the Live toggle) so the bell stays current. */
export function useNotifications() {
  return useQuery({ queryKey: KEY, queryFn: notificationsApi.list, refetchInterval: NOTIFICATION_POLL_MS });
}

/** Marks a ticket's notifications as read while the user is viewing it. */
export function useMarkTicketRead(ticketId?: string) {
  const queryClient = useQueryClient();
  const { data } = useNotifications();
  const hasUnread = Boolean(ticketId && data?.unreadTicketIds.includes(ticketId));

  useEffect(() => {
    if (!hasUnread || !ticketId) return;
    notificationsApi
      .markRead(ticketId)
      .then(() => queryClient.invalidateQueries({ queryKey: KEY }))
      .catch(() => {}); // non-critical: the item simply stays unread
  }, [hasUnread, ticketId, queryClient]);
}

/** Human-readable action, e.g. "changed status to Solved". */
export function describeNotification(n: AppNotification): string {
  const data = n.data ?? {};
  switch (n.type) {
    case 'created':
      return 'created a new ticket';
    case 'assigned':
      return `assigned the ticket to ${data.assigneeName ?? 'someone'}`;
    case 'status_changed':
      return `changed status to ${humanize(String(data.to ?? ''))}${data.auto ? ' (auto)' : ''}`;
    case 'linked':
      return `linked ${data.linkedTicketNumber ?? 'a ticket'}`;
    case 'comment':
      return 'commented';
    case 'internal_note':
      return 'added an internal note';
    case 'attachment':
      return `attached ${data.fileName ?? 'a file'}`;
    case 'chat':
      return 'sent you a chat message';
    default:
      return 'updated the ticket';
  }
}
