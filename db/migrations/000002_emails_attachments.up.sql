-- Notification emails on tickets and file attachments.

ALTER TABLE tickets ADD COLUMN notify_emails TEXT[] NOT NULL DEFAULT '{}';

CREATE TABLE ticket_attachments (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id    UUID NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
  uploader_id  UUID NOT NULL REFERENCES users (id),
  file_name    TEXT NOT NULL,
  content_type TEXT NOT NULL,
  size_bytes   INTEGER NOT NULL CHECK (size_bytes > 0),
  data         BYTEA NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ticket_attachments_ticket_idx ON ticket_attachments (ticket_id, created_at);
