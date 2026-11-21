ALTER TABLE marketplace_cases DROP CONSTRAINT marketplace_cases_kind_check;
ALTER TABLE marketplace_cases ADD CHECK(kind IN ('delayed_order','failed_delivery','ndr','rto','return_dispute','refund_request','seller_escalation','delivery_dispute','unknown_status','provider_error','cancellation_review','pickup_clearance'));

INSERT INTO marketplace_cases(order_id,seller_id,package_id,kind,reason)
SELECT order_id,seller_id,id,'pickup_clearance','Verify that this AWB was removed from the courier pickup or that the courier closed the pickup without collecting it.'
FROM marketplace_packages
WHERE delivery_method='shiprocket' AND status='cancelled' AND (awb<>'' OR pickup_reference<>'')
ON CONFLICT DO NOTHING;

ALTER TABLE notifications ADD COLUMN marketplace_package_id bigint REFERENCES marketplace_packages(id) ON DELETE SET NULL;

-- Match historical queued events by their original package identity.
UPDATE notifications n SET marketplace_package_id=p.id
FROM marketplace_packages p JOIN orders o ON o.id=p.order_id
WHERE n.kind='order' AND n.title IN ('Package shipped','Package out for delivery','Package delivered')
AND (n.source_key IN ('sms-package:'||p.id||':picked_up','sms-package:'||p.id||':delivered')
     OR (n.user_id=o.user_id AND n.link_url='/orders/'||o.order_number||'/fulfillment'
         AND n.body='Package '||p.reference||' for order '||o.order_number));

CREATE FUNCTION marketplace_package_notice_current(n notifications) RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT CASE WHEN n.kind<>'order' OR n.title NOT IN ('Package packed','Package shipped','Package out for delivery','Package delivered') THEN true
 ELSE EXISTS (
   SELECT 1 FROM marketplace_packages p WHERE p.id=n.marketplace_package_id
   AND CASE n.title
     WHEN 'Package packed' THEN p.status IN ('created','pickup_scheduled')
       AND p.pending_action<>'cancel'
       AND NOT EXISTS (
         SELECT 1 FROM marketplace_package_items pi JOIN marketplace_cancellations c ON c.order_item_id=pi.order_item_id
         WHERE pi.package_id=p.id AND c.status='requested'
       )
     WHEN 'Package shipped' THEN p.status IN ('picked_up','in_transit')
     WHEN 'Package out for delivery' THEN p.status='out_for_delivery'
     WHEN 'Package delivered' THEN p.status='delivered'
   END
 ) END
$$;

CREATE OR REPLACE FUNCTION marketplace_queue_notification_email() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.kind='order' AND NEW.title IN ('New paid order','Package packed','Package out for delivery','Package delivered','Cancellation requested','Items cancelled','Cancellation needs review','Refund pending','Refund processed'))
 OR (NEW.kind='account' AND NEW.title IN ('Seller details submitted','Seller details approved','Seller details changes requested')) THEN
  PERFORM marketplace_enqueue('transactional_email',NEW.id,'notification-email:'||NEW.id);
 END IF;
 RETURN NEW;
END $$;

DROP TRIGGER marketplace_processed_refund_notification ON marketplace_refunds;
DROP FUNCTION marketplace_notify_processed_refund();
DROP TRIGGER cancellation_sms_notice ON marketplace_cancellations;
DROP TRIGGER refund_sms_notice ON marketplace_refunds;

