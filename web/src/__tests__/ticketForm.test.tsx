import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FileInput } from '../components/FileInput';
import { MAX_FILE_BYTES, validateEmailList } from '../utils';

describe('validateEmailList', () => {
  it('accepts multiple ";"-separated emails', () => {
    expect(validateEmailList('a@x.com; b@y.org ;')).toBe('');
  });

  it('requires at least one email and flags invalid ones', () => {
    expect(validateEmailList(' ; ')).toMatch(/required/);
    expect(validateEmailList('a@x.com; nope')).toBe('Invalid email address: nope');
  });
});

describe('FileInput', () => {
  it('rejects files over the size limit and keeps valid ones', async () => {
    const onChange = jest.fn();
    const onError = jest.fn();
    render(<FileInput files={[]} onChange={onChange} onError={onError} />);

    const ok = new File(['hello'], 'ok.txt', { type: 'text/plain' });
    const big = new File(['x'], 'big.bin');
    Object.defineProperty(big, 'size', { value: MAX_FILE_BYTES + 1 });

    await userEvent.upload(screen.getByLabelText('Attachments'), [ok, big]);
    expect(onError).toHaveBeenCalledWith(expect.stringContaining('big.bin'));
    expect(onChange).toHaveBeenCalledWith([ok]);
  });
});
