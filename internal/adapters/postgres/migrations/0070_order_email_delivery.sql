CREATE TABLE order_email_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    send_key text UNIQUE NOT NULL,
    order_number text NOT NULL,
    kind text NOT NULL,
    provider_id uuid UNIQUE,
    state text NOT NULL DEFAULT 'submitting' CHECK (state IN (
        'submitting','accepted','delayed','delivered','bounced','failed','suppressed','complained','rejected','uncertain'
    )),
    started_at timestamptz NOT NULL DEFAULT now(),
    state_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_email_deliveries_started ON order_email_deliveries (started_at DESC, id DESC);
CREATE INDEX order_email_deliveries_order ON order_email_deliveries (order_number, started_at DESC, id DESC);

CREATE TABLE order_email_events (
    event_key text PRIMARY KEY,
    delivery_id uuid NOT NULL REFERENCES order_email_deliveries(id) ON DELETE CASCADE,
    state text NOT NULL CHECK (state IN ('accepted','delayed','delivered','bounced','failed','suppressed','complained')),
    occurred_at timestamptz NOT NULL
);
CREATE INDEX order_email_events_delivery ON order_email_events (delivery_id);
