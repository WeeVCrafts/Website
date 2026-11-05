-- Manual bank transfers remain external. Store private evidence references,
-- immutable recipient versions and independent control identities here.
CREATE TABLE seller_payout_payee_versions (
    id BIGSERIAL PRIMARY KEY,
    seller_id BIGINT NOT NULL REFERENCES sellers(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL CHECK (version > 0),
    account_holder TEXT NOT NULL CHECK (length(account_holder) BETWEEN 2 AND 160),
    bank_name TEXT NOT NULL CHECK (length(bank_name) BETWEEN 2 AND 120),
    account_last4 TEXT NOT NULL CHECK (account_last4 ~ '^[0-9]{4}$'),
    routing_code TEXT NOT NULL CHECK (routing_code ~ '^[A-Z]{4}0[A-Z0-9]{6}$'),
    verification_note TEXT NOT NULL CHECK (length(verification_note) BETWEEN 4 AND 1000),
    evidence_key TEXT NOT NULL UNIQUE,
    evidence_type TEXT NOT NULL CHECK (evidence_type IN ('application/pdf','image/png','image/jpeg')),
    evidence_bytes BIGINT NOT NULL CHECK (evidence_bytes BETWEEN 1 AND 5242880),
    evidence_sha256 TEXT NOT NULL CHECK (length(evidence_sha256)=64),
    prepared_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    verified_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (seller_id,version),
    UNIQUE (seller_id,id),
    CHECK ((verified_by IS NULL AND verified_at IS NULL) OR (verified_by IS NOT NULL AND verified_at IS NOT NULL AND verified_by<>prepared_by))
);
ALTER TABLE seller_settlements
    ADD COLUMN manual_controls_version INTEGER NOT NULL DEFAULT 0 CHECK (manual_controls_version IN (0,1)),
    ADD COLUMN prepared_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN approved_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN approved_at TIMESTAMPTZ,
    ADD COLUMN paid_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN paid_at TIMESTAMPTZ,
    ADD COLUMN payee_version_id BIGINT,
    ADD COLUMN revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
    ADD COLUMN approved_ledger_hash TEXT,
    ADD COLUMN transfer_amount_cents BIGINT,
    ADD COLUMN transfer_date DATE,
    ADD COLUMN bank_reference TEXT,
    ADD COLUMN transfer_evidence_key TEXT UNIQUE,
    ADD COLUMN transfer_evidence_type TEXT,
    ADD COLUMN transfer_evidence_bytes BIGINT,
    ADD COLUMN transfer_evidence_sha256 TEXT,
    ADD COLUMN transfer_submitted_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN transfer_submitted_at TIMESTAMPTZ,
    ADD CONSTRAINT settlement_payee_owner FOREIGN KEY (seller_id,payee_version_id) REFERENCES seller_payout_payee_versions(seller_id,id) ON DELETE RESTRICT,
    ADD CONSTRAINT settlement_distinct_approver CHECK (approved_by IS NULL OR approved_by<>prepared_by),
    ADD CONSTRAINT settlement_manual_approval CHECK (manual_controls_version=0 OR status NOT IN ('approved','paid') OR (prepared_by IS NOT NULL AND approved_by IS NOT NULL AND payee_version_id IS NOT NULL AND approved_at IS NOT NULL AND approved_ledger_hash IS NOT NULL AND length(approved_ledger_hash)=64)),
    ADD CONSTRAINT settlement_manual_transfer CHECK (transfer_evidence_key IS NULL OR (transfer_amount_cents IS NOT NULL AND transfer_amount_cents=net_cents AND transfer_amount_cents>0 AND transfer_date IS NOT NULL AND bank_reference IS NOT NULL AND length(bank_reference) BETWEEN 4 AND 120 AND transfer_evidence_type IS NOT NULL AND transfer_evidence_type IN ('application/pdf','image/png','image/jpeg') AND transfer_evidence_bytes IS NOT NULL AND transfer_evidence_bytes BETWEEN 1 AND 5242880 AND transfer_evidence_sha256 IS NOT NULL AND length(transfer_evidence_sha256)=64 AND transfer_submitted_by IS NOT NULL AND transfer_submitted_by=prepared_by AND transfer_submitted_at IS NOT NULL)),
    ADD CONSTRAINT settlement_manual_paid CHECK (manual_controls_version=0 OR status<>'paid' OR (transfer_evidence_key IS NOT NULL AND paid_by IS NOT NULL AND paid_by=approved_by AND paid_by<>prepared_by AND paid_at IS NOT NULL));
CREATE UNIQUE INDEX seller_settlements_bank_reference_unique ON seller_settlements(lower(bank_reference)) WHERE bank_reference IS NOT NULL;

-- Old paid records stay unchanged and are displayed as needing reconciliation.
-- Older application writers must not bypass the new controls after migration.
CREATE FUNCTION enforce_manual_settlement_payment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.status='paid' AND (TG_OP='INSERT' OR OLD.status<>'paid') AND NEW.manual_controls_version<>1 THEN
        RAISE EXCEPTION 'manual transfer controls and evidence are required before paid status';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER seller_settlement_payment_controls BEFORE INSERT OR UPDATE ON seller_settlements FOR EACH ROW EXECUTE FUNCTION enforce_manual_settlement_payment();

CREATE FUNCTION protect_payout_payee_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.seller_id,NEW.version,NEW.account_holder,NEW.bank_name,NEW.account_last4,NEW.routing_code,NEW.verification_note,NEW.evidence_key,NEW.evidence_type,NEW.evidence_bytes,NEW.evidence_sha256,NEW.prepared_by,NEW.created_at)
       IS DISTINCT FROM ROW(OLD.seller_id,OLD.version,OLD.account_holder,OLD.bank_name,OLD.account_last4,OLD.routing_code,OLD.verification_note,OLD.evidence_key,OLD.evidence_type,OLD.evidence_bytes,OLD.evidence_sha256,OLD.prepared_by,OLD.created_at)
       OR (OLD.verified_by IS NOT NULL AND ROW(NEW.verified_by,NEW.verified_at) IS DISTINCT FROM ROW(OLD.verified_by,OLD.verified_at)) THEN
        RAISE EXCEPTION 'Recipient versions are immutable; prepare a new version and obtain new approval';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER seller_payout_payee_immutable BEFORE UPDATE ON seller_payout_payee_versions FOR EACH ROW EXECUTE FUNCTION protect_payout_payee_version();
