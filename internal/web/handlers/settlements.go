package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

const maxSettlementEvidence = 5 << 20

var payoutRoutingCode = regexp.MustCompile(`^[A-Z]{4}0[A-Z0-9]{6}$`)
var payoutAccountSuffix = regexp.MustCompile(`^[0-9]{4}$`)

func (h *LivePortalHandler) SetSettlementStorage(storage ports.ObjectStorage) {
	h.settlementStorage = storage
}

func isSettlementMutationPath(path string) bool {
	for _, root := range []string{"/admin/finance/settlements/", "/super-admin/finance/settlements/"} {
		if strings.HasPrefix(path, root) {
			switch strings.TrimPrefix(path, root) {
			case "create", "prepare", "status", "transfer":
				return true
			}
		}
	}
	return false
}

func (h *LivePortalHandler) adminSettlementMutation(w http.ResponseWriter, r *http.Request) {
	root, role := "/admin/finance", "admin"
	if strings.HasPrefix(r.URL.Path, "/super-admin/") {
		root, role = "/super-admin/finance", "super_admin"
	}
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return
	}
	if r.Method != http.MethodPost || !h.hasRole(r, session.UserID, role) {
		denyPortalAccess(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSettlementEvidence+(1<<20))
	var err error
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		err = r.ParseMultipartForm(maxSettlementEvidence + (1 << 20))
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
	} else {
		err = r.ParseForm()
	}
	if err != nil {
		http.Error(w, "Choose one PDF, PNG or JPEG up to 5 MiB and try again.", http.StatusBadRequest)
		return
	}
	if session.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(r.Form.Get("csrf_token"))) != 1 {
		denyPortalAccess(w, r)
		return
	}
	action := strings.TrimPrefix(r.URL.Path, root+"/settlements/")
	if action == "create" {
		err = h.createSettlementBatches(r, session.UserID)
	} else {
		id, parseErr := strconv.ParseInt(strings.TrimSpace(r.Form.Get("settlement_id")), 10, 64)
		if parseErr != nil || id <= 0 {
			err = errors.New("invalid settlement")
		} else {
			switch action {
			case "prepare":
				err = h.prepareSettlement(r, id, session.UserID)
			case "transfer":
				err = h.recordSettlementTransfer(r, id, session.UserID)
			case "status":
				err = h.updateSettlementStatus(r, id, r.Form.Get("status"), session.UserID)
			default:
				http.NotFound(w, r)
				return
			}
		}
	}
	if err != nil {
		// SQL/storage diagnostics may contain private banking or object details.
		message := "Settlement could not be saved. Reload and verify its state, independent approver, recipient, amount and evidence."
		var validation *settlementValidationError
		if errors.As(err, &validation) {
			message = validation.Error()
		}
		http.Error(w, message, http.StatusConflict)
		return
	}
	http.Redirect(w, r, root+"?workspace=settlements&notice=Manual+settlement+workflow+saved", http.StatusSeeOther)
}

type settlementValidationError struct{ message string }

func (e *settlementValidationError) Error() string { return e.message }
func settlementInvalid(message string) error       { return &settlementValidationError{message} }

