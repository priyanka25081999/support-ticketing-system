import { FormEvent, ReactNode, RefObject, useEffect, useRef, useState } from 'react';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ticketsApi } from '../api/endpoints';
import type { Role, TicketDetail } from '../api/types';
import { useAuth } from '../auth/AuthContext';
import { ErrorMessage, RefreshControl, SeverityBadge, StatusBadge } from '../components/common';
import { FileInput } from '../components/FileInput';
import { LinkTicketPicker } from '../components/LinkTicketPicker';
import { CommentList, HistoryList } from '../components/Timeline';
import { ChatWindow } from '../components/ChatWindow';
import { Select } from '../components/Select';
import { useMarkTicketRead } from '../hooks/notifications';
import { useAgents, useTicket, useTimeline } from '../hooks/queries';
import { formatBytes, formatDate, getErrorMessage, humanize, nextStatuses } from '../utils';

type Tab = 'comments' | 'history';

/**
 * Ticket page layout:
 *   header (number, subject, action bar)
 *   main column: details, description (read-only), attachments, activity (Comments | History tabs)
 *   sidebar: people, notification emails, dates, linked tickets
 */
export function TicketDetailPage() {
  const { id = '' } = useParams();
  const { user } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const ticketQuery = useTicket(id);
  const timelineQuery = useTimeline(id);
  const ticket = ticketQuery.data;
  const composerRef = useRef<HTMLTextAreaElement>(null);
  const [tab, setTab] = useState<Tab>('comments');

  // Show the "created" banner once; clear router state so it won't reappear on reload.
  type RouteState = { created?: boolean; uploadErrors?: string[]; openChat?: boolean } | null;
  const [createdState] = useState(() => location.state as RouteState);
  const [chatOpen, setChatOpen] = useState(false);
  useEffect(() => {
    if (!location.state) return;
    if ((location.state as RouteState)?.openChat) setChatOpen(true); // opened from a chat notification
    navigate(location.pathname, { replace: true, state: null });
  }, [location.state, location.pathname, navigate]);

  useMarkTicketRead(ticket?.id);

  // Return to the page the user came from; fall back to the list on direct visits.
  const goBack = () => (location.key !== 'default' ? navigate(-1) : navigate('/tickets'));

  /** Refreshes everything that may change after an action on this ticket. */
  const refreshAll = () => {
    for (const key of [['ticket', id], ['timeline', id], ['tickets'], ['summary'], ['notifications']]) {
      queryClient.invalidateQueries({ queryKey: key });
    }
  };

  const focusComposer = () => {
    setTab('comments');
    setTimeout(() => composerRef.current?.focus(), 0);
  };

  const backButton = (
    <button className="btn-link" onClick={goBack}>
      ← Back
    </button>
  );

  // Skeleton keeps the layout stable while a ticket loads (no flash of text).
  if (ticketQuery.isLoading) return <TicketSkeleton />;
  if (ticketQuery.error || !ticket || !user) {
    return (
      <>
        <ErrorMessage error={ticketQuery.error ?? new Error('Ticket not found')} onRetry={() => ticketQuery.refetch()} />
        {backButton}
      </>
    );
  }

  const isOwner = user.id === ticket.requester.id;
  const timeline = timelineQuery.data ?? [];
  const commentCount = timeline.filter((i) => i.kind === 'message').length;
  const historyCount = timeline.length - commentCount;

  return (
    <>
      {createdState?.created && (
        <div className="alert alert-success">
          Ticket <strong>{ticket.ticketNumber}</strong> created and assigned to {ticket.assignee?.name}.
        </div>
      )}
      {createdState?.uploadErrors?.length ? (
        <div className="alert alert-error">Some files were not uploaded: {createdState.uploadErrors.join('; ')}</div>
      ) : null}

      <div className="page-header">
        {backButton}
        <RefreshControl
          isFetching={ticketQuery.isFetching || timelineQuery.isFetching}
          updatedAt={ticketQuery.dataUpdatedAt}
          onRefresh={() => {
            ticketQuery.refetch();
            timelineQuery.refetch();
          }}
        />
      </div>

      <div className="card issue">
        <header className="issue-header">
          <div className="muted issue-key">{ticket.ticketNumber}</div>
          <h1>{ticket.subject}</h1>
          <ActionBar ticket={ticket} role={user.role} onDone={refreshAll} onComment={focusComposer} onChat={() => setChatOpen(true)} />
        </header>

        <div className="issue-grid">
          <div className="issue-main">
            <dl className="field-grid">
              <Field label="Type">{humanize(ticket.type)}</Field>
              <Field label="Status"><StatusBadge status={ticket.status} /></Field>
              <Field label="Severity"><SeverityBadge severity={ticket.severity} /></Field>
            </dl>

            <Section title="Description">
              <p className="body description">{ticket.description}</p>
            </Section>

            <Section title={`Attachments (${ticket.attachments.length})`}>
              <Attachments ticket={ticket} onDone={refreshAll} />
            </Section>

            <Section title="Activity">
              <div className="tabs" role="tablist">
                <TabButton active={tab === 'comments'} onClick={() => setTab('comments')}>
                  Comments ({commentCount})
                </TabButton>
                <TabButton active={tab === 'history'} onClick={() => setTab('history')}>
                  History ({historyCount})
                </TabButton>
              </div>
              {timelineQuery.error && <ErrorMessage error={timelineQuery.error} onRetry={() => timelineQuery.refetch()} />}
              {tab === 'comments' ? (
                <>
                  <CommentList items={timeline} />
                  <Composer ticket={ticket} isStaff={user.role !== 'customer'} onDone={refreshAll} inputRef={composerRef} />
                </>
              ) : (
                <HistoryList items={timeline} requesterId={ticket.requester.id} />
              )}
            </Section>
          </div>

          <aside className="issue-side">
            <dl className="side-fields">
              <Field label="Assignee">{ticket.assignee?.name ?? 'Unassigned'}</Field>
              <AssigneeSelect ticket={ticket} role={user.role} onDone={refreshAll} />
              <Field label="Reporter">{ticket.requester.name}</Field>
              <Field label="Notify">
                {ticket.notifyEmails.length ? ticket.notifyEmails.map((e) => <div key={e}>{e}</div>) : '—'}
              </Field>
              <Field label="Created">{formatDate(ticket.createdAt)}</Field>
              <Field label="Updated">{formatDate(ticket.updatedAt)}</Field>
              {ticket.solvedAt && <Field label="Solved">{formatDate(ticket.solvedAt)}</Field>}
            </dl>
            <LinkedTickets ticket={ticket} canEdit={isOwner} onDone={refreshAll} />
          </aside>
        </div>
      </div>
      {chatOpen && <ChatWindow ticket={ticket} onClose={() => setChatOpen(false)} />}
    </>
  );
}

