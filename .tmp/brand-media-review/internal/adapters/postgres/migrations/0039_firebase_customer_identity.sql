CREATE TABLE firebase_identities (
 user_id BIGINT PRIMARY KEY REFERENCES users(id),
 firebase_uid VARCHAR(128) NOT NULL UNIQUE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE firebase_deletion_queue (
 firebase_uid VARCHAR(128) PRIMARY KEY,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE customer_push_subscriptions (
 token_hash CHAR(64) PRIMARY KEY,
 token TEXT NOT NULL,
 user_id BIGINT NOT NULL REFERENCES users(id),
 session_hash CHAR(64) NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 last_notification_id BIGINT NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX customer_push_user ON customer_push_subscriptions(user_id);
