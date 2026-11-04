-- One durable customer confirmation event per captured order, including retries.
CREATE UNIQUE INDEX IF NOT EXISTS outbox_order_paid_once
ON outbox_events(event_type,aggregate_id) WHERE event_type='OrderPaid';
