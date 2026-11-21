import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { notificationsApi } from '../api/endpoints';
import type { AppNotification } from '../api/types';
import { describeNotification, useNotifications } from '../hooks/notifications';
import { formatDate, getErrorMessage, timeAgo } from '../utils';

/** Bell with unread count; dropdown lists recent activity on the user's tickets. */
export function NotificationBell() {
  const { data, error } = useNotifications();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const unread = data?.unreadCount ?? 0;

  const markAll = useMutation({
    mutationFn: () => notificationsApi.markRead(),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['notifications'] }),
  });

  // Close on outside click or Escape.
  useEffect(() => {
    if (!open) return;
    const onClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false);
    document.addEventListener('mousedown', onClick);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onClick);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  // Show the unread count in the browser tab too.
  useEffect(() => {
    document.title = unread > 0 ? `(${unread}) Support Desk` : 'Support Desk';
  }, [unread]);

  // Opening the ticket marks its notifications as read (see useMarkTicketRead).
  const openItem = (n: AppNotification) => {
    setOpen(false);
    navigate(`/tickets/${n.ticketNumber}`, { state: n.type === 'chat' ? { openChat: true } : undefined });
  };

  return (
    <div className="notif" ref={ref}>
      <button
        className="notif-bell"
        aria-label={unread > 0 ? `Notifications, ${unread} unread` : 'Notifications'}
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
          <path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </svg>
        {unread > 0 && <span className="notif-count">{unread > 99 ? '99+' : unread}</span>}
      </button>

      {open && (
        <div className="notif-panel" role="dialog" aria-label="Notifications">
          <div className="notif-header">
            <strong>Notifications</strong>
            <button className="btn-link" disabled={unread === 0 || markAll.isPending} onClick={() => markAll.mutate()}>
              Mark all as read
            </button>
          </div>
          {error && <div className="alert alert-error">{getErrorMessage(error)}</div>}
          {data && data.items.length === 0 && <p className="muted notif-empty">You're all caught up.</p>}
          {unread > 0 && <div className="notif-section unread">New · {unread}</div>}
          <ul className="notif-list">
            {data?.items.map((n, i) => (
              <li key={`${n.kind}-${n.id}`}>
                {!n.unread && (i === 0 || data.items[i - 1].unread) && <div className="notif-section">Earlier</div>}
                <button className={`notif-item ${n.unread ? 'unread' : ''}`} onClick={() => openItem(n)}>
                  {n.unread && <span className="notif-dot" aria-label="Unread" />}
                  <span className="notif-text">
                    <strong>{n.actorName}</strong> {describeNotification(n)}
                  </span>
                  <span className="notif-ticket">
                    {n.ticketNumber} · {n.subject}
                  </span>
                  {n.preview && <span className="notif-preview">“{n.preview}”</span>}
                  <span className="notif-time" title={formatDate(n.createdAt)}>
                    {timeAgo(n.createdAt)}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
