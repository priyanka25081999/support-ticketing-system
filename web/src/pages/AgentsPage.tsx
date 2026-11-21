import { FormEvent, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { metaApi } from '../api/endpoints';
import type { StaffMember } from '../api/types';
import { ErrorMessage } from '../components/common';
import { Select } from '../components/Select';
import { useAgents } from '../hooks/queries';
import { formatDate, humanize } from '../utils';

const ROLE_FILTERS = [
  { value: 'all', label: 'All staff' },
  { value: 'agent', label: 'Agents' },
  { value: 'admin', label: 'Admins' },
];

/** Admin-only: list, search and remove agents; create new agent accounts. */
export function AgentsPage() {
  const queryClient = useQueryClient();
  const { data, error, isLoading, refetch } = useAgents(true);
  const [search, setSearch] = useState('');
  const [role, setRole] = useState('all');
  const [confirmId, setConfirmId] = useState<string | null>(null);
  const [notice, setNotice] = useState('');
  const [form, setForm] = useState({ fullName: '', email: '', password: '' });

  const refreshAll = () => {
    for (const key of [['agents'], ['tickets'], ['summary']]) queryClient.invalidateQueries({ queryKey: key });
  };

  const create = useMutation({
    mutationFn: () => metaApi.createAgent(form.fullName, form.email, form.password),
    onSuccess: (agent) => {
      setForm({ fullName: '', email: '', password: '' });
      setNotice(`${agent.fullName} was added as an agent.`);
      refreshAll();
    },
  });

  const remove = useMutation({
    mutationFn: (agent: StaffMember) => metaApi.removeAgent(agent.id).then((r) => ({ agent, ...r })),
    onSuccess: ({ agent, reassignedTickets }) => {
      setConfirmId(null);
      setNotice(`${agent.fullName} was removed. ${reassignedTickets} open ticket(s) were reassigned to the admin.`);
      refreshAll();
    },
  });

  const visible = useMemo(() => {
    const term = search.trim().toLowerCase();
    return (data ?? []).filter(
      (a) =>
        (role === 'all' || a.role === role) &&
        (!term || a.fullName.toLowerCase().includes(term) || a.email.toLowerCase().includes(term)),
    );
  }, [data, search, role]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate();
  };

  return (
    <>
      <div className="page-header">
        <h1>Agents</h1>
      </div>
      {notice && (
        <div className="alert alert-success">
          <span>{notice}</span>
          <button className="btn-link" onClick={() => setNotice('')}>Dismiss</button>
        </div>
      )}

      <div className="agents-grid">
        <section className="card">
          <div className="filters agents-filters">
            <div className="filter-field grow">
              <span className="filter-label">Search</span>
              <input type="search" placeholder="Search by name or email…" value={search} onChange={(e) => setSearch(e.target.value)} aria-label="Search agents" />
            </div>
            <div className="filter-field">
              <span className="filter-label">Role</span>
              <Select label="Role" value={role} options={ROLE_FILTERS} onChange={setRole} />
            </div>
          </div>

          {error && <ErrorMessage error={error} onRetry={() => refetch()} />}
          {remove.error && <ErrorMessage error={remove.error} />}
          {isLoading && <p className="muted">Loading…</p>}
          {data && visible.length === 0 && <p className="muted">No staff match your search.</p>}

          {visible.length > 0 && (
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Email</th>
                    <th>Role</th>
                    <th>Open tickets</th>
                    <th>Added</th>
                    <th aria-label="Actions" />
                  </tr>
                </thead>
                <tbody>
                  {visible.map((a) => (
                    <tr key={a.id}>
                      <td>{a.fullName}</td>
                      <td>{a.email}</td>
                      <td><span className={`badge role-${a.role}`}>{humanize(a.role)}</span></td>
                      <td>{a.openTickets}</td>
                      <td className="muted">{formatDate(a.createdAt)}</td>
                      <td className="actions-cell">
                        {a.role === 'agent' &&
                          (confirmId === a.id ? (
                            <span className="confirm">
                              Remove? Open tickets go to the admin.
                              <button className="btn btn-small btn-danger" disabled={remove.isPending} onClick={() => remove.mutate(a)}>
                                {remove.isPending ? 'Removing…' : 'Yes, remove'}
                              </button>
                              <button className="btn btn-small" onClick={() => setConfirmId(null)}>Cancel</button>
                            </span>
                          ) : (
                            <button className="btn btn-small btn-danger-outline" onClick={() => setConfirmId(a.id)}>Remove</button>
                          ))}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <form className="card form" onSubmit={submit}>
          <h2>Add agent</h2>
          {create.error && <ErrorMessage error={create.error} />}
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
    </>
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
