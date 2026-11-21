import { FormEvent, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ticketsApi } from '../api/endpoints';
import { useAuth } from '../auth/AuthContext';
import { FileInput } from '../components/FileInput';
import { LinkTicketPicker } from '../components/LinkTicketPicker';
import { ErrorMessage } from '../components/common';
import { useEnums } from '../hooks/queries';
import { getErrorMessage, humanize, validateEmailList } from '../utils';

/** Required fields: subject, description, type, notification emails. Severity, attachments and links are optional. */
export function NewTicketPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const { data: enums, error: enumsError } = useEnums();
  const [form, setForm] = useState({
    subject: '',
    description: '',
    type: '',
    severity: 'medium',
    notifyEmails: user?.email ?? '',
  });
  const [linkedIds, setLinkedIds] = useState<string[]>([]);
  const [files, setFiles] = useState<File[]>([]);
  const [formError, setFormError] = useState('');

  const create = useMutation({
    mutationFn: async () => {
      const ticket = await ticketsApi.create({ ...form, linkedTicketIds: linkedIds });
      // Upload files after the ticket exists; a failed upload must not lose the ticket.
      const failed: string[] = [];
      for (const file of files) {
        try {
          await ticketsApi.uploadAttachment(ticket.id, file);
        } catch (err) {
          failed.push(`${file.name}: ${getErrorMessage(err)}`);
        }
      }
      return { ticket, failed };
    },
    onSuccess: ({ ticket, failed }) => {
      queryClient.invalidateQueries({ queryKey: ['tickets'] });
      queryClient.invalidateQueries({ queryKey: ['summary'] });
      navigate(`/tickets/${ticket.ticketNumber}`, { state: { created: true, uploadErrors: failed } });
    },
  });

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const emailError = validateEmailList(form.notifyEmails);
    setFormError(emailError);
    if (!emailError) create.mutate();
  };

  const set = (field: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) =>
    setForm({ ...form, [field]: e.target.value });

  return (
    <form className="card form" onSubmit={submit}>
      <h1>New ticket</h1>
      <p className="muted">
        Fields marked <span className="required">*</span> are mandatory.
      </p>
      {enumsError && <ErrorMessage error={enumsError} />}
      {create.error && <ErrorMessage error={create.error} />}
      {formError && <div className="alert alert-error">{formError}</div>}

      <label>
        <span>Subject <span className="required">*</span></span>
        <input required minLength={3} maxLength={200} value={form.subject} onChange={set('subject')} />
      </label>
      <label>
        <span>Description <span className="required">*</span></span>
        <textarea required rows={6} maxLength={5000} value={form.description} onChange={set('description')} />
      </label>
      <div className="grid-2">
        <label>
          <span>Type <span className="required">*</span></span>
          <select required value={form.type} onChange={set('type')}>
            <option value="">Select type…</option>
            {enums?.types.map((t) => <option key={t} value={t}>{humanize(t)}</option>)}
          </select>
        </label>
        <label>
          <span>Severity <span className="muted">(optional)</span></span>
          <select value={form.severity} onChange={set('severity')}>
            {enums?.severities.map((s) => <option key={s} value={s}>{humanize(s)}</option>)}
          </select>
        </label>
      </div>
      <label>
        <span>Notification emails <span className="required">*</span></span>
        <input
          required
          placeholder="you@example.com; manager@example.com"
          value={form.notifyEmails}
          onChange={set('notifyEmails')}
        />
        <span className="muted">Separate multiple email addresses with a semicolon ( ; )</span>
      </label>

      <fieldset>
        <legend>Attachments (optional)</legend>
        <FileInput files={files} onChange={setFiles} onError={setFormError} />
      </fieldset>

      <fieldset>
        <legend>Link previous tickets (optional)</legend>
        <LinkTicketPicker selected={linkedIds} onChange={setLinkedIds} />
      </fieldset>

      <button className="btn btn-primary" disabled={create.isPending}>
        {create.isPending ? 'Submitting…' : 'Submit ticket'}
      </button>
    </form>
  );
}
