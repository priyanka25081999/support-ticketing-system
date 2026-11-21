-- Initial schema for the support ticketing system (see README §5).

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE user_role       AS ENUM ('customer', 'agent', 'admin');
CREATE TYPE ticket_status   AS ENUM ('open', 'pending', 'solved', 'closed');
CREATE TYPE ticket_type     AS ENUM ('technical', 'billing', 'account', 'feature_request', 'other');
CREATE TYPE ticket_severity AS ENUM ('low', 'medium', 'high', 'critical');
CREATE TYPE event_type      AS ENUM ('created', 'assigned', 'status_changed', 'message_added', 'linked');

CREATE TABLE users (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email               CITEXT UNIQUE NOT NULL,
  password_hash       TEXT NOT NULL,
  full_name           TEXT NOT NULL,
  role                user_role NOT NULL DEFAULT 'customer',
  is_default_assignee BOOLEAN NOT NULL DEFAULT false,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Only one user can be the default assignee for new tickets.
CREATE UNIQUE INDEX one_default_assignee ON users (is_default_assignee) WHERE is_default_assignee;

CREATE SEQUENCE ticket_number_seq;
CREATE TABLE tickets (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_number TEXT UNIQUE NOT NULL
                DEFAULT 'TKT-' || lpad(nextval('ticket_number_seq')::text, 6, '0'),
  subject       VARCHAR(200) NOT NULL,
  description   TEXT NOT NULL,
  type          ticket_type NOT NULL,
  severity      ticket_severity NOT NULL DEFAULT 'medium',
  status        ticket_status NOT NULL DEFAULT 'open',
  requester_id  UUID NOT NULL REFERENCES users (id),
  assignee_id   UUID REFERENCES users (id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  solved_at     TIMESTAMPTZ
);
CREATE INDEX tickets_requester_idx ON tickets (requester_id, created_at DESC);
CREATE INDEX tickets_assignee_idx  ON tickets (assignee_id, status);
CREATE INDEX tickets_status_idx    ON tickets (status, solved_at);

CREATE TABLE ticket_messages (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ticket_id   UUID NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
  author_id   UUID NOT NULL REFERENCES users (id),
  body        TEXT NOT NULL,
  is_internal BOOLEAN NOT NULL DEFAULT false,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ticket_messages_ticket_idx ON ticket_messages (ticket_id, created_at);

CREATE TABLE ticket_events (
  id         BIGSERIAL PRIMARY KEY,
  ticket_id  UUID NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
  actor_id   UUID REFERENCES users (id),
  type       event_type NOT NULL,
  data       JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ticket_events_ticket_idx ON ticket_events (ticket_id, created_at);

CREATE TABLE ticket_links (
  ticket_id        UUID NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
  linked_ticket_id UUID NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
  relation         TEXT NOT NULL DEFAULT 'related',
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (ticket_id, linked_ticket_id),
  CHECK (ticket_id <> linked_ticket_id)
);
CREATE INDEX ticket_links_linked_idx ON ticket_links (linked_ticket_id);

CREATE TABLE refresh_tokens (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked    BOOLEAN NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX refresh_tokens_hash_idx ON refresh_tokens (token_hash);
