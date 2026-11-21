import { FormEvent, useEffect, useState } from 'react';
import { FileInput } from '../components/FileInput';
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ticketsApi } from '../api/endpoints';
import type { TicketDetail } from '../api/types';
import { useAuth } from '../auth/AuthContext';
import { ErrorMessage, RefreshControl, SeverityBadge, StatusBadge } from '../components/common';
import { LinkTicketPicker } from '../components/LinkTicketPicker';
import { Timeline } from '../components/Timeline';
import { useAgents, useTicket, useTimeline } from '../hooks/queries';
import { formatBytes, formatDate, getErrorMessage, humanize, nextStatuses } from '../utils';

export function TicketDetailPage() {
  const { id = '' } = useParams();
  const { user } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  // Show the "created" banner once; clear router state so it won't reappear on reload.
  const [createdState] = useState(() => location.state as { created?: boolean; uploadErrors?: string[] } | null);
  const justCreated = Boolean(createdState?.created);
  useEffect(() => {
    if (location.state) navigate(location.pathname, { replace: true, state: null });
  }, [location.state, location.pathname, navigate]);
  const queryClient = useQueryClient();
  const ticketQuery = useTicket(id);
  const timelineQuery = useTimeline(id);
  const ticket = ticketQuery.data;

  /** Refreshes everything that may change after an action on this ticket. */
  const refreshAll = () => {
    queryClient.invalidateQueries({ queryKey: ['ticket', id] });
    queryClient.invalidateQueries({ queryKey: ['timeline', id] });
    queryClient.invalidateQueries({ queryKey: ['tickets'] });
    queryClient.invalidateQueries({ queryKey: ['summary'] });
  };

  if (ticketQuery.isLoading) return <p className="muted">Loading ticket…</p>;
  if (ticketQuery.error || !ticket) {
    return (
      <>
        <ErrorMessage error={ticketQuery.error ?? new Error('Ticket not found')} onRetry={() => ticketQuery.refetch()} />
        <Link to="/tickets">← Back to tickets</Link>
      </>
    );
  }

  const isOwner = user?.id === ticket.requester.id;

  return (
    <>
      {justCreated && (
        <div className="alert alert-success">
          Ticket <strong>{ticket.ticketNumber}</strong> created and assigned to {ticket.assignee?.name}.
        </div>
      )}
      {createdState?.uploadErrors?.length ? (
        <div className="alert alert-error">Some files were not uploaded: {createdState.uploadErrors.join('; ')}</div>
      ) : null}
      <div className="page-header">
        <Link to="/tickets">← Back</Link>
        <RefreshControl
          isFetching={ticketQuery.isFetching || timelineQuery.isFetching}
          updatedAt={ticketQuery.dataUpdatedAt}
          onRefresh={() => {
            ticketQuery.refetch();
            timelineQuery.refetch();
          }}
        />
      </div>

      <section className="card">
        <div className="row space-between">
          <h1>
            {ticket.ticketNumber}: {ticket.subject}
          </h1>
          <StatusBadge status={ticket.status} />
        </div>
        <div className="ticket-meta">
          <span>Severity: <SeverityBadge severity={ticket.severity} /></span>
          <span>Type: {humanize(ticket.type)}</span>
          <span>Requester: {ticket.requester.name}</span>
          <span>Assignee: {ticket.assignee?.name ?? 'Unassigned'}</span>
          <span>Created: {formatDate(ticket.createdAt)}</span>
          {ticket.notifyEmails.length > 0 && <span>Notify: {ticket.notifyEmails.join('; ')}</span>}
        </div>
        <p className="body">{ticket.description}</p>
        {user && <TicketActions ticket={ticket} role={user.role} onDone={refreshAll} />}
      </section>

      <div className="detail-grid">
        <div>
          {timelineQuery.error && <ErrorMessage error={timelineQuery.error} onRetry={() => timelineQuery.refetch()} />}
          {timelineQuery.data && <Timeline items={timelineQuery.data} />}
          <MessageComposer ticket={ticket} isStaff={user?.role !== 'customer'} onDone={refreshAll} />
        </div>
        <div>
          <AttachmentsPanel ticket={ticket} onDone={refreshAll} />
          <LinkedPanel ticket={ticket} canEdit={isOwner} onDone={refreshAll} />
        </div>
      </div>
    </>
  );
}