// Match identity administration's lock before reading the acting account, then
// use the existing finance lock shared by captures, review holds and the ledger.
func beginSettlement(ctx context.Context, h *LivePortalHandler, actorID int64) (pgx.Tx, error) {
	if h.pool == nil {
		return nil, errors.New("database unavailable")
	}
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(5165538220001)`); err == nil {
		var allowed bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug IN ('admin','super_admin'))`, actorID).Scan(&allowed)
		if err == nil && !allowed {
			err = ports.ErrForbidden
		}
	}
	if err == nil {
		_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(4856203)`)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

type settlementBatch struct {
	sellerID, gross, commission, net int64
	start, end                       time.Time
}

func (h *LivePortalHandler) createSettlementBatches(r *http.Request, actorID int64) error {
	tx, err := beginSettlement(r.Context(), h, actorID)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), `SELECT seller_id,SUM(gross_cents)::BIGINT,SUM(commission_cents)::BIGINT,SUM(net_cents)::BIGINT,MIN(created_at)::date,MAX(created_at)::date FROM seller_ledger sl WHERE status='open' AND settlement_id IS NULL AND NOT EXISTS(SELECT 1 FROM orders o WHERE o.id=sl.order_id AND o.payment_review_required) GROUP BY seller_id HAVING SUM(net_cents)>0 ORDER BY seller_id`)
	if err != nil {
		return err
	}
	batches := []settlementBatch{}
	for rows.Next() {
		var b settlementBatch
		if err = rows.Scan(&b.sellerID, &b.gross, &b.commission, &b.net, &b.start, &b.end); err != nil {
			rows.Close()
			return err
		}
		batches = append(batches, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, b := range batches {
		reference := fmt.Sprintf("SETTLE-%d-%s", b.sellerID, time.Now().UTC().Format("20060102T150405.000000000Z"))
		var id int64
		if err = tx.QueryRow(r.Context(), `INSERT INTO seller_settlements(seller_id,period_start,period_end,gross_cents,commission_cents,net_cents,status,reference,manual_controls_version,prepared_by) VALUES($1,$2,$3,$4,$5,$6,'pending',$7,1,$8) RETURNING id`, b.sellerID, b.start, b.end, b.gross, b.commission, b.net, reference, actorID).Scan(&id); err != nil {
			return err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE seller_ledger sl SET settlement_id=$1 WHERE seller_id=$2 AND status='open' AND settlement_id IS NULL AND NOT EXISTS(SELECT 1 FROM orders o WHERE o.id=sl.order_id AND o.payment_review_required)`, id, b.sellerID); err != nil {
			return err
		}
		if err = auditSettlement(r, tx, actorID, id, "batch_created"); err != nil {
			return err
		}
	}
	return tx.Commit(r.Context())
}

type lockedSettlement struct {
	id, sellerID, preparedBy, approvedBy, payeeID, revision, controls int64
	gross, commission, net                                            int64
	status, ledgerHash, transferKey                                   string
	approvedAt                                                        *time.Time
}

func lockSettlement(r *http.Request, tx pgx.Tx, id int64) (lockedSettlement, error) {
	s := lockedSettlement{id: id}
	err := tx.QueryRow(r.Context(), `SELECT seller_id,status,COALESCE(prepared_by,0),COALESCE(approved_by,0),COALESCE(payee_version_id,0),revision,manual_controls_version,gross_cents,commission_cents,net_cents,COALESCE(approved_ledger_hash,''),COALESCE(transfer_evidence_key,''),approved_at FROM seller_settlements WHERE id=$1 FOR UPDATE`, id).Scan(&s.sellerID, &s.status, &s.preparedBy, &s.approvedBy, &s.payeeID, &s.revision, &s.controls, &s.gross, &s.commission, &s.net, &s.ledgerHash, &s.transferKey, &s.approvedAt)
	if err != nil {
		return s, err
	}
	revision, err := strconv.ParseInt(r.Form.Get("revision"), 10, 64)
	if err != nil || revision != s.revision {
		return s, settlementInvalid("Settlement changed. Reload before making another decision.")
	}
	return s, nil
}

func settlementLedger(r *http.Request, tx pgx.Tx, s lockedSettlement) (string, error) {
	var gross, commission, net, count int64
	var contents string
	var blocked bool
	err := tx.QueryRow(r.Context(), `SELECT COUNT(*),COALESCE(SUM(sl.gross_cents),0)::bigint,COALESCE(SUM(sl.commission_cents),0)::bigint,COALESCE(SUM(sl.net_cents),0)::bigint,
 COALESCE(string_agg(concat_ws(':',sl.id,sl.seller_id,COALESCE(sl.order_id,0),sl.entry_type,sl.gross_cents,sl.commission_cents,sl.net_cents,sl.status),'|' ORDER BY sl.id),''),
 COALESCE(bool_or(sl.status<>'open' OR sl.seller_id<>$2 OR COALESCE(o.payment_review_required,false) OR (sl.entry_type='sale' AND o.status='refunded' AND NOT EXISTS(SELECT 1 FROM seller_ledger refund WHERE refund.order_id=o.id AND refund.seller_id=sl.seller_id AND refund.entry_type='refund' AND refund.settlement_id=$1)) OR EXISTS(SELECT 1 FROM payment_refunds pr WHERE pr.order_id=sl.order_id AND pr.status='pending')),false)
 FROM seller_ledger sl LEFT JOIN orders o ON o.id=sl.order_id WHERE sl.settlement_id=$1`, s.id, s.sellerID).Scan(&count, &gross, &commission, &net, &contents, &blocked)
	if err != nil {
		return "", err
	}
	if blocked || count == 0 || s.net <= 0 || gross != s.gross || commission != s.commission || net != s.net {
		return "", settlementInvalid("Payment hold, refund or ledger changes block this batch. Reconcile it before continuing.")
	}
	hash := sha256.Sum256([]byte(contents))
	return hex.EncodeToString(hash[:]), nil
}

