-- Private 1:1 chat between a ticket's reporter and its current assignee.
CREATE TABLE ticket_chat_messages (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id  UUID NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
  sender_id  UUID NOT NULL REFERENCES users (id),
  body       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ticket_chat_ticket_idx ON ticket_chat_messages (ticket_id, created_at);

-- Speeds up report queries on resolution events and created-date ranges.
CREATE INDEX ticket_events_status_idx ON ticket_events (created_at) WHERE type = 'status_changed';
CREATE INDEX tickets_created_idx ON tickets (created_at);
