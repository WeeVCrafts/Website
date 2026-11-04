-- Fail closed: staff identities must be reviewed before losing seller scope.
DO $$ DECLARE affected TEXT; BEGIN
  IF EXISTS (SELECT 1 FROM user_roles WHERE role_slug = 'seller_staff') OR
     EXISTS (SELECT 1 FROM seller_users WHERE status = 'active') THEN
    SELECT string_agg(user_id::text,', ') INTO affected FROM (SELECT user_id FROM user_roles WHERE role_slug='seller_staff' UNION SELECT user_id FROM seller_users WHERE status='active') accounts;
 RAISE EXCEPTION 'Four-role migration blocked: resolve Seller Staff assignments and revoke active team memberships for user IDs [%] before retrying.',affected;
  END IF;
  IF EXISTS (SELECT 1 FROM user_roles GROUP BY user_id HAVING COUNT(*) > 1) THEN
    SELECT string_agg(user_id::text,', ') INTO affected FROM (SELECT user_id FROM user_roles GROUP BY user_id HAVING COUNT(*)>1) accounts;
 RAISE EXCEPTION 'Four-role migration blocked: assign exactly one role to user IDs [%] before retrying.',affected;
  END IF;
  IF EXISTS (SELECT 1 FROM users u WHERE u.status <> 'deleted' AND NOT EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id=u.id)) THEN
    SELECT string_agg(u.id::text,', ') INTO affected FROM users u WHERE u.status<>'deleted' AND NOT EXISTS(SELECT 1 FROM user_roles ur WHERE ur.user_id=u.id);
 RAISE EXCEPTION 'Four-role migration blocked: assign a role to user IDs [%] before retrying.',affected;
  END IF;
END $$;

INSERT INTO roles(slug,name) VALUES ('seller','Seller'),('admin','Admin') ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name;
UPDATE user_roles SET role_slug=CASE role_slug WHEN 'seller_owner' THEN 'seller'
  WHEN 'marketplace_admin' THEN 'admin' WHEN 'support_agent' THEN 'admin'
  WHEN 'finance_operator' THEN 'admin' WHEN 'operations_returns' THEN 'admin' ELSE role_slug END;
DELETE FROM roles WHERE slug NOT IN ('customer','seller','admin','super_admin');
ALTER TABLE roles ADD CONSTRAINT roles_four_values CHECK(slug IN ('customer','seller','admin','super_admin'));
ALTER TABLE user_roles ADD CONSTRAINT user_roles_one_assignment UNIQUE(user_id);
-- Deferred validation permits atomic replacement but forbids dropping the final assignment.
CREATE FUNCTION enforce_one_account_role() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE account_id BIGINT := COALESCE(NEW.user_id,OLD.user_id);
BEGIN
 IF EXISTS(SELECT 1 FROM users WHERE id=account_id AND status<>'deleted') AND
    (SELECT COUNT(*) FROM user_roles WHERE user_id=account_id)<>1 THEN
  RAISE EXCEPTION 'Account % requires exactly one role assignment',account_id;
 END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER account_requires_one_role AFTER INSERT OR UPDATE OR DELETE ON user_roles
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION enforce_one_account_role();
UPDATE users SET sessions_revoked_at=NOW();
UPDATE sessions_metadata SET revoked_at=NOW() WHERE revoked_at IS NULL;

-- Existing message authorship is historical evidence, not an active assignment.
ALTER TABLE support_ticket_messages DROP CONSTRAINT support_ticket_messages_author_role_check;
ALTER TABLE support_ticket_messages ADD CONSTRAINT support_ticket_messages_author_role_check
  CHECK(author_role IN ('support_agent','seller_owner','customer','seller','admin','super_admin'));