// ---------- Small layout helpers ----------

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="field">
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="issue-section">
      <h3>{title}</h3>
      {children}
    </section>
  );
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button role="tab" aria-selected={active} className={`tab ${active ? 'active' : ''}`} onClick={onClick}>
      {children}
    </button>
  );
}

// ---------- Actions ----------

/** Comment shortcut + status transitions allowed for the user's role. */
function ActionBar({ ticket, role, onDone, onComment, onChat }: { ticket: TicketDetail; role: Role; onDone: () => void; onComment: () => void; onChat: () => void }) {
  const statusMutation = useMutation({ mutationFn: (s: string) => ticketsApi.changeStatus(ticket.id, s), onSuccess: onDone });
  const options = nextStatuses(role, ticket.status);
  const isStaff = role !== 'customer';

  return (
    <div className="action-bar">
      {isStaff && (
        <Select
          label="Change status"
          className="status-select"
          value={ticket.status}
          disabled={statusMutation.isPending || options.length === 0}
          options={[ticket.status, ...options].map((s) => ({ value: s, label: s === ticket.status ? `Status: ${humanize(s)}` : `Move to ${humanize(s)}` }))}
          onChange={(s) => statusMutation.mutate(s)}
        />
      )}
      <button className="btn btn-small btn-chat" onClick={onChat} title="Private live chat between reporter and assignee">
        💬 Chat
      </button>
      {ticket.status !== 'closed' && (
        <button className="btn btn-small" onClick={onComment}>📝 Comment</button>
      )}
      {!isStaff &&
        options.map((s) => (
          <button key={s} className="btn btn-small" disabled={statusMutation.isPending} onClick={() => statusMutation.mutate(s)}>
            Reopen ticket
          </button>
        ))}
      {statusMutation.error && <ErrorMessage error={statusMutation.error} />}
    </div>
  );
}

