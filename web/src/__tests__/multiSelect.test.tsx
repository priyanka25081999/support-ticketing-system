import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MultiSelect } from '../components/MultiSelect';

function Harness({ onChange }: { onChange: (v: string[]) => void }) {
  const [values, setValues] = useState<string[]>([]);
  return (
    <MultiSelect
      label="Status"
      allLabel="All statuses"
      values={values}
      options={[{ value: 'open', label: 'Open' }, { value: 'pending', label: 'Pending' }, { value: 'solved', label: 'Solved' }]}
      onChange={(v) => {
        setValues(v);
        onChange(v);
      }}
    />
  );
}

test('selects several values, summarises them and clears', async () => {
  const onChange = jest.fn();
  render(<Harness onChange={onChange} />);
  await userEvent.click(screen.getByRole('button', { name: 'Status: All statuses' }));
  await userEvent.click(screen.getByRole('option', { name: 'Open' }));
  await userEvent.click(screen.getByRole('option', { name: 'Pending' }));
  expect(onChange).toHaveBeenLastCalledWith(['open', 'pending']);
  expect(screen.getByRole('listbox')).toBeInTheDocument(); // stays open while toggling
  expect(screen.getByRole('button', { name: 'Status: Open, Pending' })).toBeInTheDocument();

  await userEvent.click(screen.getByRole('option', { name: 'Solved' }));
  expect(screen.getByRole('button', { name: 'Status: 3 selected' })).toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: 'Clear selection' }));
  expect(onChange).toHaveBeenLastCalledWith([]);
});

test('rapid toggles keep earlier choices even before the parent re-renders', async () => {
  const onChange = jest.fn();
  // Parent never updates `values` (like URL state that hasn't caught up yet).
  render(
    <MultiSelect label="Status" allLabel="All statuses" values={[]} onChange={onChange}
      options={[{ value: 'open', label: 'Open' }, { value: 'closed', label: 'Closed' }]} />,
  );
  await userEvent.click(screen.getByRole('button', { name: 'Status: All statuses' }));
  await userEvent.click(screen.getByRole('option', { name: 'Open' }));
  await userEvent.click(screen.getByRole('option', { name: 'Closed' }));
  expect(onChange).toHaveBeenLastCalledWith(['open', 'closed']);
});
