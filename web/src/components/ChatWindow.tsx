import { FormEvent, KeyboardEvent, useEffect, useRef, useState } from 'react';
import type { TicketDetail } from '../api/types';
import { useAuth } from '../auth/AuthContext';
import { useChat } from '../hooks/useChat';
import { formatDate } from '../utils';
import { ErrorMessage } from './common';

const STATUS_LABEL = { connecting: 'Connecting…', live: 'Live', reconnecting: 'Reconnecting…' };

/** Floating 1:1 chat between the ticket's reporter and its assignee. */
export function ChatWindow({ ticket, onClose }: { ticket: TicketDetail; onClose: () => void }) {
  const { user } = useAuth();
  const { thread, status, send } = useChat(ticket.id);
  const [draft, setDraft] = useState('');
  const [minimized, setMinimized] = useState(false);
  const bodyRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const messages = thread.data?.messages ?? [];

  // The other participant, from the viewer's point of view.
  const isRequester = user?.id === ticket.requester.id;
  const partner = isRequester ? ticket.assignee?.name ?? 'Support' : ticket.requester.name;

  useEffect(() => {
    if (bodyRef.current) bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
  }, [messages.length, minimized]);

  useEffect(() => {
    if (!minimized) inputRef.current?.focus();
  }, [minimized]);

  const submit = (e?: FormEvent) => {
    e?.preventDefault();
    const body = draft.trim();
    if (!body || send.isPending) return;
    send.mutate(body, { onSuccess: () => setDraft('') });
  };

  // Enter sends, Shift+Enter adds a new line.
  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  };

  return (
    <section className={`chat-window ${minimized ? 'minimized' : ''}`} role="dialog" aria-label={`Chat with ${partner}`}>
      <header className="chat-header">
        <span className="avatar" aria-hidden="true">{partner.charAt(0).toUpperCase()}</span>
        <div className="chat-title">
          <strong>{partner}</strong>
          <span className="chat-sub">
            <span className={`live-dot ${status === 'live' ? 'on' : ''}`} /> {STATUS_LABEL[status]} · {ticket.ticketNumber}
          </span>
        </div>
        <button className="chat-icon-btn" aria-label={minimized ? 'Expand chat' : 'Minimize chat'} onClick={() => setMinimized((m) => !m)}>
          {minimized ? '▴' : '▾'}
        </button>
        <button className="chat-icon-btn" aria-label="Close chat" onClick={onClose}>
          ✕
        </button>
      </header>

      {!minimized && (
        <>
          <div className="chat-body" ref={bodyRef} aria-live="polite">
            {thread.isLoading && <p className="muted chat-empty">Loading messages…</p>}
            {thread.error && <ErrorMessage error={thread.error} onRetry={() => thread.refetch()} />}
            {thread.data && messages.length === 0 && (
              <p className="muted chat-empty">No messages yet. Say hello 👋 — this chat is private between the reporter and the assignee.</p>
            )}
            {messages.map((m) => {
              const mine = m.sender.id === user?.id;
              return (
                <div key={m.id} className={`chat-msg ${mine ? 'mine' : 'theirs'}`}>
                  {!mine && <span className="chat-sender">{m.sender.name}</span>}
                  <div className="chat-bubble">{m.body}</div>
                  <span className="chat-time">{formatDate(m.createdAt)}</span>
                </div>
              );
            })}
          </div>

          {thread.data && !thread.data.canSend ? (
            <p className="chat-readonly">{thread.data.reason}</p>
          ) : (
            <form className="chat-composer" onSubmit={submit}>
              {send.error && <ErrorMessage error={send.error} />}
              <textarea
                ref={inputRef}
                rows={1}
                maxLength={2000}
                placeholder="Type a message…"
                aria-label="Chat message"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={onKeyDown}
              />
              <button className="btn btn-primary btn-small" disabled={!draft.trim() || send.isPending} aria-label="Send message">
                Send
              </button>
            </form>
          )}
        </>
      )}
    </section>
  );
}
