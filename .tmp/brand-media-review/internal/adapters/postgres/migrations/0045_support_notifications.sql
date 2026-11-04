ALTER TABLE notifications ADD COLUMN link_url TEXT NOT NULL DEFAULT '';
ALTER TABLE notifications ADD COLUMN source_key TEXT;
CREATE UNIQUE INDEX notifications_user_source_key ON notifications(user_id,source_key) WHERE source_key IS NOT NULL;

-- Notification text never copies private messages. Conversation access remains
-- authorized by its existing handler; destinations follow the recipient's role.
CREATE FUNCTION support_notification_link(account_id BIGINT, case_id BIGINT) RETURNS TEXT LANGUAGE sql STABLE AS $$
 SELECT CASE role_slug
 WHEN 'seller' THEN '/seller-admin/support?ticket_id='||case_id||'#ticket-'||case_id
 WHEN 'admin' THEN '/admin/support?ticket_id='||case_id
 WHEN 'super_admin' THEN '/super-admin/support?ticket_id='||case_id
 ELSE '/account/support?ticket_id='||case_id||'#ticket-'||case_id END
 FROM user_roles WHERE user_id=account_id
$$;

CREATE FUNCTION notify_support_activity() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE target BIGINT; case_id BIGINT; author BIGINT; event_key TEXT; heading TEXT; detail TEXT;
BEGIN
 IF TG_TABLE_NAME='support_tickets' THEN
  case_id:=NEW.id; author:=NEW.user_id; event_key:='case:'||NEW.id;
  heading:='New support request'; detail:='A new case needs a response.';
 ELSE
  case_id:=NEW.ticket_id; author:=NEW.author_id; event_key:='reply:'||NEW.id;
  heading:='New support reply'; detail:='A reply is ready in your support conversation.';
 END IF;
 FOR target IN
  SELECT u.id FROM users u JOIN user_roles r ON r.user_id=u.id
  WHERE u.status='active' AND u.id<>author AND
   ((TG_TABLE_NAME='support_tickets' OR NOT EXISTS(SELECT 1 FROM user_roles WHERE user_id=author AND role_slug IN ('admin','super_admin')))
     AND r.role_slug IN ('admin','super_admin')
    OR TG_TABLE_NAME='support_ticket_messages' AND EXISTS(SELECT 1 FROM user_roles WHERE user_id=author AND role_slug IN ('admin','super_admin'))
     AND u.id=(SELECT user_id FROM support_tickets WHERE id=case_id))
 LOOP
  INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
  VALUES(target,'support',heading,detail,support_notification_link(target,case_id),event_key)
  ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 END LOOP;
 RETURN NEW;
END $$;
CREATE TRIGGER support_case_notification AFTER INSERT ON support_tickets FOR EACH ROW EXECUTE FUNCTION notify_support_activity();
CREATE TRIGGER support_reply_notification AFTER INSERT ON support_ticket_messages FOR EACH ROW EXECUTE FUNCTION notify_support_activity();

CREATE FUNCTION notify_seller_coordination() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE request seller_confirmations%ROWTYPE; target BIGINT; author BIGINT; event_key TEXT; heading TEXT;
BEGIN
 IF TG_TABLE_NAME='seller_confirmation_messages' THEN
  SELECT * INTO request FROM seller_confirmations WHERE id=NEW.confirmation_id;
  author:=NEW.author_id; event_key:='coordination-reply:'||NEW.id; heading:='Seller confirmation received';
 ELSE
  request:=NEW; author:=NEW.created_by;
  IF TG_OP='UPDATE' AND NEW.status IS NOT DISTINCT FROM OLD.status THEN RETURN NEW; END IF;
  IF TG_OP='UPDATE' AND NEW.status<>'closed' THEN RETURN NEW; END IF;
  event_key:='coordination:'||NEW.id||':'||NEW.status;
  heading:=CASE WHEN NEW.status='closed' THEN 'Seller request resolved' ELSE 'New seller confirmation request' END;
 END IF;
 FOR target IN
  SELECT u.id FROM users u JOIN user_roles r ON r.user_id=u.id WHERE u.status='active' AND u.id<>author AND
   (TG_TABLE_NAME='seller_confirmation_messages' AND r.role_slug IN ('admin','super_admin')
    OR TG_TABLE_NAME='seller_confirmations' AND r.role_slug='seller' AND u.id=(SELECT owner_user_id FROM sellers WHERE id=request.seller_id))
 LOOP
  INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
  VALUES(target,'seller_coordination',heading,'Open the private order conversation to review the update.',
   CASE WHEN TG_TABLE_NAME='seller_confirmations' THEN '/seller-admin/support?ticket_id='||request.ticket_id||'#confirmation-'||request.id
    ELSE support_notification_link(target,request.ticket_id)||'#confirmation-'||request.id END,event_key)
  ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
 END LOOP;
 RETURN NEW;
END $$;
CREATE TRIGGER seller_request_notification AFTER INSERT OR UPDATE OF status ON seller_confirmations FOR EACH ROW EXECUTE FUNCTION notify_seller_coordination();
CREATE TRIGGER seller_response_notification AFTER INSERT ON seller_confirmation_messages FOR EACH ROW EXECUTE FUNCTION notify_seller_coordination();

-- Surface each account's most recent unanswered awareness gap without flooding
-- its inbox with historical messages. Existing conversations are unchanged.
INSERT INTO notifications(user_id,kind,title,body,link_url,source_key)
 SELECT t.user_id,'support','Support reply available','Open your conversation to review the latest company response.',support_notification_link(t.user_id,t.id),'reply:'||m.id
 FROM support_tickets t JOIN users u ON u.id=t.user_id AND u.status='active'
 JOIN LATERAL(SELECT id,author_role,created_at FROM support_ticket_messages WHERE ticket_id=t.id ORDER BY created_at DESC,id DESC LIMIT 1)m ON true
 WHERE m.author_role IN ('admin','super_admin','support_agent') AND m.created_at>NOW()-INTERVAL '30 days'
 ON CONFLICT(user_id,source_key) WHERE source_key IS NOT NULL DO NOTHING;
