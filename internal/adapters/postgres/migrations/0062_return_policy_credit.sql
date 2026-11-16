CREATE TABLE marketplace_policy_settings (
 id boolean PRIMARY KEY DEFAULT true CHECK(id), published_at timestamptz, published_by bigint REFERENCES users(id),
 version text NOT NULL DEFAULT '2026-11-16-v1', shipping_mode text NOT NULL DEFAULT 'off' CHECK(shipping_mode IN ('off','pilot','live')),
 pilot_user_id bigint REFERENCES users(id), pilot_variant_id bigint REFERENCES product_variants(id), pilot_order_id bigint REFERENCES orders(id),
 updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO marketplace_policy_settings(id) VALUES(true);
CREATE TABLE marketplace_policy_versions(version text PRIMARY KEY,content jsonb NOT NULL,published_at timestamptz NOT NULL,published_by bigint NOT NULL REFERENCES users(id));
ALTER TABLE product_variants ADD COLUMN return_class text NOT NULL DEFAULT 'unclassified' CHECK(return_class IN ('unclassified','textile_craft','fragile_art'));
ALTER TABLE order_items ADD COLUMN return_class text NOT NULL DEFAULT 'unclassified',ADD COLUMN compensated_quantity integer NOT NULL DEFAULT 0 CHECK(compensated_quantity>=0 AND compensated_quantity<=quantity);
ALTER TABLE order_items ADD COLUMN financially_reversed_quantity integer NOT NULL DEFAULT 0 CHECK(financially_reversed_quantity>=0 AND financially_reversed_quantity<=quantity);
UPDATE order_items SET financially_reversed_quantity=refunded_quantity;
ALTER TABLE orders ADD COLUMN credit_cents bigint NOT NULL DEFAULT 0 CHECK(credit_cents>=0 AND credit_cents<=total_cents),ADD COLUMN funding_status text NOT NULL DEFAULT 'pending' CHECK(funding_status IN ('pending','funded','review'));
UPDATE orders SET funding_status='funded' WHERE EXISTS(SELECT 1 FROM payment_attempts p WHERE p.order_id=orders.id AND p.status IN ('captured','refunded'));
ALTER TABLE payment_attempts DROP CONSTRAINT payment_attempts_status_check;
ALTER TABLE payment_attempts ADD CHECK(status IN ('created','pending','authorized','captured','failed','refunded','funded'));

CREATE TABLE marketplace_claims (
 id bigserial PRIMARY KEY,order_id bigint NOT NULL REFERENCES orders(id),seller_id bigint NOT NULL REFERENCES sellers(id),
 order_item_id bigint NOT NULL REFERENCES order_items(id),quantity integer NOT NULL CHECK(quantity>0),
 kind text NOT NULL CHECK(kind IN ('damage','defect','wrong_item','change_of_mind')),
 resolution text NOT NULL CHECK(resolution IN ('replacement','refund','credit')),
 status text NOT NULL DEFAULT 'requested' CHECK(status IN ('requested','approved','pickup_pending','received','inspected','completed','rejected')),
 reason text NOT NULL,request_key text NOT NULL UNIQUE,policy_version text NOT NULL,
 evidence_exception text NOT NULL DEFAULT '',inspection_note text NOT NULL DEFAULT '',restock boolean NOT NULL DEFAULT false,
 return_reference text UNIQUE,return_order_id bigint UNIQUE,return_shipment_id bigint UNIQUE,return_awb text NOT NULL DEFAULT '',return_courier text NOT NULL DEFAULT '',return_status text NOT NULL DEFAULT '',pending_action text NOT NULL DEFAULT '',
 refund_id bigint REFERENCES marketplace_refunds(id),replacement_package_id bigint REFERENCES marketplace_packages(id),
 amount_cents bigint NOT NULL DEFAULT 0 CHECK(amount_cents>=0),fee_cents bigint NOT NULL DEFAULT 0 CHECK(fee_cents>=0),
 approved_by bigint REFERENCES users(id),submitted_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),
 evidence_shared boolean NOT NULL DEFAULT false
);
CREATE TABLE marketplace_claim_evidence (
 id bigserial PRIMARY KEY,claim_id bigint NOT NULL REFERENCES marketplace_claims(id),uploaded_by bigint NOT NULL REFERENCES users(id),
 object_key text NOT NULL UNIQUE,filename text NOT NULL,content_type text NOT NULL,size_bytes bigint NOT NULL CHECK(size_bytes>0 AND size_bytes<=209715200),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE store_credit_grants (
 id bigserial PRIMARY KEY,user_id bigint NOT NULL REFERENCES users(id),claim_id bigint UNIQUE REFERENCES marketplace_claims(id),
 amount_cents bigint NOT NULL CHECK(amount_cents>0),remaining_cents bigint NOT NULL CHECK(remaining_cents>=0 AND remaining_cents<=amount_cents),
 expires_at timestamptz NOT NULL,created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX store_credit_owner_expiry ON store_credit_grants(user_id,expires_at,id);
CREATE TABLE store_credit_allocations (
 order_id bigint NOT NULL REFERENCES orders(id),grant_id bigint NOT NULL REFERENCES store_credit_grants(id),amount_cents bigint NOT NULL CHECK(amount_cents>0),
 restored_cents bigint NOT NULL DEFAULT 0 CHECK(restored_cents>=0 AND restored_cents<=amount_cents),
 status text NOT NULL DEFAULT 'reserved' CHECK(status IN ('reserved','committed','released')),PRIMARY KEY(order_id,grant_id)
);
CREATE TABLE store_credit_history (
 id bigserial PRIMARY KEY,user_id bigint NOT NULL REFERENCES users(id),grant_id bigint NOT NULL REFERENCES store_credit_grants(id),
 order_id bigint REFERENCES orders(id),amount_cents bigint NOT NULL,kind text NOT NULL CHECK(kind IN ('issued','reserved','committed','released','restored')),
 source_key text NOT NULL UNIQUE,created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE marketplace_packages ADD COLUMN replacement_claim_id bigint UNIQUE REFERENCES marketplace_claims(id);
ALTER TABLE seller_fulfillments ADD COLUMN handover_at timestamptz,ADD COLUMN qc_evidence text NOT NULL DEFAULT '';
ALTER TABLE seller_ledger ADD COLUMN policy_claim_id bigint UNIQUE REFERENCES marketplace_claims(id);
DROP INDEX seller_ledger_order_entry_unique;
CREATE UNIQUE INDEX seller_ledger_order_entry_unique ON seller_ledger(seller_id,order_id,entry_type) WHERE order_id IS NOT NULL AND marketplace_refund_id IS NULL AND policy_claim_id IS NULL;
ALTER TABLE marketplace_claims ADD COLUMN replacement_reserved boolean NOT NULL DEFAULT false,ADD COLUMN replacement_committed boolean NOT NULL DEFAULT false;
ALTER TABLE marketplace_claims ADD COLUMN weight_grams integer NOT NULL DEFAULT 0,ADD COLUMN length_cm integer NOT NULL DEFAULT 0,ADD COLUMN width_cm integer NOT NULL DEFAULT 0,ADD COLUMN height_cm integer NOT NULL DEFAULT 0;
ALTER TABLE marketplace_claims ADD COLUMN delivery_package_id bigint REFERENCES marketplace_packages(id),ADD COLUMN inspection_passed boolean NOT NULL DEFAULT false;
ALTER TABLE marketplace_claims ADD COLUMN next_check_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE marketplace_refunds ADD COLUMN credit_cents bigint NOT NULL DEFAULT 0 CHECK(credit_cents>=0),ADD COLUMN compensation_cents bigint NOT NULL DEFAULT 0 CHECK(compensation_cents>=0);
ALTER TABLE marketplace_credit_notes ALTER COLUMN refund_id DROP NOT NULL;
ALTER TABLE marketplace_credit_notes ADD COLUMN claim_id bigint UNIQUE REFERENCES marketplace_claims(id);
ALTER TABLE marketplace_credit_notes ADD CHECK((refund_id IS NULL)<>(claim_id IS NULL));

CREATE OR REPLACE FUNCTION marketplace_order_defaults() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE p marketplace_policy_settings%ROWTYPE;
BEGIN
 IF NEW.workflow_version=2 AND OLD.policy_snapshot IS NULL THEN
 SELECT * INTO p FROM marketplace_policy_settings WHERE id;
 NEW.policy_snapshot=COALESCE(NEW.policy_snapshot,'{}'::jsonb)||jsonb_build_object('standard_returns_allowed',false,'settlement_hold_days',7);
 IF p.published_at IS NOT NULL THEN NEW.policy_snapshot=NEW.policy_snapshot||jsonb_build_object('return_policy_version',p.version,'damage_hours',48,'change_of_mind_hours',72,'cancellation_hours',12,'reverse_fee_cents',20000,'credit_months',6); END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION marketplace_item_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 SELECT p.seller_id,v.return_class INTO NEW.seller_id,NEW.return_class FROM product_variants v JOIN products p ON p.id=v.product_id WHERE v.id=NEW.variant_id;
 SELECT jsonb_build_object('hsn',t.hsn,'rate_basis_points',t.rate_basis_points,'inclusive',true) INTO NEW.tax_snapshot FROM variant_tax_profiles t WHERE t.variant_id=NEW.variant_id;
 RETURN NEW;
END $$;
CREATE FUNCTION marketplace_order_funded(p_order bigint) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT EXISTS(SELECT 1 FROM orders o WHERE o.id=p_order AND NOT o.payment_review_required AND o.funding_status='funded'
 AND COALESCE((SELECT sum(a.amount_cents) FROM store_credit_allocations a WHERE a.order_id=o.id AND a.status='committed'),0)=o.credit_cents
 AND (o.total_cents=o.credit_cents OR EXISTS(SELECT 1 FROM payment_attempts pa WHERE pa.order_id=o.id AND pa.provider='razorpay' AND pa.status='captured' AND pa.amount_cents=o.total_cents-o.credit_cents)))
$$;
CREATE FUNCTION marketplace_credit_release(p_order bigint) RETURNS void LANGUAGE plpgsql AS $$
DECLARE a record;
BEGIN
 PERFORM 1 FROM users WHERE id=(SELECT user_id FROM orders WHERE id=p_order) FOR UPDATE;
 FOR a IN SELECT * FROM store_credit_allocations WHERE order_id=p_order AND status='reserved' ORDER BY grant_id FOR UPDATE LOOP
 UPDATE store_credit_grants SET remaining_cents=remaining_cents+a.amount_cents WHERE id=a.grant_id;
 INSERT INTO store_credit_history(user_id,grant_id,order_id,amount_cents,kind,source_key) SELECT user_id,a.grant_id,p_order,a.amount_cents,'released','release:'||p_order||':'||a.grant_id FROM orders WHERE id=p_order ON CONFLICT DO NOTHING;
 END LOOP;
 UPDATE store_credit_allocations SET status='released' WHERE order_id=p_order AND status='reserved';
END $$;
CREATE FUNCTION marketplace_credit_order_state() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status IN ('cancelled','payment_failed') AND OLD.status='pending_payment' THEN PERFORM marketplace_credit_release(NEW.id); END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER marketplace_credit_release_order AFTER UPDATE OF status ON orders FOR EACH ROW EXECUTE FUNCTION marketplace_credit_order_state();

CREATE OR REPLACE FUNCTION marketplace_payout_eligible(p_order bigint,p_seller bigint) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT EXISTS(SELECT 1 FROM orders o WHERE o.id=p_order AND marketplace_order_funded(o.id)
 AND EXISTS(SELECT 1 FROM seller_fulfillments sf WHERE sf.order_id=o.id AND sf.seller_id=p_seller AND sf.status='delivered'
 AND COALESCE(sf.delivered_at,o.delivered_at)+make_interval(days=>GREATEST(7,COALESCE((o.policy_snapshot->>'settlement_hold_days')::int,7)))<=now())
 AND NOT EXISTS(SELECT 1 FROM marketplace_cases c WHERE c.order_id=o.id AND (c.seller_id IS NULL OR c.seller_id=p_seller) AND c.status='open')
 AND NOT EXISTS(SELECT 1 FROM return_requests rr WHERE rr.order_id=o.id AND rr.status NOT IN ('rejected','completed'))
 AND NOT EXISTS(SELECT 1 FROM support_tickets st WHERE st.order_id=o.id AND st.status NOT IN ('resolved','closed'))
 AND NOT EXISTS(SELECT 1 FROM payment_refunds pr WHERE pr.order_id=o.id AND pr.status='pending')
 AND NOT EXISTS(SELECT 1 FROM marketplace_refunds mr WHERE mr.order_id=o.id AND mr.seller_id=p_seller AND mr.status NOT IN ('processed','rejected'))
 AND NOT EXISTS(SELECT 1 FROM marketplace_returns mr WHERE mr.order_id=o.id AND mr.seller_id=p_seller AND mr.status NOT IN ('completed','rejected'))
 AND NOT EXISTS(SELECT 1 FROM marketplace_claims mc WHERE mc.order_id=o.id AND mc.seller_id=p_seller AND mc.status NOT IN ('completed','rejected'))
 AND NOT EXISTS(SELECT 1 FROM marketplace_packages mp WHERE mp.order_id=o.id AND mp.seller_id=p_seller AND mp.status NOT IN ('delivered','cancelled')))
$$;
