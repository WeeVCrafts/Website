-- Shipping Support is a narrow operational role. It can coordinate support and
-- delivery work, but it is not an admin, finance, catalog, or security role.
ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_four_values;
ALTER TABLE roles ADD CONSTRAINT roles_four_values CHECK(slug IN ('customer','seller','admin','shipping_support','super_admin'));
INSERT INTO roles(slug,name) VALUES ('shipping_support','Shipping Support')
ON CONFLICT (slug) DO UPDATE SET name=EXCLUDED.name;