/** Staff can reassign; an agent who hands a ticket off loses access and returns to the list. */
function AssigneeSelect({ ticket, role, onDone }: { ticket: TicketDetail; role: Role; onDone: () => void }) {
  const navigate = useNavigate();
  const isStaff = role !== 'customer';
  const agents = useAgents(isStaff);
  const assign = useMutation({
    mutationFn: (assigneeId: string) => ticketsApi.assign(ticket.id, assigneeId),
    onSuccess: (updated) => {
      onDone();
      if (role === 'agent' && updated.assignee?.id !== ticket.assignee?.id) navigate('/tickets', { replace: true });
    },
  });

  if (!isStaff || ticket.status === 'closed') return null;
  return (
    <div className="field">
      <dt />
      <dd>
        <Select
          label="Change assignee"
          placeholder="Choose assignee…"
          value={ticket.assignee?.id ?? ''}
          disabled={assign.isPending || !agents.data}
          options={(agents.data ?? []).map((a) => ({ value: a.id, label: `${a.fullName} · ${a.role} · ${a.openTickets} open` }))}
          onChange={(v) => assign.mutate(v)}
        />
        {assign.error && <ErrorMessage error={assign.error} />}
      </dd>
    </div>
  );
}

function Composer({ ticket, isStaff, onDone, inputRef }: { ticket: TicketDetail; isStaff: boolean; onDone: () => void; inputRef: RefObject<HTMLTextAreaElement> }) {
  const [body, setBody] = useState('');
  const [isInternal, setIsInternal] = useState(false);
  const send = useMutation({
    mutationFn: () => ticketsApi.addMessage(ticket.id, body, isInternal),
    onSuccess: () => {
      setBody('');
      setIsInternal(false);
      onDone();
    },
  });

  if (ticket.status === 'closed') {
    return <p className="muted composer-closed">This ticket is closed. Create a new ticket and link this one if the problem returns.</p>;
  }

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (body.trim()) send.mutate();
  };

  return (
    <form className="composer" onSubmit={submit}>
      {send.error && <ErrorMessage error={send.error} />}
      <textarea ref={inputRef} rows={3} maxLength={5000} placeholder="Add a comment…" aria-label="Add a comment" value={body} onChange={(e) => setBody(e.target.value)} />
      <div className="row space-between">
        {isStaff ? (
          <label className="check">
            <input type="checkbox" checked={isInternal} onChange={(e) => setIsInternal(e.target.checked)} /> Internal note (hidden from customer)
          </label>
        ) : (
          <span />
        )}
        <button className="btn btn-primary" disabled={send.isPending || !body.trim()}>
          {send.isPending ? 'Sending…' : 'Comment'}
        </button>
      </div>
    </form>
  );
}

