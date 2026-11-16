-- Existing financial columns and historical policies remain intact.
ALTER TABLE orders ADD COLUMN workflow_version integer NOT NULL DEFAULT 1,
    ADD COLUMN lifecycle_status text NOT NULL DEFAULT 'awaiting_payment'
    CHECK (lifecycle_status IN ('awaiting_payment','confirmed','partially_shipped','partially_delivered','completed','cancelled'));
ALTER TABLE orders ALTER COLUMN workflow_version SET DEFAULT 2;
ALTER TABLE payment_attempts ADD COLUMN expiry_checked_at timestamptz,ADD COLUMN expiry_unresolved boolean NOT NULL DEFAULT false;
CREATE FUNCTION marketplace_payment_resolved_for_release(p_order bigint) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT NOT EXISTS(SELECT 1 FROM payment_attempts pa JOIN inventory_reservations ir ON ir.order_id=pa.order_id AND ir.status='active'
 WHERE pa.order_id=p_order AND pa.provider='razorpay' AND (
 (pa.provider_order_id IS NULL AND (pa.creation_started_at IS NOT NULL OR pa.creation_review_reason<>''))
 OR (pa.provider_order_id IS NOT NULL AND (pa.expiry_checked_at IS NULL OR pa.expiry_checked_at<LEAST(ir.expires_at,now()-interval '1 minute') OR pa.expiry_unresolved))))
$$;
CREATE FUNCTION marketplace_enforce_payout_hold() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status IN ('approved','paid') AND (TG_OP='INSERT' OR OLD.status IS DISTINCT FROM NEW.status) THEN
  PERFORM pg_advisory_xact_lock(4856203);
  IF EXISTS(SELECT 1 FROM seller_ledger sl WHERE sl.settlement_id=NEW.id AND sl.order_id IS NOT NULL AND sl.net_cents>0 AND NOT marketplace_payout_eligible(sl.order_id,sl.seller_id)) THEN
   RAISE EXCEPTION 'Delivery, reconciliation or exception hold blocks this payout';
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER marketplace_payout_hold BEFORE INSERT OR UPDATE OF status ON seller_settlements FOR EACH ROW EXECUTE FUNCTION marketplace_enforce_payout_hold();
ALTER TABLE order_items ADD COLUMN seller_id bigint REFERENCES sellers(id),
    ADD COLUMN cancelled_quantity integer NOT NULL DEFAULT 0 CHECK(cancelled_quantity>=0),
    ADD COLUMN refunded_quantity integer NOT NULL DEFAULT 0 CHECK(refunded_quantity>=0),
    ADD COLUMN tax_snapshot jsonb;
UPDATE order_items oi SET seller_id=p.seller_id FROM product_variants v JOIN products p ON p.id=v.product_id WHERE v.id=oi.variant_id;
ALTER TABLE order_items ADD CHECK(cancelled_quantity<=quantity AND refunded_quantity<=quantity);

ALTER TABLE seller_fulfillments ADD COLUMN lifecycle_status text NOT NULL DEFAULT 'new'
    CHECK(lifecycle_status IN ('new','accepted','packing','ready_to_ship','shipped','delivered','cancelled')),
    ADD COLUMN delivered_at timestamptz, ADD COLUMN accepted_at timestamptz,
    ADD COLUMN reminded_at timestamptz, ADD COLUMN escalated_at timestamptz;
UPDATE seller_fulfillments SET lifecycle_status=CASE status WHEN 'processing' THEN 'accepted' WHEN 'out_for_delivery' THEN 'shipped' ELSE CASE WHEN status='pending' THEN 'new' ELSE status END END,
    delivered_at=CASE WHEN status='delivered' THEN updated_at END;

