-- Notification read state.
-- notifications_read_at: "mark all as read" watermark (existing users start fully read).
-- ticket_reads: per-ticket watermark, updated when the user opens a ticket.
ALTER TABLE users ADD COLUMN notifications_read_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE ticket_reads (
  user_id   UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  ticket_id UUID NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
  read_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, ticket_id)
);
