CREATE TABLE seller_confirmations (
 id BIGSERIAL PRIMARY KEY,
 ticket_id BIGINT NOT NULL REFERENCES support_tickets(id) ON DELETE RESTRICT,
 order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
 seller_id BIGINT NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
 created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 request_key TEXT NOT NULL CHECK(length(request_key) BETWEEN 16 AND 200),
 question TEXT NOT NULL CHECK(length(btrim(question)) BETWEEN 1 AND 4000),
 status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','answered','closed')),
 close_reason TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(created_by,request_key)
);
CREATE INDEX seller_confirmations_seller_queue ON seller_confirmations(seller_id,status,updated_at DESC,id DESC);
CREATE INDEX seller_confirmations_case_queue ON seller_confirmations(ticket_id,updated_at DESC,id DESC);
CREATE TABLE seller_confirmation_messages (
 id BIGSERIAL PRIMARY KEY,
 confirmation_id BIGINT NOT NULL REFERENCES seller_confirmations(id) ON DELETE RESTRICT,
 author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 request_key TEXT NOT NULL CHECK(length(request_key) BETWEEN 16 AND 200),
 body TEXT NOT NULL CHECK(length(btrim(body)) BETWEEN 1 AND 4000),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(confirmation_id,author_id,request_key)
);
