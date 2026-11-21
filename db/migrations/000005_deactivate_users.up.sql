-- Soft delete: removed agents keep their history (messages, events) but can no longer sign in.
ALTER TABLE users ADD COLUMN deactivated_at TIMESTAMPTZ;
