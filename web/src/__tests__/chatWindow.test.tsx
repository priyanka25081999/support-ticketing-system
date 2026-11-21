import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ChatWindow } from '../components/ChatWindow';
import type { TicketDetail } from '../api/types';

const mutate = jest.fn();
jest.mock('../auth/AuthContext', () => ({ useAuth: () => ({ user: { id: 'cust', role: 'customer' } }) }));
jest.mock('../hooks/useChat', () => ({
  useChat: () => ({
    status: 'live',
    thread: {
      isLoading: false,
      error: null,
      data: {
        canSend: true,
        messages: [
          { id: '1', ticketId: 't', sender: { id: 'admin', name: 'Default Admin' }, senderRole: 'admin', body: 'Hello', createdAt: '2026-11-21T08:00:00Z' },
          { id: '2', ticketId: 't', sender: { id: 'cust', name: 'Cathy' }, senderRole: 'customer', body: 'Hi', createdAt: '2026-11-21T08:01:00Z' },
        ],
      },
    },
    send: { mutate, isPending: false, error: null },
  }),
}));

const ticket = {
  id: 't', ticketNumber: 'TKT-000002', requester: { id: 'cust', name: 'Cathy' }, assignee: { id: 'admin', name: 'Default Admin' },
} as TicketDetail;

describe('ChatWindow', () => {
  beforeEach(() => mutate.mockReset());

  it('shows the partner, aligns bubbles and sends on Enter (Shift+Enter = newline)', async () => {
    render(<ChatWindow ticket={ticket} onClose={() => {}} />);
    expect(screen.getByRole('dialog', { name: 'Chat with Default Admin' })).toBeInTheDocument();
    expect(screen.getByText('Hello').closest('.chat-msg')).toHaveClass('theirs');
    expect(screen.getByText('Hi').closest('.chat-msg')).toHaveClass('mine');

    const input = screen.getByLabelText('Chat message');
    await userEvent.type(input, 'line one{Shift>}{Enter}{/Shift}line two');
    expect(mutate).not.toHaveBeenCalled();
    await userEvent.type(input, '{Enter}');
    expect(mutate).toHaveBeenCalledWith('line one\nline two', expect.anything());
  });
});
