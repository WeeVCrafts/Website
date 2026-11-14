ALTER TABLE shiprocket_shipments
    ADD COLUMN manifest_url TEXT NOT NULL DEFAULT '',
    DROP CONSTRAINT shiprocket_shipments_pending_action_check,
    ADD CONSTRAINT shiprocket_shipments_pending_action_check
        CHECK (pending_action IN ('', 'create', 'assign', 'pickup', 'label', 'manifest'));