CREATE TABLE marketplace_packages (
    id bigserial PRIMARY KEY, order_id bigint NOT NULL REFERENCES orders(id), seller_id bigint NOT NULL REFERENCES sellers(id),
    reference text NOT NULL UNIQUE,request_hash text NOT NULL DEFAULT '',rejected_courier_ids bigint[] NOT NULL DEFAULT '{}', status text NOT NULL DEFAULT 'created' CHECK(status IN ('created','pickup_scheduled','picked_up','in_transit','out_for_delivery','delivered','ndr','rto','cancelled')),
    provider_status text NOT NULL DEFAULT '', provider_order_id bigint UNIQUE, provider_shipment_id bigint UNIQUE,
    delivery_method text NOT NULL DEFAULT 'shiprocket' CHECK(delivery_method IN ('shiprocket','manual')),
    pickup_location text NOT NULL, weight_grams integer NOT NULL CHECK(weight_grams BETWEEN 100 AND 50000),
    length_cm integer NOT NULL CHECK(length_cm BETWEEN 1 AND 200), width_cm integer NOT NULL CHECK(width_cm BETWEEN 1 AND 200), height_cm integer NOT NULL CHECK(height_cm BETWEEN 1 AND 200),
    awb text NOT NULL DEFAULT '', courier text NOT NULL DEFAULT '', label_url text NOT NULL DEFAULT '', manifest_url text NOT NULL DEFAULT '', pickup_reference text NOT NULL DEFAULT '',
    pending_action text NOT NULL DEFAULT '', next_check_at timestamptz NOT NULL DEFAULT now(), delivered_at timestamptz,rto_inspected_at timestamptz,
    revision bigint NOT NULL DEFAULT 0, requested_by bigint NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX marketplace_packages_due ON marketplace_packages(next_check_at) WHERE status NOT IN ('delivered','cancelled');
CREATE TABLE marketplace_package_items (
    package_id bigint NOT NULL REFERENCES marketplace_packages(id), order_item_id bigint NOT NULL REFERENCES order_items(id),
    quantity integer NOT NULL CHECK(quantity>0), PRIMARY KEY(package_id,order_item_id)
);
CREATE TABLE marketplace_ndr_actions (
 id bigserial PRIMARY KEY,package_id bigint NOT NULL REFERENCES marketplace_packages(id),action text NOT NULL CHECK(action IN ('re-attempt','return')),
 preferred_date date,comments text NOT NULL,request_key text NOT NULL UNIQUE,
 status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','submitting','accepted')),created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE marketplace_history (
    id bigserial PRIMARY KEY, order_id bigint NOT NULL REFERENCES orders(id), seller_id bigint REFERENCES sellers(id),
    package_id bigint REFERENCES marketplace_packages(id), system text NOT NULL, from_status text NOT NULL DEFAULT '', to_status text NOT NULL,
    actor_id bigint REFERENCES users(id), reason text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE marketplace_cases (
    id bigserial PRIMARY KEY, order_id bigint NOT NULL REFERENCES orders(id), seller_id bigint REFERENCES sellers(id), package_id bigint REFERENCES marketplace_packages(id),
    kind text NOT NULL CHECK(kind IN ('delayed_order','failed_delivery','ndr','rto','return_dispute','refund_request','seller_escalation','delivery_dispute','unknown_status','provider_error')),
    status text NOT NULL DEFAULT 'open' CHECK(status IN ('open','resolved')), reason text NOT NULL,
    assignee_id bigint REFERENCES users(id), resolution text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), resolved_at timestamptz
);
CREATE UNIQUE INDEX marketplace_cases_open ON marketplace_cases(order_id,COALESCE(seller_id,0),COALESCE(package_id,0),kind) WHERE status='open';

CREATE TABLE marketplace_refunds (
    id bigserial PRIMARY KEY, order_id bigint NOT NULL REFERENCES orders(id), seller_id bigint NOT NULL REFERENCES sellers(id),
    order_item_id bigint REFERENCES order_items(id), quantity integer NOT NULL DEFAULT 0 CHECK(quantity>=0),
    amount_cents bigint NOT NULL CHECK(amount_cents>0), shipping_cents bigint NOT NULL DEFAULT 0 CHECK(shipping_cents>=0),
    reason text NOT NULL, kind text NOT NULL CHECK(kind IN ('cancellation','exception')),
    status text NOT NULL DEFAULT 'requested' CHECK(status IN ('requested','approved','submitting','uncertain','pending','processed','rejected','failed')),
    provider_refund_id text UNIQUE,submission_attempt integer NOT NULL DEFAULT 1 CHECK(submission_attempt>0), request_key text NOT NULL UNIQUE, approved_by bigint REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE marketplace_cancellations (
    id bigserial PRIMARY KEY, order_id bigint NOT NULL REFERENCES orders(id), order_item_id bigint NOT NULL REFERENCES order_items(id),
    quantity integer NOT NULL CHECK(quantity>0), reason text NOT NULL, request_key text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'requested' CHECK(status IN ('requested','completed','rejected')),
    requested_by bigint NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE marketplace_returns (
 id bigserial PRIMARY KEY,order_id bigint NOT NULL REFERENCES orders(id),seller_id bigint NOT NULL REFERENCES sellers(id),order_item_id bigint NOT NULL REFERENCES order_items(id),quantity integer NOT NULL CHECK(quantity>0),
 status text NOT NULL DEFAULT 'requested' CHECK(status IN ('requested','approved','pickup_pending','received','inspected','rejected','completed')),
 reason text NOT NULL,carrier text NOT NULL DEFAULT '',tracking_number text NOT NULL DEFAULT '',restocked boolean NOT NULL DEFAULT false,
 request_key text NOT NULL UNIQUE,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE payment_attempts ADD COLUMN refunded_amount_cents bigint NOT NULL DEFAULT 0 CHECK(refunded_amount_cents>=0 AND refunded_amount_cents<=amount_cents);
ALTER TABLE seller_ledger ADD COLUMN marketplace_refund_id bigint UNIQUE REFERENCES marketplace_refunds(id);
DROP INDEX seller_ledger_order_entry_unique;
CREATE UNIQUE INDEX seller_ledger_order_entry_unique ON seller_ledger(seller_id,order_id,entry_type) WHERE order_id IS NOT NULL AND marketplace_refund_id IS NULL;
CREATE TABLE marketplace_jobs (
    id bigserial PRIMARY KEY, kind text NOT NULL, resource_id bigint NOT NULL, dedupe_key text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','done','failed')),
    attempts integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL DEFAULT now(), claimed_at timestamptz,
    last_error text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX marketplace_jobs_due ON marketplace_jobs(available_at,id) WHERE status IN ('queued','running');
CREATE FUNCTION marketplace_enqueue(p_kind text,p_id bigint,p_key text) RETURNS void LANGUAGE plpgsql AS $$
DECLARE job_id bigint;
BEGIN
 INSERT INTO marketplace_jobs(kind,resource_id,dedupe_key) VALUES(p_kind,p_id,p_key) ON CONFLICT(dedupe_key) DO NOTHING RETURNING id INTO job_id;
 IF job_id IS NOT NULL THEN
 INSERT INTO outbox_events(event_type,aggregate_type,aggregate_id,payload) VALUES('MarketplaceJob','Job',job_id::text,jsonb_build_object('job_id',job_id));
 END IF;
END $$;

CREATE TABLE seller_invoice_profiles (
    seller_id bigint PRIMARY KEY REFERENCES sellers(id), legal_name text NOT NULL, address text NOT NULL,
    state_code text NOT NULL, gstin text NOT NULL DEFAULT '', registration text NOT NULL CHECK(registration IN ('registered','unregistered')),
    prefix text NOT NULL, next_number bigint NOT NULL DEFAULT 1 CHECK(next_number>0),
    verified_by bigint NOT NULL REFERENCES users(id), verified_at timestamptz NOT NULL DEFAULT now(),
    CHECK(length(legal_name) BETWEEN 2 AND 160 AND length(address) BETWEEN 10 AND 1000),
    CHECK(state_code ~ '^[0-9]{2}$'), CHECK(prefix ~ '^[A-Z0-9-]{1,8}$'),
    CHECK((registration='registered' AND gstin ~ '^[0-9]{2}[A-Z0-9]{13}$') OR (registration='unregistered' AND gstin=''))
);
CREATE TABLE variant_tax_profiles (
    variant_id bigint PRIMARY KEY REFERENCES product_variants(id), hsn text NOT NULL CHECK(hsn ~ '^[0-9]{4,8}$'),
    rate_basis_points integer NOT NULL CHECK(rate_basis_points BETWEEN 0 AND 10000),
    verified_by bigint NOT NULL REFERENCES users(id), verified_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE marketplace_invoices (
    id bigserial PRIMARY KEY, order_id bigint NOT NULL REFERENCES orders(id), seller_id bigint NOT NULL REFERENCES sellers(id),
    number text NOT NULL, snapshot jsonb NOT NULL,pdf_bytes bytea NOT NULL,pdf_sha256 text NOT NULL CHECK(length(pdf_sha256)=64), issued_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(order_id,seller_id), UNIQUE(seller_id,number)
);
ALTER TABLE seller_invoice_profiles ADD COLUMN next_credit_number bigint NOT NULL DEFAULT 1 CHECK(next_credit_number>0);
CREATE TABLE marketplace_credit_notes (
 id bigserial PRIMARY KEY,refund_id bigint NOT NULL UNIQUE REFERENCES marketplace_refunds(id),order_id bigint NOT NULL REFERENCES orders(id),seller_id bigint NOT NULL REFERENCES sellers(id),
 number text NOT NULL,snapshot jsonb NOT NULL,pdf_bytes bytea NOT NULL,pdf_sha256 text NOT NULL CHECK(length(pdf_sha256)=64),issued_at timestamptz NOT NULL DEFAULT now(),UNIQUE(seller_id,number)
);
CREATE TABLE marketplace_shipping_profiles (
    seller_id bigint PRIMARY KEY REFERENCES sellers(id), pickup_location text NOT NULL,
    postal_code text NOT NULL CHECK(postal_code ~ '^[1-9][0-9]{5}$'), verified boolean NOT NULL DEFAULT false,
    verified_by bigint REFERENCES users(id), updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE FUNCTION marketplace_order_defaults() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.workflow_version=2 THEN
 NEW.policy_snapshot=COALESCE(NEW.policy_snapshot,'{}'::jsonb)||jsonb_build_object('standard_returns_allowed',false,'settlement_hold_days',7);
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER marketplace_order_defaults BEFORE UPDATE OF policy_snapshot ON orders FOR EACH ROW EXECUTE FUNCTION marketplace_order_defaults();
CREATE FUNCTION marketplace_item_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 SELECT p.seller_id INTO NEW.seller_id FROM product_variants v JOIN products p ON p.id=v.product_id WHERE v.id=NEW.variant_id;
 SELECT jsonb_build_object('hsn',t.hsn,'rate_basis_points',t.rate_basis_points,'inclusive',true) INTO NEW.tax_snapshot FROM variant_tax_profiles t WHERE t.variant_id=NEW.variant_id;
 RETURN NEW;
END $$;
CREATE TRIGGER marketplace_item_snapshot BEFORE INSERT ON order_items FOR EACH ROW EXECUTE FUNCTION marketplace_item_snapshot();
CREATE FUNCTION marketplace_confirm_order() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status='paid' AND OLD.status IS DISTINCT FROM NEW.status THEN
 INSERT INTO seller_fulfillments(seller_id,order_id,status) SELECT DISTINCT seller_id,NEW.id,'pending' FROM order_items WHERE order_id=NEW.id AND seller_id IS NOT NULL ON CONFLICT(seller_id,order_id) DO NOTHING;
 NEW.lifecycle_status='confirmed';
 PERFORM marketplace_enqueue('notify_sellers',NEW.id,'paid:'||NEW.id);
 PERFORM marketplace_enqueue('invoice',NEW.id,'invoice:'||NEW.id);
 ELSIF NEW.status='cancelled' THEN NEW.lifecycle_status='cancelled';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER marketplace_confirm_order BEFORE UPDATE OF status ON orders FOR EACH ROW EXECUTE FUNCTION marketplace_confirm_order();
UPDATE orders SET lifecycle_status=CASE WHEN status IN ('cancelled','payment_failed') THEN 'cancelled' WHEN status IN ('delivered','refunded') THEN 'completed' WHEN status='shipped' THEN 'partially_shipped' WHEN status IN ('paid','processing') THEN 'confirmed' ELSE 'awaiting_payment' END;

-- Eligibility is shared by batch creation, approval and final bank confirmation.
CREATE FUNCTION marketplace_payout_eligible(p_order bigint,p_seller bigint) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT EXISTS(SELECT 1 FROM orders o WHERE o.id=p_order AND NOT o.payment_review_required
 AND EXISTS(SELECT 1 FROM payment_attempts pa WHERE pa.order_id=o.id AND pa.status='captured')
 AND EXISTS(SELECT 1 FROM seller_fulfillments sf WHERE sf.order_id=o.id AND sf.seller_id=p_seller AND sf.status='delivered'
   AND COALESCE(sf.delivered_at,o.delivered_at) IS NOT NULL
   AND COALESCE(sf.delivered_at,o.delivered_at)+make_interval(days=>GREATEST(7,COALESCE((o.policy_snapshot->>'settlement_hold_days')::int,7)))<=now())
 AND NOT EXISTS(SELECT 1 FROM marketplace_cases c WHERE c.order_id=o.id AND (c.seller_id IS NULL OR c.seller_id=p_seller) AND c.status='open')
 AND NOT EXISTS(SELECT 1 FROM return_requests rr WHERE rr.order_id=o.id AND rr.status NOT IN ('rejected','completed'))
 AND NOT EXISTS(SELECT 1 FROM support_tickets st WHERE st.order_id=o.id AND st.status NOT IN ('resolved','closed'))
 AND NOT EXISTS(SELECT 1 FROM payment_refunds pr WHERE pr.order_id=o.id AND pr.status='pending')
 AND NOT EXISTS(SELECT 1 FROM marketplace_refunds mr WHERE mr.order_id=o.id AND mr.seller_id=p_seller AND mr.status NOT IN ('processed','rejected'))
 AND NOT EXISTS(SELECT 1 FROM marketplace_returns mr WHERE mr.order_id=o.id AND mr.seller_id=p_seller AND mr.status NOT IN ('completed','rejected'))
 AND NOT EXISTS(SELECT 1 FROM marketplace_packages mp WHERE mp.order_id=o.id AND mp.seller_id=p_seller AND mp.status NOT IN ('delivered','cancelled')))
$$;
