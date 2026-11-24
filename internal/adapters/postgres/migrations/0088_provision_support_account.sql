-- Provision Support & Operations Admin accounts: suppport@weevcrafts.com and support@weevcrafts.com
-- Role is 'admin' (Marketplace Operations Admin), which has operational access to:
-- Support Tickets (/admin/support), Returns (/admin/returns), Refunds (/admin/finance),
-- Sellers (/admin/sellers), Customers (/admin/customers), and Orders (/admin/orders).

INSERT INTO users (email, display_name, phone, status, email_verified_at, created_at, updated_at)
VALUES 
  ('suppport@weevcrafts.com', 'WeevCrafts Support', '+91 98765 00000', 'active', NOW(), NOW(), NOW()),
  ('support@weevcrafts.com', 'WeevCrafts Support', '+91 98765 00000', 'active', NOW(), NOW(), NOW())
ON CONFLICT (email) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  phone = EXCLUDED.phone,
  status = 'active',
  email_verified_at = COALESCE(users.email_verified_at, NOW()),
  updated_at = NOW();

-- Credentials with password Support@12345
INSERT INTO credentials (user_id, password_hash, password_changed_at, failed_attempts, created_at, updated_at)
SELECT u.id, '$argon2id$v=19$m=65536,t=3,p=1$3fhQXTvc15igzCdz6EEZzQ$N+fjYP0m/1zBelWQzsQoKYeC8KlBhCn1IB8fBbDCbUo', NOW(), 0, NOW(), NOW()
FROM users u
WHERE u.email IN ('suppport@weevcrafts.com', 'support@weevcrafts.com')
ON CONFLICT (user_id) DO UPDATE SET
  password_hash = EXCLUDED.password_hash,
  failed_attempts = 0,
  locked_until = NULL,
  updated_at = NOW();

-- Role assignment to admin
INSERT INTO user_roles (user_id, role_slug)
SELECT u.id, 'admin'
FROM users u
WHERE u.email IN ('suppport@weevcrafts.com', 'support@weevcrafts.com')
ON CONFLICT (user_id, role_slug) DO NOTHING;
