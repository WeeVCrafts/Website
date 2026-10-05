CREATE TABLE IF NOT EXISTS support_ticket_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    author_role TEXT NOT NULL CHECK (author_role IN ('support_agent')),
    body TEXT NOT NULL CHECK (length(btrim(body)) BETWEEN 1 AND 4000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS support_ticket_messages_latest_idx
    ON support_ticket_messages (ticket_id, created_at DESC, id DESC);