func auditSettlement(r *http.Request, tx pgx.Tx, actor, id int64, action string) error {
	_, err := tx.Exec(r.Context(), `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) SELECT $1,$2,'seller_settlement',id::text,$4,jsonb_build_object('status',status,'revision',revision,'payee_version_id',payee_version_id,'prepared_by',prepared_by,'approved_by',approved_by) FROM seller_settlements WHERE id=$3`, fmt.Sprint(actor), "settlement."+action, id, r.Header.Get("X-Request-ID"))
	return err
}

type settlementEvidence struct {
	key, contentType, sha256 string
	size                     int64
}

type settlementEvidenceQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func settlementEvidenceRecord(ctx context.Context, query settlementEvidenceQuerier, id int64, kind string) (settlementEvidence, error) {
	var e settlementEvidence
	var err error
	if kind == "payee" {
		err = query.QueryRow(ctx, `SELECT p.evidence_key,p.evidence_type,p.evidence_bytes,p.evidence_sha256 FROM seller_settlements ss JOIN seller_payout_payee_versions p ON p.id=ss.payee_version_id WHERE ss.id=$1`, id).Scan(&e.key, &e.contentType, &e.size, &e.sha256)
	} else if kind == "transfer" {
		err = query.QueryRow(ctx, `SELECT transfer_evidence_key,transfer_evidence_type,transfer_evidence_bytes,transfer_evidence_sha256 FROM seller_settlements WHERE id=$1 AND transfer_evidence_key IS NOT NULL`, id).Scan(&e.key, &e.contentType, &e.size, &e.sha256)
	} else {
		err = ports.ErrNotFound
	}
	return e, err
}

func (h *LivePortalHandler) settlementEvidenceBytes(ctx context.Context, e settlementEvidence) ([]byte, error) {
	storage, ok := h.settlementStorage.(ports.ObjectStorageContentReader)
	if !ok || e.size <= 0 || e.size > maxSettlementEvidence {
		return nil, settlementInvalid("Private evidence is unavailable. Restore and verify the document before continuing.")
	}
	body, info, err := storage.OpenObject(ctx, e.key)
	if err != nil {
		return nil, settlementInvalid("Private evidence is unavailable. Restore and verify the document before continuing.")
	}
	defer body.Close()
	if info.ByteSize != e.size || info.ContentType != e.contentType {
		return nil, settlementInvalid("Private evidence does not match its saved record. Reconcile before continuing.")
	}
	contents, err := io.ReadAll(io.LimitReader(body, maxSettlementEvidence+1))
	digest := sha256.Sum256(contents)
	if err != nil || int64(len(contents)) != e.size || hex.EncodeToString(digest[:]) != e.sha256 || http.DetectContentType(contents) != e.contentType {
		return nil, settlementInvalid("Private evidence does not match its saved record. Reconcile before continuing.")
	}
	return contents, nil
}

func (h *LivePortalHandler) verifySettlementEvidence(ctx context.Context, tx pgx.Tx, id int64, kind string) error {
	e, err := settlementEvidenceRecord(ctx, tx, id, kind)
	if err != nil {
		return settlementInvalid("Required private evidence is missing. Reconcile before continuing.")
	}
	_, err = h.settlementEvidenceBytes(ctx, e)
	return err
}

