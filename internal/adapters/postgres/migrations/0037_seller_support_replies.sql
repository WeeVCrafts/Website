ALTER TABLE support_ticket_messages DROP CONSTRAINT IF EXISTS support_ticket_messages_author_role_check;
ALTER TABLE support_ticket_messages ADD CONSTRAINT support_ticket_messages_author_role_check CHECK (author_role IN ('support_agent', 'customer', 'seller_owner'));
