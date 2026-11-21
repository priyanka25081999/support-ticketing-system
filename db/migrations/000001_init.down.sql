DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS ticket_links;
DROP TABLE IF EXISTS ticket_events;
DROP TABLE IF EXISTS ticket_messages;
DROP TABLE IF EXISTS tickets;
DROP SEQUENCE IF EXISTS ticket_number_seq;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS event_type;
DROP TYPE IF EXISTS ticket_severity;
DROP TYPE IF EXISTS ticket_type;
DROP TYPE IF EXISTS ticket_status;
DROP TYPE IF EXISTS user_role;
