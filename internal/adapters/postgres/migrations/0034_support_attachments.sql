-- Keep support attachments on a unique migration number. Older checkouts had
-- two 0028 files; this forward-only repair is safe for both fresh and upgraded
-- databases and leaves the 0028 seller-follow migration untouched.
CREATE TABLE IF NOT EXISTS support_attachments (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    message_id BIGINT REFERENCES support_ticket_messages(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    byte_size BIGINT NOT NULL CHECK (byte_size > 0 AND byte_size <= 10485760),
    object_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS support_attachments_ticket_idx
    ON support_attachments (ticket_id, created_at, id);
