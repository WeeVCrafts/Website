$ErrorActionPreference = 'Stop'
# Keep Unicode variant names intact when piping SQL through Windows PowerShell.
$OutputEncoding = New-Object System.Text.UTF8Encoding($false)
# UI-only fixture preparation, never the active application's database.
$qaContainer = 'weev-ui-qa-20261014-postgres'
$qaDatabase = 'weev_qa_ui_20261014'
$qaDetails = docker inspect $qaContainer | ConvertFrom-Json
if ($LASTEXITCODE -ne 0) { throw 'Could not inspect the isolated QA container.' }
$qaPort = $qaDetails.NetworkSettings.Ports.'5432/tcp'[0].HostPort
if ($qaPort -ne '15432') { throw 'Unexpected QA database port; no fixtures changed.' }
$fixtureSQL = @'
BEGIN;
INSERT INTO product_variants(product_id,sku,name,price_cents,compare_at_cents)
SELECT id,'UI-QA-TOTE-LARGE','Large · natural cotton',104900,124900 FROM products WHERE slug='blue-tote'
ON CONFLICT (sku) DO UPDATE SET name=EXCLUDED.name;
INSERT INTO product_variants(product_id,sku,name,price_cents)
SELECT id,'UI-QA-TOTE-SOLDOUT','Extra large · unavailable',129900 FROM products WHERE slug='blue-tote'
ON CONFLICT (sku) DO UPDATE SET name=EXCLUDED.name;
INSERT INTO inventory_stock(variant_id,available_quantity)
SELECT id,CASE WHEN sku='UI-QA-TOTE-SOLDOUT' THEN 0 ELSE 6 END FROM product_variants WHERE sku IN ('UI-QA-TOTE-LARGE','UI-QA-TOTE-SOLDOUT')
ON CONFLICT (variant_id) DO NOTHING;
INSERT INTO products(slug,brand_slug,category_slug,name,description,price_cents,image_url,seller_id,status)
SELECT 'ui-qa-missing-media',brand_slug,category_slug,'QA only: an unusually long handmade product title for checking wrapping, keyboard use and missing photography','Isolated QA fixture; not a real product.',150000,'/assets/images/ui-qa-image-does-not-exist.webp',seller_id,'approved' FROM products WHERE slug='blue-tote'
ON CONFLICT (slug) DO NOTHING;
INSERT INTO product_variants(product_id,sku,name,price_cents)
SELECT id,'UI-QA-NO-MEDIA','Standard',150000 FROM products WHERE slug='ui-qa-missing-media'
ON CONFLICT (sku) DO NOTHING;
INSERT INTO inventory_stock(variant_id,available_quantity)
SELECT id,0 FROM product_variants WHERE sku='UI-QA-NO-MEDIA'
ON CONFLICT (variant_id) DO NOTHING;
INSERT INTO notifications(user_id,kind,title,body,source_key,read_at)
SELECT u.id,'ui-qa','UI QA '||n.state||' state','Isolated interface fixture; no private conversation content.',
 'ui-qa-20261014:'||n.state,CASE WHEN n.state='read' THEN NOW() ELSE NULL END
FROM users u CROSS JOIN (VALUES('read'),('unread')) n(state)
WHERE u.email IN ('demo.customer01@weevcrafts.local','demo.customer05@weevcrafts.local','demo.owner01@weevcrafts.local','demo.marketplace01@weevcrafts.local','demo.super01@weevcrafts.local')
ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO UPDATE SET read_at=EXCLUDED.read_at;
COMMIT;
'@
$fixtureSQL | docker exec -i $qaContainer psql -v ON_ERROR_STOP=1 -U weev_ui_qa -d $qaDatabase
if ($LASTEXITCODE -ne 0) { throw 'Isolated UI fixture preparation failed.' }