func readSettlementEvidence(header *multipart.FileHeader) ([]byte, string, error) {
	if header == nil || header.Size <= 0 || header.Size > maxSettlementEvidence {
		return nil, "", settlementInvalid("Choose one PDF, PNG or JPEG up to 5 MiB.")
	}
	file, err := header.Open()
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxSettlementEvidence+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) == 0 || len(body) > maxSettlementEvidence {
		return nil, "", settlementInvalid("Choose one PDF, PNG or JPEG up to 5 MiB.")
	}
	contentType := http.DetectContentType(body)
	if contentType != "application/pdf" && contentType != "image/png" && contentType != "image/jpeg" {
		return nil, "", settlementInvalid("Evidence must contain PDF, PNG or JPEG bytes.")
	}
	return body, contentType, nil
}

func (h *LivePortalHandler) storeSettlementEvidence(r *http.Request, id int64, field string) (settlementEvidence, error) {
	var e settlementEvidence
	if r.MultipartForm == nil || len(r.MultipartForm.File[field]) != 1 {
		return e, settlementInvalid("Select the required private evidence file.")
	}
	body, kind, err := readSettlementEvidence(r.MultipartForm.File[field][0])
	if err != nil {
		return e, err
	}
	writer, ok := h.settlementStorage.(ports.ObjectStorageWriter)
	if !ok {
		return e, settlementInvalid("Private finance storage is unavailable. No settlement change was saved.")
	}
	var token [24]byte
	if _, err = rand.Read(token[:]); err != nil {
		return e, err
	}
	e.key = fmt.Sprintf("finance/settlements/%d/%s", id, hex.EncodeToString(token[:]))
	e.contentType, e.size = kind, int64(len(body))
	digest := sha256.Sum256(body)
	e.sha256 = hex.EncodeToString(digest[:])
	if err = writer.PutObject(r.Context(), e.key, kind, e.size, bytes.NewReader(body)); err != nil {
		h.discardSettlementEvidence(e)
		return settlementEvidence{}, err
	}
	return e, nil
}

func (h *LivePortalHandler) discardSettlementEvidence(e settlementEvidence) {
	if e.key == "" {
		return
	}
	if storage, ok := h.settlementStorage.(ports.ObjectStorageReader); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = storage.DeleteObject(ctx, e.key)
	}
}

