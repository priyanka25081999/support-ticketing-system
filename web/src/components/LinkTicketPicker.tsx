import { useMemo, useState } from 'react';
import { useTickets } from '../hooks/queries';
import { StatusBadge } from './common';

interface Props {
  selected: string[];
  onChange: (ids: string[]) => void;
  excludeIds?: string[];
}

/** Lets a customer pick their own solved/closed tickets to link, with search. */
export function LinkTicketPicker({ selected, onChange, excludeIds = [] }: Props) {
  const [search, setSearch] = useState('');
  const { data, isLoading } = useTickets({ pageSize: 100, sort: 'newest' });

  const candidates = useMemo(() => {
    const term = search.trim().toLowerCase();
    return (data?.items ?? []).filter(
      (t) =>
        (t.status === 'solved' || t.status === 'closed') &&
        !excludeIds.includes(t.id) &&
        (!term || t.subject.toLowerCase().includes(term) || t.ticketNumber.toLowerCase().includes(term)),
    );
  }, [data, search, excludeIds]);

  const toggle = (id: string) =>
    onChange(selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id]);

  return (
    <div className="link-picker">
      <input placeholder="Search previous tickets…" value={search} onChange={(e) => setSearch(e.target.value)} />
      {isLoading && <p className="muted">Loading…</p>}
      {!isLoading && candidates.length === 0 && <p className="muted">No solved or closed tickets to link.</p>}
      <ul>
        {candidates.map((t) => (
          <li key={t.id}>
            <label>
              <input type="checkbox" checked={selected.includes(t.id)} onChange={() => toggle(t.id)} />
              <strong>{t.ticketNumber}</strong> {t.subject} <StatusBadge status={t.status} />
            </label>
          </li>
        ))}
      </ul>
    </div>
  );
}
