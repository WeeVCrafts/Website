-- SMS has its own durable receipt. Acknowledging email/outbox transport never
-- establishes SMS acceptance or handset delivery. Historical notices are not sent.
CREATE TABLE transactional_sms (
 id bigserial PRIMARY KEY,notification_id bigint NOT NULL UNIQUE REFERENCES notifications(id),
 user_id bigint NOT NULL REFERENCES users(id),role_slug text NOT NULL CHECK(role_slug IN ('customer','seller')),
 category text NOT NULL CHECK(category IN ('order','support')),
 template_key text NOT NULL CHECK(template_key IN ('customer_order','seller_order','customer_support','seller_support')),
 reference text NOT NULL,update_label text NOT NULL,
 status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','submitting','accepted','uncertain','rejected','skipped')),
 provider_request_id text UNIQUE,outcome_reason text NOT NULL DEFAULT '',
 expires_at timestamptz NOT NULL DEFAULT now()+interval '24 hours',submitted_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX transactional_sms_queued ON transactional_sms(id) WHERE status='queued';

CREATE FUNCTION queue_transactional_sms() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE account_role text; category text; reference text;
BEGIN
 IF NEW.kind NOT IN ('order','support','seller_coordination') THEN RETURN NEW; END IF;
 SELECT role_slug INTO account_role FROM user_roles WHERE user_id=NEW.user_id AND (SELECT count(*) FROM user_roles WHERE user_id=NEW.user_id)=1;
 IF account_role IS NULL OR account_role NOT IN ('customer','seller') THEN RETURN NEW; END IF;
 category:=CASE WHEN NEW.kind='order' THEN 'order' ELSE 'support' END;
 reference:=substring(NEW.link_url from '/orders/([A-Za-z0-9_.-]+)');
 IF reference IS NULL THEN reference:=substring(NEW.link_url from 'ticket_id=([0-9]+)'); END IF;
 IF reference IS NULL THEN reference:='N'||NEW.id; END IF;
 INSERT INTO transactional_sms(notification_id,user_id,role_slug,category,template_key,reference,update_label)
 VALUES(NEW.id,NEW.user_id,account_role,category,account_role||'_'||category,left(reference,80),left(NEW.title,80))
 ON CONFLICT(notification_id) DO NOTHING;
 PERFORM marketplace_enqueue('sms-notifications',0,'sms-notification:'||NEW.id);
 RETURN NEW;
END $$;
CREATE TRIGGER notification_transactional_sms AFTER INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION queue_transactional_sms();

CREATE FUNCTION marketplace_financial_sms_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE customer bigint; seller_owner bigint; number text; label text; event_key text;
BEGIN
 SELECT o.user_id,o.order_number INTO customer,number FROM orders o WHERE o.id=NEW.order_id;
 IF TG_TABLE_NAME='marketplace_cancellations' THEN
  IF NEW.status<>'completed' OR OLD.status=NEW.status THEN RETURN NEW; END IF;
  SELECT s.owner_user_id INTO seller_owner FROM order_items oi JOIN sellers s ON s.id=oi.seller_id WHERE oi.id=NEW.order_item_id;
  label:='Items cancelled';
 ELSIF TG_TABLE_NAME='marketplace_refunds' THEN
  IF NEW.status<>'processed' OR OLD.status=NEW.status THEN RETURN NEW; END IF;
  SELECT owner_user_id INTO seller_owner FROM sellers WHERE id=NEW.seller_id;
  label:='Refund processed';
 ELSE
  IF TG_OP='UPDATE' AND OLD.status=NEW.status THEN RETURN NEW; END IF;
  SELECT owner_user_id INTO seller_owner FROM sellers WHERE id=NEW.seller_id;
  label:='Return '||replace(NEW.status,'_',' ');
 END IF;
 event_key:='sms-business:'||TG_TABLE_NAME||':'||NEW.id||':'||NEW.status;
 INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
 VALUES(customer,'order',label,'Open the order for the recorded decision.','/orders/'||number||'/fulfillment',event_key)
 ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 IF seller_owner IS NOT NULL THEN
 INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
 VALUES(seller_owner,'order',label,'Open your seller order for the recorded decision.','/seller-admin/orders/'||number||'/fulfillment',event_key)
 ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER cancellation_sms_notice AFTER UPDATE OF status ON marketplace_cancellations FOR EACH ROW EXECUTE FUNCTION marketplace_financial_sms_notice();
CREATE TRIGGER refund_sms_notice AFTER UPDATE OF status ON marketplace_refunds FOR EACH ROW EXECUTE FUNCTION marketplace_financial_sms_notice();
CREATE TRIGGER return_sms_notice AFTER INSERT OR UPDATE OF status ON marketplace_returns FOR EACH ROW EXECUTE FUNCTION marketplace_financial_sms_notice();

CREATE FUNCTION marketplace_package_sms_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE label text; number text;
BEGIN
 IF OLD.status=NEW.status THEN RETURN NEW; END IF;
 label:=CASE NEW.status WHEN 'picked_up' THEN 'Package shipped' WHEN 'ndr' THEN 'Delivery needs attention' WHEN 'rto' THEN 'Package returning to seller' WHEN 'delivered' THEN CASE WHEN NEW.delivery_method='manual' THEN 'Package delivered' END END;
 IF label IS NULL THEN RETURN NEW; END IF;
 SELECT order_number INTO number FROM orders WHERE id=NEW.order_id;
 INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
 SELECT user_id,'order',label,'Open the package for verified tracking details.','/orders/'||number||'/fulfillment','sms-package:'||NEW.id||':'||NEW.status FROM orders WHERE id=NEW.order_id
 ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 IF NEW.status IN ('ndr','rto') THEN
 INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
 SELECT owner_user_id,'order',label,'Open the seller package to review the delivery exception.','/seller-admin/orders/'||number||'/fulfillment','sms-package:'||NEW.id||':'||NEW.status FROM sellers WHERE id=NEW.seller_id
 ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER package_sms_notice AFTER UPDATE OF status ON marketplace_packages FOR EACH ROW EXECUTE FUNCTION marketplace_package_sms_notice();