func (h *LivePortalHandler) prepareSettlement(r *http.Request, id, actor int64) error {
	name, bank := strings.TrimSpace(r.Form.Get("payee_name")), strings.TrimSpace(r.Form.Get("bank_name"))
	last4, routing := strings.TrimSpace(r.Form.Get("account_last4")), strings.ToUpper(strings.TrimSpace(r.Form.Get("routing_code")))
	note := strings.TrimSpace(r.Form.Get("verification_note"))
	if len(name) < 2 || len(name) > 160 || len(bank) < 2 || len(bank) > 120 || !payoutAccountSuffix.MatchString(last4) || !payoutRoutingCode.MatchString(routing) || len(note) < 4 || len(note) > 1000 {
		return settlementInvalid("Enter account holder, bank, last four account digits, valid IFSC and verification instructions.")
	}
	tx, err := beginSettlement(r.Context(), h, actor)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	s, err := lockSettlement(r, tx, id)
	if err != nil {
		return err
	}
	if (s.status != "pending" && s.status != "approved") || s.transferKey != "" || (s.preparedBy != 0 && s.preparedBy != actor) {
		return settlementInvalid("Only this batch's preparer can prepare it before transfer evidence is submitted.")
	}
	if _, err = settlementLedger(r, tx, s); err != nil {
		return err
	}
	e, err := h.storeSettlementEvidence(r, id, "payee_evidence")
	if err != nil {
		return err
	}
	commitAttempted := false
	defer func() {
		if !commitAttempted {
			h.discardSettlementEvidence(e)
		}
	}()
	var payeeID int64
	err = tx.QueryRow(r.Context(), `INSERT INTO seller_payout_payee_versions(seller_id,version,account_holder,bank_name,account_last4,routing_code,verification_note,evidence_key,evidence_type,evidence_bytes,evidence_sha256,prepared_by) SELECT $1,COALESCE(MAX(version),0)+1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11 FROM seller_payout_payee_versions WHERE seller_id=$1 RETURNING id`, s.sellerID, name, bank, last4, routing, note, e.key, e.contentType, e.size, e.sha256, actor).Scan(&payeeID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(r.Context(), `UPDATE seller_settlements SET manual_controls_version=1,prepared_by=$2,payee_version_id=$3,status='pending',approved_by=NULL,approved_at=NULL,approved_ledger_hash=NULL,revision=revision+1,updated_at=NOW() WHERE id=$1`, id, actor, payeeID)
	if err != nil {
		return err
	}
	if err = auditSettlement(r, tx, actor, id, "prepared"); err != nil {
		return err
	}
	commitAttempted = true
	return tx.Commit(r.Context())
}

func (h *LivePortalHandler) updateSettlementStatus(r *http.Request, id int64, status string, actor int64) error {
	if status != "approved" && status != "paid" && status != "failed" {
		return settlementInvalid("Invalid settlement decision.")
	}
	tx, err := beginSettlement(r.Context(), h, actor)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	s, err := lockSettlement(r, tx, id)
	if err != nil {
		return err
	}
	if s.status == "paid" || s.status == "failed" {
		return settlementInvalid("This settlement is closed.")
	}
	if status == "failed" {
		if s.transferKey != "" {
			return settlementInvalid("Transfer evidence already exists. Reconcile the transfer; do not release its ledger into another batch.")
		}
		if _, err = tx.Exec(r.Context(), `UPDATE seller_settlements SET status='failed',revision=revision+1,updated_at=NOW() WHERE id=$1`, id); err != nil {
			return err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE seller_ledger SET settlement_id=NULL WHERE settlement_id=$1 AND status='open'`, id); err != nil {
			return err
		}
	} else {
		if s.controls != 1 || s.preparedBy == 0 || s.payeeID == 0 || s.preparedBy == actor {
			return settlementInvalid("A different operator must verify the prepared recipient and approve this batch.")
		}
		var latest bool
		if err = tx.QueryRow(r.Context(), `SELECT id=(SELECT id FROM seller_payout_payee_versions WHERE seller_id=$2 ORDER BY version DESC LIMIT 1) FROM seller_payout_payee_versions WHERE id=$1 AND seller_id=$2`, s.payeeID, s.sellerID).Scan(&latest); err != nil {
			return err
		}
		if !latest {
			return settlementInvalid("Seller recipient changed. The preparer must prepare this batch again for independent approval.")
		}
		hash, err := settlementLedger(r, tx, s)
		if err != nil {
			return err
		}
		if status == "approved" {
			if s.status != "pending" || r.Form.Get("verified_payee") != "yes" {
				return settlementInvalid("Verify private recipient evidence and confirm the payee before approval.")
			}
			if err = h.verifySettlementEvidence(r.Context(), tx, id, "payee"); err != nil {
				return err
			}
			if _, err = tx.Exec(r.Context(), `UPDATE seller_payout_payee_versions SET verified_by=$2,verified_at=NOW() WHERE id=$1 AND verified_by IS NULL`, s.payeeID, actor); err != nil {
				return err
			}
			_, err = tx.Exec(r.Context(), `UPDATE seller_settlements SET status='approved',approved_by=$2,approved_at=NOW(),approved_ledger_hash=$3,revision=revision+1,updated_at=NOW() WHERE id=$1`, id, actor, hash)
		} else {
			if s.status != "approved" || s.approvedBy != actor || s.transferKey == "" || s.ledgerHash != hash || r.Form.Get("verified_transfer") != "yes" {
				return settlementInvalid("The independent approver must verify recorded transfer evidence and unchanged ledger before confirming paid.")
			}
			for _, kind := range []string{"payee", "transfer"} {
				if err = h.verifySettlementEvidence(r.Context(), tx, id, kind); err != nil {
					return err
				}
			}
			_, err = tx.Exec(r.Context(), `UPDATE seller_settlements SET status='paid',paid_by=$2,paid_at=NOW(),revision=revision+1,updated_at=NOW() WHERE id=$1`, id, actor)
			if err == nil {
				_, err = tx.Exec(r.Context(), `UPDATE seller_ledger SET status='settled' WHERE settlement_id=$1 AND status='open'`, id)
			}
		}
		if err != nil {
			return err
		}
	}
	if err = auditSettlement(r, tx, actor, id, status); err != nil {
		return err
	}
	return tx.Commit(r.Context())
}

func (h *LivePortalHandler) recordSettlementTransfer(r *http.Request, id, actor int64) error {
	amount, err := strconv.ParseInt(r.Form.Get("transfer_amount_cents"), 10, 64)
	if err != nil || amount <= 0 {
		return settlementInvalid("Enter the exact transfer amount in paise.")
	}
	date, err := time.Parse("2006-01-02", r.Form.Get("transfer_date"))
	reference := strings.TrimSpace(r.Form.Get("bank_reference"))
	if err != nil || len(reference) < 4 || len(reference) > 120 {
		return settlementInvalid("Enter transfer date and bank UTR/reference (4–120 characters).")
	}
	tx, err := beginSettlement(r.Context(), h, actor)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	s, err := lockSettlement(r, tx, id)
	if err != nil {
		return err
	}
	if s.status != "approved" || s.preparedBy != actor || s.approvedBy == actor || s.transferKey != "" || amount != s.net || s.approvedAt == nil {
		return settlementInvalid("Only the preparer can submit the exact approved transfer once.")
	}
	zone := time.FixedZone("IST", 19800)
	today, _ := time.Parse("2006-01-02", time.Now().In(zone).Format("2006-01-02"))
	approvedDate, _ := time.Parse("2006-01-02", s.approvedAt.In(zone).Format("2006-01-02"))
	if date.After(today) || date.Before(approvedDate) {
		return settlementInvalid("Transfer date must be between approval date and today.")
	}
	hash, err := settlementLedger(r, tx, s)
	if err != nil {
		return err
	}
	if hash != s.ledgerHash {
		return settlementInvalid("Ledger changed after approval. Reconcile before recording a transfer.")
	}
	var latest bool
	if err = tx.QueryRow(r.Context(), `SELECT id=(SELECT id FROM seller_payout_payee_versions WHERE seller_id=$2 ORDER BY version DESC LIMIT 1) FROM seller_payout_payee_versions WHERE id=$1 AND seller_id=$2`, s.payeeID, s.sellerID).Scan(&latest); err != nil {
		return err
	}
	if !latest {
		return settlementInvalid("Recipient changed after approval. Reconcile before recording a transfer.")
	}
	if err = h.verifySettlementEvidence(r.Context(), tx, id, "payee"); err != nil {
		return err
	}
	e, err := h.storeSettlementEvidence(r, id, "transfer_evidence")
	if err != nil {
		return err
	}
	commitAttempted := false
	defer func() {
		if !commitAttempted {
			h.discardSettlementEvidence(e)
		}
	}()
	_, err = tx.Exec(r.Context(), `UPDATE seller_settlements SET transfer_amount_cents=$2,transfer_date=$3,bank_reference=$4,transfer_evidence_key=$5,transfer_evidence_type=$6,transfer_evidence_bytes=$7,transfer_evidence_sha256=$8,transfer_submitted_by=$9,transfer_submitted_at=NOW(),revision=revision+1,updated_at=NOW() WHERE id=$1`, id, amount, date, reference, e.key, e.contentType, e.size, e.sha256, actor)
	if err != nil {
		return err
	}
	if err = auditSettlement(r, tx, actor, id, "transfer_submitted"); err != nil {
		return err
	}
	commitAttempted = true
	return tx.Commit(r.Context())
}

func (h *LivePortalHandler) serveSettlementEvidence(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/admin/finance/settlements/evidence" && r.URL.Path != "/super-admin/finance/settlements/evidence" {
		return false
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", 405)
		return true
	}
	session, ok := h.session(r)
	role := "admin"
	if strings.HasPrefix(r.URL.Path, "/super-admin/") {
		role = "super_admin"
	}
	if !ok || !h.hasRole(r, session.UserID, role) {
		http.Error(w, "forbidden", 403)
		return true
	}
	id, err := strconv.ParseInt(r.URL.Query().Get("settlement_id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return true
	}
	e, err := settlementEvidenceRecord(r.Context(), h.pool, id, r.URL.Query().Get("kind"))
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	contents, err := h.settlementEvidenceBytes(r.Context(), e)
	if err != nil {
		http.Error(w, "Evidence could not be verified", 503)
		return true
	}
	kind, size := e.contentType, e.size
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Content-Disposition", `attachment; filename="settlement-evidence"`)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	if r.Method == http.MethodGet {
		_, _ = w.Write(contents)
	}
	return true
}

func (h *LivePortalHandler) adminSettlements(r *http.Request) []viewmodels.AdminSettlement {
	rows, err := h.projectionRows(r, "admin-settlements", `SELECT ss.id,s.display_name,ss.reference,ss.gross_cents,ss.commission_cents,ss.net_cents,ss.status,ss.period_start,ss.period_end,
 (SELECT COUNT(DISTINCT sl.order_id) FROM seller_ledger sl WHERE sl.settlement_id=ss.id AND sl.entry_type='sale'),
 COALESCE(ss.prepared_by,0),COALESCE(ss.approved_by,0),ss.revision,COALESCE(p.version,0),COALESCE(p.account_holder,''),COALESCE(p.bank_name,''),COALESCE(p.account_last4,''),COALESCE(p.routing_code,''),COALESCE(p.verification_note,''),COALESCE(ss.bank_reference,''),COALESCE(to_char(ss.transfer_date,'YYYY-MM-DD'),''),COALESCE(ss.transfer_amount_cents,0),ss.payee_version_id IS NOT NULL,ss.transfer_evidence_key IS NOT NULL,ss.status='paid' AND ss.manual_controls_version=0
 FROM seller_settlements ss JOIN sellers s ON s.id=ss.seller_id LEFT JOIN seller_payout_payee_versions p ON p.id=ss.payee_version_id
 WHERE ($1='' OR concat_ws(' ',s.display_name,ss.reference,ss.status) ILIKE '%'||$1||'%') AND (NOT $3::boolean OR ss.created_at>=date_trunc('day',NOW() AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata'-($2::int-1)*INTERVAL '1 day') ORDER BY ss.id DESC`, portalQuery(r.URL.Query().Get("q")), insightDays(r), superAdminRequest(r))
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := []viewmodels.AdminSettlement{}
	for rows.Next() {
		var item viewmodels.AdminSettlement
		var id, gross, commission, amount, count, revision, version, transfer int64
		var start, end time.Time
		if rows.Scan(&id, &item.Seller, &item.Reference, &gross, &commission, &amount, &item.Status, &start, &end, &count, &item.PreparedBy, &item.ApprovedBy, &revision, &version, &item.PayeeName, &item.BankName, &item.AccountLast4, &item.RoutingCode, &item.VerificationNote, &item.BankReference, &item.TransferDate, &transfer, &item.HasPayee, &item.HasTransfer, &item.LegacyPaid) == nil {
			item.ID, item.Orders, item.Revision, item.PayeeVersion = strconv.FormatInt(id, 10), strconv.FormatInt(count, 10), strconv.FormatInt(revision, 10), strconv.FormatInt(version, 10)
			item.Gross, item.Commission, item.Amount = money(gross), money(commission), money(amount)
			item.TransferAmount = strconv.FormatInt(amount, 10)
			item.Date = end.Format("2 Jan 2006")
			item.Period = start.Format("2 Jan 2006") + " – " + end.Format("2 Jan 2006")
			item.EvidenceState = "Awaiting preparation"
			if item.HasPayee {
				item.EvidenceState = "Recipient prepared"
			}
			if item.HasTransfer {
				item.EvidenceState = "Transfer evidence awaiting confirmation"
			}
			if item.Status == "paid" {
				item.EvidenceState = "Transfer confirmed"
			}
			if item.LegacyPaid {
				item.EvidenceState = "Historical paid record: evidence requires reconciliation"
			}
			result = append(result, item)
		}
	}
	return result
}
