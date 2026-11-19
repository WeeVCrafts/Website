ALTER TABLE marketplace_setup_submissions DROP CONSTRAINT marketplace_setup_submissions_status_check;
ALTER TABLE marketplace_setup_submissions ADD CHECK(status IN ('draft','submitted','changes_requested','approved','superseded'));
ALTER TABLE order_email_deliveries ADD COLUMN workflow_reference text NOT NULL DEFAULT '';

CREATE OR REPLACE FUNCTION marketplace_queue_notification_email() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.kind='order' AND NEW.title IN ('New paid order','Package out for delivery','Package delivered','Refund processed'))
 OR (NEW.kind='account' AND NEW.title IN ('Seller details submitted','Seller details approved','Seller details changes requested')) THEN
  PERFORM marketplace_enqueue('transactional_email',NEW.id,'notification-email:'||NEW.id);
 END IF;
 RETURN NEW;
END $$;

CREATE FUNCTION commerce_invoice_snapshot_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.snapshot IS DISTINCT FROM OLD.snapshot OR NEW.profile_revision<>OLD.profile_revision OR NEW.order_id<>OLD.order_id OR NEW.seller_id<>OLD.seller_id THEN
  RAISE EXCEPTION 'Order invoice identity is immutable';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER commerce_invoice_snapshot_immutable BEFORE UPDATE ON order_seller_invoice_snapshots FOR EACH ROW EXECUTE FUNCTION commerce_invoice_snapshot_immutable();