/** File list (click to download) plus drop zone to upload more. */
function Attachments({ ticket, onDone }: { ticket: TicketDetail; onDone: () => void }) {
  const [files, setFiles] = useState<File[]>([]);
  const [error, setError] = useState('');
  const upload = useMutation({
    mutationFn: async () => {
      for (const file of files) await ticketsApi.uploadAttachment(ticket.id, file);
    },
    onSuccess: () => setFiles([]),
    onError: (err) => setError(getErrorMessage(err)),
    onSettled: onDone,
  });
  const download = (a: TicketDetail['attachments'][number]) =>
    ticketsApi.downloadAttachment(ticket.id, a).catch((err) => setError(getErrorMessage(err)));

  return (
    <>
      {error && <div className="alert alert-error">{error}</div>}
      {ticket.attachments.length > 0 && (
        <ul className="attachment-list">
          {ticket.attachments.map((a) => (
            <li key={a.id}>
              <button className="btn-link" onClick={() => download(a)}>📄 {a.fileName}</button>
              <span className="muted">
                {formatBytes(a.sizeBytes)} · {a.uploader.name} · {formatDate(a.createdAt)}
              </span>
            </li>
          ))}
        </ul>
      )}
      {ticket.status !== 'closed' && (
        <>
          <FileInput files={files} onChange={(f) => { setError(''); setFiles(f); }} onError={setError} />
          {files.length > 0 && (
            <button className="btn btn-primary btn-small" disabled={upload.isPending} onClick={() => upload.mutate()}>
              {upload.isPending ? 'Uploading…' : `Upload ${files.length} file(s)`}
            </button>
          )}
        </>
      )}
    </>
  );
}

function LinkedTickets({ ticket, canEdit, onDone }: { ticket: TicketDetail; canEdit: boolean; onDone: () => void }) {
  const [picking, setPicking] = useState<string[]>([]);
  const [open, setOpen] = useState(false);
  const add = useMutation({
    mutationFn: async (ids: string[]) => {
      for (const linkedId of ids) await ticketsApi.addLink(ticket.id, linkedId);
    },
    onSuccess: () => {
      setPicking([]);
      setOpen(false);
    },
    onSettled: onDone,
  });
  const remove = useMutation({ mutationFn: (linkedId: string) => ticketsApi.removeLink(ticket.id, linkedId), onSuccess: onDone });

  return (
    <section className="issue-section">
      <h3>Linked tickets</h3>
      {ticket.linkedTickets.length === 0 && <p className="muted">None</p>}
      <ul className="plain">
        {ticket.linkedTickets.map((l) => (
          <li key={l.id} className="row space-between linked-row">
            <span>
              <Link to={`/tickets/${l.ticketNumber}`}>{l.ticketNumber}</Link> {l.subject} <StatusBadge status={l.status} />
            </span>
            {canEdit && (
              <button className="btn-link" disabled={remove.isPending} onClick={() => remove.mutate(l.id)}>
                Remove
              </button>
            )}
          </li>
        ))}
      </ul>
      {remove.error && <ErrorMessage error={remove.error} />}
      {add.error && <ErrorMessage error={add.error} />}
      {canEdit && ticket.status !== 'closed' && (open ? (
        <>
          <LinkTicketPicker selected={picking} onChange={setPicking} excludeIds={[ticket.id, ...ticket.linkedTickets.map((l) => l.id)]} />
          <div className="row">
            <button className="btn btn-primary btn-small" disabled={!picking.length || add.isPending} onClick={() => add.mutate(picking)}>
              Link selected
            </button>
            <button className="btn btn-small" onClick={() => setOpen(false)}>Cancel</button>
          </div>
        </>
      ) : (
        <button className="btn btn-small" onClick={() => setOpen(true)}>+ Link previous ticket</button>
      ))}
    </section>
  );
}

function TicketSkeleton() {
  return (
    <div className="card issue skeleton" aria-busy="true" aria-label="Loading ticket">
      <div className="sk sk-line short" />
      <div className="sk sk-title" />
      <div className="sk sk-line" />
      <div className="issue-grid">
        <div>
          <div className="sk sk-block" />
          <div className="sk sk-block tall" />
        </div>
        <div className="sk sk-block tall" />
      </div>
    </div>
  );
}
