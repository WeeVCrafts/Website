-- Usernames are account identity, independent of the account's current role.
-- Existing accounts keep email login until explicitly assigned a username.
ALTER TABLE users ADD COLUMN username TEXT;
ALTER TABLE users ADD CONSTRAINT users_username_format
  CHECK (username IS NULL OR username ~ '^[a-z][a-z0-9_]{2,31}$');
CREATE UNIQUE INDEX users_username_unique ON users (username) WHERE username IS NOT NULL;
