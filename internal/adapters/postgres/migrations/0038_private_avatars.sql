CREATE TABLE user_avatars (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    object_key TEXT NOT NULL UNIQUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Durable cleanup survives storage outages and account deletion transactions.
CREATE TABLE avatar_cleanup (
    object_key TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE FUNCTION queue_avatar_cleanup() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' OR OLD.object_key <> NEW.object_key THEN
        INSERT INTO avatar_cleanup(object_key) VALUES(OLD.object_key) ON CONFLICT DO NOTHING;
    END IF;
    RETURN NULL;
END;
$$;
CREATE TRIGGER user_avatar_cleanup AFTER UPDATE OR DELETE ON user_avatars
    FOR EACH ROW EXECUTE FUNCTION queue_avatar_cleanup();