CREATE FUNCTION marketplace_cancellation_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE label text; message text; item text; number text; buyer bigint; seller_owner bigint; key text;
BEGIN
 IF TG_OP='INSERT' AND NEW.status='requested' THEN
  label:='Cancellation requested';
 ELSIF TG_OP='UPDATE' AND OLD.status<>NEW.status AND NEW.status='completed' THEN
  label:='Items cancelled';
 ELSIF TG_OP='UPDATE' AND OLD.status<>NEW.status AND NEW.status='rejected' THEN
  label:='Cancellation needs review';
 ELSE
  RETURN NEW;
 END IF;
 SELECT o.order_number,o.user_id,oi.product_name,s.owner_user_id INTO number,buyer,item,seller_owner
 FROM orders o JOIN order_items oi ON oi.id=NEW.order_item_id LEFT JOIN sellers s ON s.id=oi.seller_id WHERE o.id=NEW.order_id;
 message:=format('%s for %s × %s in order %s.',label,NEW.quantity,item,number);
 IF NEW.status='rejected' THEN
  message:=message||' The courier confirmed collection. Open your order to get help.';
 END IF;
 key:='marketplace-cancellation:'||NEW.id||':'||NEW.status;
 INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
 VALUES(buyer,'order',label,message,'/orders/'||number||'?tab=returns',key)
 ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 IF seller_owner IS NOT NULL AND NEW.status<>'rejected' THEN
  INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
  VALUES(seller_owner,'order',label,message||' Do not hand over the affected parcel. Check its pickup status in Packages.',
    '/seller-admin/orders/'||number||'?tab=packages',key)
  ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER marketplace_cancellation_notice AFTER INSERT OR UPDATE OF status ON marketplace_cancellations
FOR EACH ROW EXECUTE FUNCTION marketplace_cancellation_notice();

CREATE FUNCTION marketplace_refund_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE label text; message text; item text; amount text; number text; buyer bigint; key text;
BEGIN
 IF TG_OP='INSERT' AND NEW.kind='cancellation' AND NEW.status='requested' THEN
  label:='Refund pending';
 ELSIF TG_OP='UPDATE' AND OLD.status<>NEW.status AND NEW.status='processed'
       AND COALESCE(NEW.provider_refund_id,'')<>'' THEN
  label:='Refund processed';
 ELSE
  RETURN NEW;
 END IF;
 SELECT o.order_number,o.user_id,COALESCE(oi.product_name,'your order') INTO number,buyer,item
 FROM orders o LEFT JOIN order_items oi ON oi.id=NEW.order_item_id WHERE o.id=NEW.order_id;
 amount:='₹'||to_char((NEW.amount_cents+NEW.shipping_cents)::numeric/100,'FM999999999999990.00');
 message:=CASE WHEN label='Refund pending'
   THEN format('A refund of %s for %s in order %s is awaiting approval.',amount,item,number)
   ELSE format('Your payment provider confirmed a refund of %s for %s in order %s. Your bank may take additional time to post it.',amount,item,number)
 END;
 key:='marketplace-refund:'||NEW.id||':'||NEW.status;
 INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
 VALUES(buyer,'order',label,message,'/orders/'||number||'?tab=returns',key)
 ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 IF label='Refund processed' THEN
  INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
  SELECT s.owner_user_id,'order',label,'The payment provider confirmed the refund. Open your seller order for details.',
    '/seller-admin/orders/'||number||'?tab=returns',key FROM sellers s WHERE s.id=NEW.seller_id AND s.owner_user_id IS NOT NULL
  ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER marketplace_refund_notice AFTER INSERT OR UPDATE OF status ON marketplace_refunds
FOR EACH ROW EXECUTE FUNCTION marketplace_refund_notice();

CREATE OR REPLACE FUNCTION marketplace_package_sms_notice() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE label text; number text; key text;
BEGIN
 IF OLD.status=NEW.status THEN RETURN NEW; END IF;
 label:=CASE NEW.status WHEN 'picked_up' THEN 'Package shipped' WHEN 'ndr' THEN 'Delivery needs attention' WHEN 'rto' THEN 'Package returning to seller' WHEN 'delivered' THEN CASE WHEN NEW.delivery_method='manual' THEN 'Package delivered' END END;
 IF label IS NULL THEN RETURN NEW; END IF;
 SELECT order_number INTO number FROM orders WHERE id=NEW.order_id;
 key:='sms-package:'||NEW.id||':'||NEW.status;
 INSERT INTO notifications(user_id,kind,title,body,link_url,source_key,marketplace_package_id)
 SELECT user_id,'order',label,'Open the package for verified tracking details.','/orders/'||number||'/fulfillment',key,NEW.id FROM orders WHERE id=NEW.order_id
 ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 IF NEW.status IN ('ndr','rto') THEN
  INSERT INTO notifications(user_id,kind,title,body,link_url,source_key,marketplace_package_id)
  SELECT owner_user_id,'order',label,'Open the seller package to review the delivery exception.','/seller-admin/orders/'||number||'/fulfillment',key,NEW.id FROM sellers WHERE id=NEW.seller_id
  ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
