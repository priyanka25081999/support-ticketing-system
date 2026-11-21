import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Select } from '../components/Select';

const OPTIONS = [
  { value: '', label: 'All types' },
  { value: 'billing', label: 'Billing' },
  { value: 'technical', label: 'Technical' },
];

function Harness({ onChange = () => {} }: { onChange?: (v: string) => void }) {
  const [value, setValue] = useState('');
  return (
    <Select
      label="Type"
      value={value}
      options={OPTIONS}
      onChange={(v) => {
        setValue(v);
        onChange(v);
      }}
    />
  );
}

describe('Select', () => {
  it('opens on click and picks an option', async () => {
    const onChange = jest.fn();
    render(<Harness onChange={onChange} />);
    await userEvent.click(screen.getByRole('button', { name: 'Type: All types' }));
    expect(screen.getByRole('listbox')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('option', { name: 'Technical' }));
    expect(onChange).toHaveBeenCalledWith('technical');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Type: Technical' })).toHaveFocus();
  });

  it('supports keyboard navigation and Escape', async () => {
    const onChange = jest.fn();
    render(<Harness onChange={onChange} />);
    screen.getByRole('button').focus();
    await userEvent.keyboard('{ArrowDown}'); // open
    await userEvent.keyboard('{ArrowDown}{Enter}'); // move to Billing, pick
    expect(onChange).toHaveBeenLastCalledWith('billing');

    await userEvent.keyboard('{Enter}'); // reopen
    await userEvent.keyboard('t{Enter}'); // type-ahead to Technical
    expect(onChange).toHaveBeenLastCalledWith('technical');

    await userEvent.keyboard('{Enter}{Escape}');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('closes when clicking outside', async () => {
    render(
      <>
        <Harness />
        <p>outside</p>
      </>,
    );
    await userEvent.click(screen.getByRole('button'));
    await userEvent.click(screen.getByText('outside'));
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });
});
