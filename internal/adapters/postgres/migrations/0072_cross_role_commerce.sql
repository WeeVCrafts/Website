-- Read-side union keeps historical and current records distinguishable. Every
-- application read must still apply current actor/order/seller authorization.
CREATE VIEW commerce_request_summary AS
SELECT 'legacy_return'::text AS kind,r.id,r.order_id,NULL::bigint AS seller_id,
 'Order return'::text AS item,''::text AS seller,r.status,r.reason,o.total_cents AS amount_cents,r.created_at
FROM return_requests r JOIN orders o ON o.id=r.order_id
UNION ALL
SELECT 'legacy_refund',r.id,r.order_id,NULL::bigint,'Order refund','',r.status,r.reason,r.amount_cents,r.created_at FROM payment_refunds r
UNION ALL
SELECT 'return',r.id,r.order_id,r.seller_id,i.product_name,s.display_name,r.status,r.reason,
 (i.line_total_cents-COALESCE(i.allocated_discount_cents,0))*r.quantity/NULLIF(i.quantity,0),r.created_at
FROM marketplace_returns r JOIN order_items i ON i.id=r.order_item_id JOIN sellers s ON s.id=r.seller_id
UNION ALL
SELECT 'claim',r.id,r.order_id,r.seller_id,i.product_name,s.display_name,r.status,r.reason,r.amount_cents,r.submitted_at
FROM marketplace_claims r JOIN order_items i ON i.id=r.order_item_id JOIN sellers s ON s.id=r.seller_id
UNION ALL
SELECT 'refund',r.id,r.order_id,r.seller_id,COALESCE(i.product_name,'Order items'),s.display_name,r.status,r.reason,r.amount_cents+r.shipping_cents,r.created_at
FROM marketplace_refunds r LEFT JOIN order_items i ON i.id=r.order_item_id JOIN sellers s ON s.id=r.seller_id;

ALTER TABLE seller_invoice_profiles ADD COLUMN revision bigint NOT NULL DEFAULT 1,
 ADD COLUMN checkout_review_hold boolean NOT NULL DEFAULT false;
ALTER TABLE variant_tax_profiles ADD COLUMN revision bigint NOT NULL DEFAULT 1,
 ADD COLUMN checkout_review_hold boolean NOT NULL DEFAULT false;

CREATE TABLE marketplace_setup_submissions (
 id bigserial PRIMARY KEY, seller_id bigint NOT NULL REFERENCES sellers(id),
 variant_id bigint REFERENCES product_variants(id),kind text NOT NULL CHECK(kind IN ('invoice','tax')),
 status text NOT NULL CHECK(status IN ('draft','submitted','changes_requested','approved')),
 payload jsonb NOT NULL, note text NOT NULL DEFAULT '', review_note text NOT NULL DEFAULT '',
 revision bigint NOT NULL DEFAULT 1, created_by bigint NOT NULL REFERENCES users(id),
 reviewed_by bigint REFERENCES users(id), submitted_at timestamptz, reviewed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK((kind='invoice' AND variant_id IS NULL) OR (kind='tax' AND variant_id IS NOT NULL))
);
CREATE UNIQUE INDEX setup_submission_active_invoice ON marketplace_setup_submissions(seller_id)
 WHERE kind='invoice' AND status IN ('draft','submitted','changes_requested');
CREATE UNIQUE INDEX setup_submission_active_tax ON marketplace_setup_submissions(variant_id)
 WHERE kind='tax' AND status IN ('draft','submitted','changes_requested');
CREATE INDEX setup_submission_review_queue ON marketplace_setup_submissions(status,updated_at,id);

CREATE TABLE order_seller_invoice_snapshots (
 order_id bigint NOT NULL REFERENCES orders(id),seller_id bigint NOT NULL REFERENCES sellers(id),
 profile_revision bigint NOT NULL,snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(order_id,seller_id)
);
-- Only existing immutable invoice evidence can supply a historical snapshot.
INSERT INTO order_seller_invoice_snapshots(order_id,seller_id,profile_revision,snapshot)
 SELECT order_id,seller_id,0,snapshot->'Seller' FROM marketplace_invoices
 WHERE jsonb_typeof(snapshot->'Seller')='object' ON CONFLICT DO NOTHING;

CREATE FUNCTION commerce_snapshot_seller_invoice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE profile seller_invoice_profiles%ROWTYPE;
BEGIN
 SELECT * INTO profile FROM seller_invoice_profiles WHERE seller_id=NEW.seller_id FOR SHARE;
 IF FOUND THEN
  IF profile.checkout_review_hold THEN RAISE EXCEPTION 'Seller invoice details await verification'; END IF;
  INSERT INTO order_seller_invoice_snapshots(order_id,seller_id,profile_revision,snapshot)
   VALUES(NEW.order_id,NEW.seller_id,profile.revision,jsonb_build_object('SellerID',profile.seller_id,'LegalName',profile.legal_name,'Address',profile.address,'StateCode',profile.state_code,'GSTIN',profile.gstin,'Registration',profile.registration,'Prefix',profile.prefix)) ON CONFLICT DO NOTHING;
 END IF;
 IF EXISTS(SELECT 1 FROM variant_tax_profiles WHERE variant_id=NEW.variant_id AND checkout_review_hold) THEN
  RAISE EXCEPTION 'Product tax details await verification';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER commerce_snapshot_seller_invoice AFTER INSERT ON order_items FOR EACH ROW EXECUTE FUNCTION commerce_snapshot_seller_invoice();

CREATE OR REPLACE VIEW marketplace_variant_readiness AS
SELECT v.id AS variant_id,p.seller_id,p.name AS product,v.sku,s.display_name AS seller,
 v.shipping_weight_g,v.shipping_length_cm,v.shipping_width_cm,v.shipping_height_cm,
 COALESCE(sp.verified,false) AS pickup_ready,
 (ip.seller_id IS NOT NULL AND NOT ip.checkout_review_hold) AS invoice_ready,
 COALESCE(tp.variant_id IS NOT NULL AND NOT tp.checkout_review_hold AND (ip.registration<>'unregistered' OR tp.rate_basis_points=0),false) AS tax_ready,
 v.return_class<>'unclassified' AS classified,COALESCE(i.available_quantity,0) AS stock,lv.variant_id IS NOT NULL AS selected
FROM product_variants v JOIN products p ON p.id=v.product_id AND p.status='approved'
JOIN sellers s ON s.id=p.seller_id AND s.status='active'
LEFT JOIN marketplace_shipping_profiles sp ON sp.seller_id=s.id
LEFT JOIN seller_invoice_profiles ip ON ip.seller_id=s.id
LEFT JOIN variant_tax_profiles tp ON tp.variant_id=v.id
LEFT JOIN inventory_stock i ON i.variant_id=v.id
LEFT JOIN marketplace_launch_variants lv ON lv.variant_id=v.id WHERE v.status='active';
