CREATE FUNCTION marketplace_queue_notification_email() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.kind='order' AND NEW.title IN ('New paid order','Package out for delivery','Package delivered','Refund processed') THEN
        PERFORM marketplace_enqueue('transactional_email',NEW.id,'notification-email:'||NEW.id);
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER marketplace_notification_email AFTER INSERT ON notifications
FOR EACH ROW EXECUTE FUNCTION marketplace_queue_notification_email();

CREATE FUNCTION marketplace_notify_processed_refund() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status<>'processed' AND NEW.status='processed' AND COALESCE(NEW.provider_refund_id,'')<>'' THEN
        INSERT INTO notifications(user_id,kind,title,body,link_url)
        SELECT o.user_id,'order','Refund processed','Your refund for order '||o.order_number||
            ' was confirmed. Your bank or payment provider may take additional time to post it.',
            '/orders/'||o.order_number||'/fulfillment'
        FROM orders o WHERE o.id=NEW.order_id;
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER marketplace_processed_refund_notification AFTER UPDATE OF status ON marketplace_refunds
FOR EACH ROW EXECUTE FUNCTION marketplace_notify_processed_refund();
