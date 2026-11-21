-- New lifecycle: open -> in_progress -> on_hold -> solved -> closed.
-- "pending" (waiting on the customer) becomes "on_hold"; existing rows keep their meaning.
ALTER TYPE ticket_status RENAME VALUE 'pending' TO 'on_hold';
ALTER TYPE ticket_status ADD VALUE IF NOT EXISTS 'in_progress' AFTER 'open';

-- Keep the history timeline consistent with the renamed status.
UPDATE ticket_events SET data = jsonb_set(data, '{from}', '"on_hold"') WHERE type = 'status_changed' AND data->>'from' = 'pending';
UPDATE ticket_events SET data = jsonb_set(data, '{to}', '"on_hold"')   WHERE type = 'status_changed' AND data->>'to' = 'pending';
