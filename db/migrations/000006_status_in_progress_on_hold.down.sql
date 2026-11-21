-- Postgres cannot drop an enum value; tickets in 'in_progress' are moved back to 'open'.
UPDATE tickets SET status = 'open' WHERE status = 'in_progress';
UPDATE ticket_events SET data = jsonb_set(data, '{from}', '"pending"') WHERE type = 'status_changed' AND data->>'from' = 'on_hold';
UPDATE ticket_events SET data = jsonb_set(data, '{to}', '"pending"')   WHERE type = 'status_changed' AND data->>'to' = 'on_hold';
ALTER TYPE ticket_status RENAME VALUE 'on_hold' TO 'pending';
