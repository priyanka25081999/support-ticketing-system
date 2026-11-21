import { FormEvent, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { metaApi } from '../api/endpoints';
import { ErrorMessage } from '../components/common';
import { useAgents } from '../hooks/queries';
import { formatDate, humanize } from '../utils';

/** Admin-only: list staff and create new agent accounts. */
export function AgentsPage() {
  const queryClient = useQueryClient();
  const { data, error, isLoading, refetch } = useAgents(true);
  const [form, setForm] = useState({ fullName: '', email: '', password: '' });
  const create = useMutation({
    mutationFn: () => metaApi.createAgent(form.fullName, form.email, form.password),
    onSuccess: () => {
      setForm({ fullName: '', email: '', password: '' });
      queryClient.invalidateQueries({ queryKey: ['agents'] });
    },
  });

  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate();
  };

  return (
    <div className="grid-2">
      <section className="card">
        <h2>Agents & admins</h2>
        {error && <ErrorMessage error={error} onRetry={() => refetch()} />}
        {isLoading && <p className="muted">Loading…</p>}
        <table className="table">
          <tbody>
            {data?.map((a) => (
              <tr key={a.id}>
                <td>{a.fullName}</td>
                <td>{a.email}</td>
                <td><span className="badge">{humanize(a.role)}</span></td>
                <td className="muted">{formatDate(a.createdAt)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <form className="card form" onSubmit={submit}>
        <h2>Add agent</h2>
        {create.error && <ErrorMessage error={create.error} />}
        {create.isSuccess && <div className="alert alert-success">Agent created.</div>}
        <label>
          Full name
          <input required value={form.fullName} onChange={(e) => setForm({ ...form, fullName: e.target.value })} />
        </label>
        <label>
          Email
          <input type="email" required value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
        </label>
        <label>
          Temporary password
          <input type="password" required minLength={8} maxLength={72} value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
        </label>
        <button className="btn btn-primary" disabled={create.isPending}>Create agent</button>
      </form>
    </div>
  );
}

export function NotFoundPage() {
  return (
    <div className="center">
      <h2>Page not found</h2>
      <Link to="/dashboard">Go to dashboard</Link>
    </div>
  );
}