/** Status buttons (role-aware) and admin reassignment. */
function TicketActions({ ticket, role, onDone }: { ticket: TicketDetail; role: 'customer' | 'agent' | 'admin'; onDone: () => void }) {
  const navigate = useNavigate();
  const isStaff = role !== 'customer';
  const agents = useAgents(isStaff);
  const statusMutation = useMutation({ mutationFn: (s: string) => ticketsApi.changeStatus(ticket.id, s), onSuccess: onDone });
  const assignMutation = useMutation({
    mutationFn: (a: string) => ticketsApi.assign(ticket.id, a),
    onSuccess: (updated) => {
      onDone();
      // An agent who hands the ticket to someone else can no longer view it.
      if (role === 'agent' && updated.assignee?.id !== ticket.assignee?.id) {
        navigate('/tickets', { replace: true });
      }
    },
  });
  const options = nextStatuses(role, ticket.status);

  if (options.length === 0 && !isStaff) return null;

  return (
    <div className="actions">
      {options.map((s) => (
        <button key={s} className="btn" disabled={statusMutation.isPending} onClick={() => statusMutation.mutate(s)}>
          {role === 'customer' ? 'Reopen ticket' : `Mark ${humanize(s)}`}
        </button>
      ))}
      {isStaff && ticket.status !== 'closed' && (
        <label className="inline">
          Assign to
          <select
            value={ticket.assignee?.id ?? ''}
            disabled={assignMutation.isPending || !agents.data}
            onChange={(e) => assignMutation.mutate(e.target.value)}
          >
            {agents.data?.map((a) => (
              <option key={a.id} value={a.id}>
                {a.fullName} ({a.role})
              </option>
            ))}
          </select>
        </label>
      )}
      {statusMutation.error && <ErrorMessage error={statusMutation.error} />}
      {assignMutation.error && <ErrorMessage error={assignMutation.error} />}
    </div>
  );
}

function MessageComposer({ ticket, isStaff, onDone }: { ticket: TicketDetail; isStaff: boolean; onDone: () => void }) {
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
    return <p className="card muted">This ticket is closed. Create a new ticket and link this one if the problem returns.</p>;
  }

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (body.trim()) send.mutate();
  };

  return (
    <form className="card composer" onSubmit={submit}>
      {send.error && <ErrorMessage error={send.error} />}
      <textarea rows={3} maxLength={5000} placeholder="Write a reply…" value={body} onChange={(e) => setBody(e.target.value)} />
      <div className="row space-between">
        {isStaff ? (
          <label className="muted">
            <input type="checkbox" checked={isInternal} onChange={(e) => setIsInternal(e.target.checked)} /> Internal note (hidden from customer)
          </label>
        ) : (
          <span />
        )}
        <button className="btn btn-primary" disabled={send.isPending || !body.trim()}>
          {send.isPending ? 'Sending…' : 'Send'}
        </button>
      </div>
    </form>
  );
}

function LinkedPanel({ ticket, canEdit, onDone }: { ticket: TicketDetail; canEdit: boolean; onDone: () => void }) {
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
    <aside className="card">
      <h3>Linked tickets</h3>
      {ticket.linkedTickets.length === 0 && <p className="muted">None</p>}
      <ul className="plain">
        {ticket.linkedTickets.map((l) => (
          <li key={l.id} className="row space-between">
            <span>
              <Link to={`/tickets/${l.ticketNumber}`}>{l.ticketNumber}</Link> {l.subject} <StatusBadge status={l.status} />
            </span>
            {canEdit && (
              <button className="btn btn-small btn-link" disabled={remove.isPending} onClick={() => remove.mutate(l.id)}>
                Remove
              </button>
            )}
          </li>
        ))}
      </ul>
      {remove.error && <ErrorMessage error={remove.error} />}
      {add.error && <ErrorMessage error={add.error} />}
      {canEdit && ticket.status !== 'closed' && (
        open ? (
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
        )
      )}
    </aside>
  );
}

/** Lists files (download on click) and lets anyone with access upload more. */
function AttachmentsPanel({ ticket, onDone }: { ticket: TicketDetail; onDone: () => void }) {
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
    <aside className="card">
      <h3>Attachments</h3>
      {error && <div className="alert alert-error">{error}</div>}
      {ticket.attachments.length === 0 && <p className="muted">None</p>}
      <ul className="plain">
        {ticket.attachments.map((a) => (
          <li key={a.id}>
            <button className="btn-link" onClick={() => download(a)}>📄 {a.fileName}</button>{' '}
            <span className="muted">{formatBytes(a.sizeBytes)} · {a.uploader.name}</span>
          </li>
        ))}
      </ul>
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
    </aside>
  );
}
