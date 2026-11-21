package service

import (
	"context"
	"sync"

	"ticketing/api/internal/apperr"
	"ticketing/api/internal/domain"
)

const (
	maxChatMessageLen = 2000
	chatHistoryLimit  = 200
	chatBufferSize    = 16
)

// chatSub is one open chat stream (one browser tab).
type chatSub struct {
	actor domain.Actor
	ch    chan domain.ChatMessage
}

// chatHub fans out new chat messages to open streams of the same ticket.
// It is in-process; running several API instances would need Redis/Postgres LISTEN.
type chatHub struct {
	mu   sync.Mutex
	subs map[string]map[*chatSub]struct{} // ticketID -> subscribers
}

func newChatHub() *chatHub { return &chatHub{subs: map[string]map[*chatSub]struct{}{}} }

func (h *chatHub) subscribe(ticketID string, actor domain.Actor) (*chatSub, func()) {
	sub := &chatSub{actor: actor, ch: make(chan domain.ChatMessage, chatBufferSize)}
	h.mu.Lock()
	if h.subs[ticketID] == nil {
		h.subs[ticketID] = map[*chatSub]struct{}{}
	}
	h.subs[ticketID][sub] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	return sub, func() {
		once.Do(func() { h.remove(ticketID, sub) })
	}
}

// remove unregisters a subscriber and closes its channel (safe to call once per sub).
func (h *chatHub) remove(ticketID string, sub *chatSub) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[ticketID][sub]; !ok {
		return
	}
	delete(h.subs[ticketID], sub)
	if len(h.subs[ticketID]) == 0 {
		delete(h.subs, ticketID)
	}
	close(sub.ch)
}

// publish delivers msg to subscribers who may still read this ticket's chat.
// A slow subscriber is disconnected (its client reconnects and reloads history).
func (h *chatHub) publish(t domain.Ticket, msg domain.ChatMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subs[t.ID] {
		if !canView(sub.actor, t) {
			continue // e.g. an agent who handed the ticket off
		}
		select {
		case sub.ch <- msg:
		default:
			delete(h.subs[t.ID], sub)
			close(sub.ch)
		}
	}
}

// canSendChat: only the reporter and the current assignee take part in the 1:1 chat.
func canSendChat(actor domain.Actor, t domain.Ticket) (bool, string) {
	isParticipant := actor.ID == t.Requester.ID || (t.Assignee != nil && actor.ID == t.Assignee.ID)
	switch {
	case !isParticipant:
		return false, "Only the reporter and the assignee can chat. You have read-only access."
	case t.Status == domain.StatusClosed:
		return false, "This ticket is closed."
	default:
		return true, ""
	}
}

// ChatThread returns chat history. Anyone who can view the ticket may read it
// (reporter, assignee, admins); only participants can send.
func (s *TicketService) ChatThread(ctx context.Context, actor domain.Actor, idOrNumber string) (domain.ChatThread, error) {
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return domain.ChatThread{}, err
	}
	msgs, err := s.repo.ListChatMessages(ctx, t.ID, chatHistoryLimit)
	if err != nil {
		return domain.ChatThread{}, err
	}
	canSend, reason := canSendChat(actor, t)
	return domain.ChatThread{Messages: msgs, CanSend: canSend, Reason: reason, Requester: t.Requester, Assignee: t.Assignee}, nil
}

// SendChat stores a chat message and pushes it to open streams.
func (s *TicketService) SendChat(ctx context.Context, actor domain.Actor, idOrNumber, body string) (domain.ChatMessage, error) {
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return domain.ChatMessage{}, err
	}
	if ok, reason := canSendChat(actor, t); !ok {
		return domain.ChatMessage{}, apperr.Forbidden(reason)
	}
	body, err = requireLength("Message", body, 1, maxChatMessageLen)
	if err != nil {
		return domain.ChatMessage{}, err
	}
	msg, err := s.repo.CreateChatMessage(ctx, t.ID, actor.ID, body)
	if err != nil {
		return domain.ChatMessage{}, err
	}
	msg.Sender.Name, msg.SenderRole = actor.Name, actor.Role
	s.chat.publish(t, msg)
	return msg, nil
}

// SubscribeChat opens a live stream for a ticket the actor can view.
// The caller must call the returned unsubscribe function.
func (s *TicketService) SubscribeChat(ctx context.Context, actor domain.Actor, idOrNumber string) (<-chan domain.ChatMessage, func(), error) {
	t, err := s.loadAccessible(ctx, actor, idOrNumber)
	if err != nil {
		return nil, nil, err
	}
	sub, unsubscribe := s.chat.subscribe(t.ID, actor)
	return sub.ch, unsubscribe, nil
}
