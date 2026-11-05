package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	postgresadapter "github.com/wecratfs/commerce/internal/adapters/postgres"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

const financePreparer = 900101
const financeApprover = 900102
const financeSeller = 900103
const financeCustomer = 900104

type financeStorage struct {
	objects map[string][]byte
	types   map[string]string
}

func (s *financeStorage) CreateUploadURL(context.Context, ports.UploadRequest) (ports.UploadURL, error) {
	return ports.UploadURL{}, nil
}
func (s *financeStorage) PutObject(_ context.Context, key, kind string, _ int64, body io.Reader) error {
	b, e := io.ReadAll(body)
	s.objects[key] = b
	s.types[key] = kind
	return e
}
func (s *financeStorage) OpenObject(_ context.Context, key string) (io.ReadCloser, ports.ObjectInfo, error) {
	b, ok := s.objects[key]
	if !ok {
		return nil, ports.ObjectInfo{}, fmt.Errorf("missing")
	}
	return io.NopCloser(bytes.NewReader(b)), ports.ObjectInfo{ContentType: s.types[key], ByteSize: int64(len(b))}, nil
}

func financeTestHandler(t *testing.T) *LivePortalHandler { return financeTestHandlerAtVersion(t, 55) }

func financeTestHandlerAtVersion(t *testing.T, lastVersion int) *LivePortalHandler {
	t.Helper()
	dsn := os.Getenv("FINANCE_DATABASE_URL")
	if dsn == "" {
		t.Skip("FINANCE_DATABASE_URL required for isolated manual payout tests")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.Database != "weevcrafts_finance_20261104" {
		t.Fatal("finance tests require their dedicated database")
	}
	admin, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("finance_%d", time.Now().UnixNano())
	if _, err = admin.Exec(context.Background(), `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.ConnConfig.RuntimeParams["application_name"] = schema
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	})
	if lastVersion == 55 {
		if err = postgresadapter.ApplyMigrations(context.Background(), pool); err != nil {
			t.Fatal(err)
		}
	} else {
		if _, err = pool.Exec(context.Background(), `CREATE TABLE schema_migrations(version BIGINT PRIMARY KEY,applied_at TIMESTAMPTZ DEFAULT NOW())`); err != nil {
			t.Fatal(err)
		}
		folder := "../../adapters/postgres/migrations"
		entries, err := os.ReadDir(folder)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			version, err := strconv.Atoi(strings.SplitN(entry.Name(), "_", 2)[0])
			if err != nil {
				t.Fatal(err)
			}
			if version > lastVersion {
				continue
			}
			data, err := os.ReadFile(filepath.Join(folder, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(context.Background(), string(data)); err != nil {
				t.Fatalf("historical migration %s: %v", entry.Name(), err)
			}
			if _, err = pool.Exec(context.Background(), `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO users(id,email,display_name,status) VALUES(900101,'prepare@example.invalid','Preparer','active'),(900102,'approve@example.invalid','Approver','active'),(900103,'seller@example.invalid','Seller','active'),(900104,'customer@example.invalid','Customer','active'); INSERT INTO user_roles(user_id,role_slug) VALUES(900101,'admin'),(900102,'super_admin'),(900103,'seller'),(900104,'customer'); INSERT INTO sellers(id,owner_user_id,display_name,status) VALUES(900103,900103,'Finance Seller','active'); INSERT INTO orders(id,order_number,user_id,status,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) VALUES(900104,'FINANCE-ORDER',900104,'paid',1000,1000,'{}','finance','one'); INSERT INTO seller_ledger(seller_id,order_id,entry_type,gross_cents,commission_cents,net_cents) VALUES(900103,900104,'sale',1000,100,900);`)
	if err != nil {
		t.Fatal(err)
	}
	h := NewLivePortalHandler(pool, nil, nil)
	h.SetSettlementStorage(&financeStorage{objects: map[string][]byte{}, types: map[string]string{}})
	return h
}

func financeRequest(fields url.Values, fileField string) *http.Request {
	if fileField == "" {
		r := httptest.NewRequest("POST", "/admin/finance/settlements/status", strings.NewReader(fields.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		_ = r.ParseForm()
		return r
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, values := range fields {
		for _, value := range values {
			_ = writer.WriteField(key, value)
		}
	}
	part, _ := writer.CreateFormFile(fileField, "evidence.pdf")
	_, _ = part.Write([]byte("%PDF-1.7\nfinance evidence\n%%EOF"))
	_ = writer.Close()
	r := httptest.NewRequest("POST", "/admin/finance/settlements/prepare", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	_ = r.ParseMultipartForm(6 << 20)
	return r
}

func financeRevision(t *testing.T, h *LivePortalHandler, id int64) string {
	t.Helper()
	var n int64
	if err := h.pool.QueryRow(context.Background(), `SELECT revision FROM seller_settlements WHERE id=$1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(n)
}
func financeBatch(t *testing.T, h *LivePortalHandler) int64 {
	t.Helper()
	if err := h.createSettlementBatches(financeRequest(url.Values{}, ""), financePreparer); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := h.pool.QueryRow(context.Background(), `SELECT id FROM seller_settlements ORDER BY id DESC LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func financePrepare(t *testing.T, h *LivePortalHandler, id int64) {
	t.Helper()
	r := financeRequest(url.Values{"revision": {financeRevision(t, h, id)}, "payee_name": {"Seller Account"}, "bank_name": {"Verified Bank"}, "account_last4": {"1234"}, "routing_code": {"ABCD0123456"}, "verification_note": {"Match seller supplied bank confirmation"}}, "payee_evidence")
	if err := h.prepareSettlement(r, id, financePreparer); err != nil {
		t.Fatal(err)
	}
}
func financeApprove(t *testing.T, h *LivePortalHandler, id int64) {
	t.Helper()
	r := financeRequest(url.Values{"revision": {financeRevision(t, h, id)}, "verified_payee": {"yes"}}, "")
	if err := h.updateSettlementStatus(r, id, "approved", financeApprover); err != nil {
		t.Fatal(err)
	}
}
func financeTransfer(t *testing.T, h *LivePortalHandler, id int64) error {
	t.Helper()
	r := financeRequest(url.Values{"revision": {financeRevision(t, h, id)}, "transfer_amount_cents": {"900"}, "transfer_date": {time.Now().In(time.FixedZone("IST", 19800)).Format("2006-01-02")}, "bank_reference": {"BANK-UTR-1001"}}, "transfer_evidence")
	return h.recordSettlementTransfer(r, id, financePreparer)
}

func TestManualSettlementRequiresIndependentApprovalAndTransferEvidence(t *testing.T) {
	h := financeTestHandler(t)
	id := financeBatch(t, h)
	r := financeRequest(url.Values{"revision": {financeRevision(t, h, id)}, "verified_payee": {"yes"}}, "")
	if err := h.updateSettlementStatus(r, id, "approved", financeApprover); err == nil {
		t.Fatal("unprepared recipient approved")
	}
	financePrepare(t, h, id)
	r = financeRequest(url.Values{"revision": {financeRevision(t, h, id)}, "verified_payee": {"yes"}}, "")
	if err := h.updateSettlementStatus(r, id, "approved", financePreparer); err == nil {
		t.Fatal("preparer approved own batch")
	}
	financeApprove(t, h, id)
	r = financeRequest(url.Values{"revision": {financeRevision(t, h, id)}, "verified_transfer": {"yes"}}, "")
	if err := h.updateSettlementStatus(r, id, "paid", financeApprover); err == nil {
		t.Fatal("paid without transfer evidence")
	}
	if err := financeTransfer(t, h, id); err != nil {
		t.Fatal(err)
	}
	r = financeRequest(url.Values{"revision": {financeRevision(t, h, id)}, "verified_transfer": {"yes"}}, "")
	if err := h.updateSettlementStatus(r, id, "paid", financePreparer); err == nil {
		t.Fatal("preparer confirmed own transfer")
	}
	if err := h.updateSettlementStatus(r, id, "paid", financeApprover); err != nil {
		t.Fatal(err)
	}
	if err := h.updateSettlementStatus(r, id, "paid", financeApprover); err == nil {
		t.Fatal("duplicate paid decision accepted")
	}
	var state, ledger, bank string
	var count int
	if err := h.pool.QueryRow(context.Background(), `SELECT ss.status,sl.status,ss.bank_reference,(SELECT count(*) FROM audit_logs WHERE resource_type='seller_settlement') FROM seller_settlements ss JOIN seller_ledger sl ON sl.settlement_id=ss.id WHERE ss.id=$1`, id).Scan(&state, &ledger, &bank, &count); err != nil {
		t.Fatal(err)
	}
	if state != "paid" || ledger != "settled" || bank != "BANK-UTR-1001" || count != 5 {
		t.Fatalf("state=%s ledger=%s reference=%s audits=%d", state, ledger, bank, count)
	}
}

func TestManualSettlementRejectsChangedLedgerRecipientAndStaleDecisions(t *testing.T) {
	for _, change := range []string{"amount", "membership", "hold", "refund", "payee"} {
		t.Run(change, func(t *testing.T) {
			h := financeTestHandler(t)
			id := financeBatch(t, h)
			financePrepare(t, h, id)
			financeApprove(t, h, id)
			var sql string
			switch change {
			case "amount":
				sql = `UPDATE seller_ledger SET net_cents=net_cents+1`
			case "membership":
				sql = `UPDATE seller_ledger SET note='unrelated note'; INSERT INTO seller_ledger(seller_id,entry_type,gross_cents,commission_cents,net_cents,settlement_id) SELECT seller_id,'adjustment',0,0,0,id FROM seller_settlements`
			case "hold":
				sql = `UPDATE orders SET payment_review_required=true`
			case "refund":
				sql = `UPDATE orders SET status='refunded'`
			case "payee":
				sql = `INSERT INTO seller_payout_payee_versions(seller_id,version,account_holder,bank_name,account_last4,routing_code,verification_note,evidence_key,evidence_type,evidence_bytes,evidence_sha256,prepared_by) SELECT seller_id,version+1,account_holder,bank_name,'9876',routing_code,verification_note,evidence_key||'-changed',evidence_type,evidence_bytes,evidence_sha256,prepared_by FROM seller_payout_payee_versions`
			}
			if _, err := h.pool.Exec(context.Background(), sql); err != nil {
				t.Fatal(err)
			}
			if err := financeTransfer(t, h, id); err == nil {
				t.Fatalf("transfer accepted after %s change", change)
			}
		})
	}
	h := financeTestHandler(t)
	id := financeBatch(t, h)
	stale := financeRevision(t, h, id)
	financePrepare(t, h, id)
	if err := h.updateSettlementStatus(financeRequest(url.Values{"revision": {stale}, "verified_payee": {"yes"}}, ""), id, "approved", financeApprover); err == nil {
		t.Fatal("stale approval accepted")
	}
}

func TestManualSettlementTransferCannotBeReplacedOrReleased(t *testing.T) {
	h := financeTestHandler(t)
	id := financeBatch(t, h)
	financePrepare(t, h, id)
	financeApprove(t, h, id)
	if err := financeTransfer(t, h, id); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(context.Background(), `UPDATE seller_settlements SET status='paid',paid_at=NOW(),paid_by=NULL WHERE id=$1`, id); err == nil {
		t.Fatal("NULL paid actor bypassed database controls")
	}
	if err := financeTransfer(t, h, id); err == nil {
		t.Fatal("duplicate transfer proof accepted")
	}
	r := financeRequest(url.Values{"revision": {financeRevision(t, h, id)}}, "")
	if err := h.updateSettlementStatus(r, id, "failed", financeApprover); err == nil {
		t.Fatal("submitted transfer released into a new batch")
	}
	if _, err := h.pool.Exec(context.Background(), `UPDATE seller_ledger SET net_cents=901`); err != nil {
		t.Fatal(err)
	}
	r = financeRequest(url.Values{"revision": {financeRevision(t, h, id)}, "verified_transfer": {"yes"}}, "")
	if err := h.updateSettlementStatus(r, id, "paid", financeApprover); err == nil {
		t.Fatal("changed ledger marked paid")
	}
}

func TestManualSettlementConcurrentApprovalAndPaymentAreSingleTransitions(t *testing.T) {
	h := financeTestHandler(t)
	id := financeBatch(t, h)
	financePrepare(t, h, id)
	for _, status := range []string{"approved", "paid"} {
		if status == "paid" {
			if err := financeTransfer(t, h, id); err != nil {
				t.Fatal(err)
			}
		}
		revision := financeRevision(t, h, id)
		var group sync.WaitGroup
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			group.Add(1)
			go func() {
				defer group.Done()
				r := financeRequest(url.Values{"revision": {revision}, "verified_payee": {"yes"}, "verified_transfer": {"yes"}}, "")
				results <- h.updateSettlementStatus(r, id, status, financeApprover)
			}()
		}
		group.Wait()
		close(results)
		passed := 0
		for err := range results {
			if err == nil {
				passed++
			}
		}
		if passed != 1 {
			t.Fatalf("%s successes=%d", status, passed)
		}
	}
}

func TestManualSettlementDatabaseRejectsLegacyWriterAndPreservesHistoricalPaid(t *testing.T) {
	h := financeTestHandlerAtVersion(t, 54)
	_, err := h.pool.Exec(context.Background(), `INSERT INTO seller_settlements(seller_id,period_start,period_end,reference,status,net_cents) VALUES(900103,CURRENT_DATE,CURRENT_DATE,'HISTORICAL-PAID','paid',900); INSERT INTO seller_settlements(seller_id,period_start,period_end,reference,net_cents) VALUES(900103,CURRENT_DATE,CURRENT_DATE,'LEGACY-PENDING',900);`)
	if err != nil {
		t.Fatal(err)
	}
	if err = postgresadapter.ApplyMigrations(context.Background(), h.pool); err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(context.Background(), `UPDATE seller_settlements SET status='paid' WHERE reference='LEGACY-PENDING'`); err == nil {
		t.Fatal("legacy status-only writer bypassed controls")
	}
	rows := h.adminSettlements(projectionRequest(httptest.NewRequest("GET", "/admin/finance", nil)))
	found := false
	for _, row := range rows {
		if row.Reference == "HISTORICAL-PAID" {
			found = row.LegacyPaid && row.Status == "paid" && !row.HasTransfer && strings.Contains(row.EvidenceState, "reconciliation")
		}
	}
	if !found {
		t.Fatal("historical paid row lost or presented as verified")
	}
}

type financeSessionStore struct {
	records map[string]ports.SessionRecord
}

func (s *financeSessionStore) List(_ context.Context, userID int64) ([]ports.SessionRecord, error) {
	var result []ports.SessionRecord
	for _, r := range s.records {
		if r.UserID == userID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *financeSessionStore) Create(_ context.Context, r ports.SessionRecord) error {
	s.records[r.ID] = r
	return nil
}
func (s *financeSessionStore) Get(_ context.Context, id string) (ports.SessionRecord, error) {
	r, ok := s.records[id]
	if !ok {
		return r, applicationauth.ErrInvalidSession
	}
	return r, nil
}
func (s *financeSessionStore) Delete(_ context.Context, id string) error {
	delete(s.records, id)
	return nil
}
func (s *financeSessionStore) DeleteAll(_ context.Context, id int64) error {
	for key, r := range s.records {
		if r.UserID == id {
			delete(s.records, key)
		}
	}
	return nil
}

func TestManualSettlementPrivateEvidenceAndHTTPBoundaries(t *testing.T) {
	h := financeTestHandler(t)
	id := financeBatch(t, h)
	financePrepare(t, h, id)
	store := &financeSessionStore{records: map[string]ports.SessionRecord{}}
	h.sessions = applicationauth.NewSessionService(store)
	for _, test := range []struct {
		actor int64
		role  domainidentity.Role
		path  string
		want  int
	}{
		{financePreparer, domainidentity.RoleAdmin, "/admin/finance/settlements/evidence", 200},
		{financeApprover, domainidentity.RoleSuperAdmin, "/super-admin/finance/settlements/evidence", 200},
		{financeSeller, domainidentity.RoleSeller, "/admin/finance/settlements/evidence", 403},
		{financeCustomer, domainidentity.RoleCustomer, "/admin/finance/settlements/evidence", 403},
	} {
		session, err := h.sessions.Start(context.Background(), domainidentity.User{ID: test.actor, Roles: []domainidentity.Role{test.role}}, "test", "", time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("GET", test.path+"?kind=payee&settlement_id="+strconv.FormatInt(id, 10), nil)
		r.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != test.want {
			t.Fatalf("actor%d status=%d body=%s", test.actor, w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("private evidence cacheable")
		}
		if test.want == 200 && !strings.HasPrefix(w.Body.String(), "%PDF-") {
			t.Fatal("private evidence missing")
		}
		// No CSRF token: neither canonical nor alternate company route can mutate.
		if test.role == domainidentity.RoleAdmin || test.role == domainidentity.RoleSuperAdmin {
			root := strings.TrimSuffix(test.path, "evidence")
			r = financeRequest(url.Values{"settlement_id": {fmt.Sprint(id)}, "revision": {financeRevision(t, h, id)}, "status": {"paid"}}, "")
			r.URL.Path = root + "status"
			r.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
			w = httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("missing CSRF status=%d", w.Code)
			}
		}
	}
	var key string
	if err := h.pool.QueryRow(context.Background(), `SELECT p.evidence_key FROM seller_settlements s JOIN seller_payout_payee_versions p ON p.id=s.payee_version_id WHERE s.id=$1`, id).Scan(&key); err != nil {
		t.Fatal(err)
	}
	storage := h.settlementStorage.(*financeStorage)
	storage.objects[key][len(storage.objects[key])-1] ^= 1
	session, err := h.sessions.Start(context.Background(), domainidentity.User{ID: financePreparer, Roles: []domainidentity.Role{domainidentity.RoleAdmin}}, "test", "", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", fmt.Sprintf("/admin/finance/settlements/evidence?kind=payee&settlement_id=%d", id), nil)
	r.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatalf("corrupted evidence status=%d", w.Code)
	}
}

func TestManualSettlementRejectsWrongAmountFutureDateAndCompanySelfConfirmation(t *testing.T) {
	h := financeTestHandler(t)
	id := financeBatch(t, h)
	financePrepare(t, h, id)
	financeApprove(t, h, id)
	for _, values := range []url.Values{
		{"revision": {financeRevision(t, h, id)}, "transfer_amount_cents": {"901"}, "transfer_date": {time.Now().Format("2006-01-02")}, "bank_reference": {"UTR-0001"}},
		{"revision": {financeRevision(t, h, id)}, "transfer_amount_cents": {"900"}, "transfer_date": {"2099-01-01"}, "bank_reference": {"UTR-0001"}},
	} {
		if err := h.recordSettlementTransfer(financeRequest(values, "transfer_evidence"), id, financePreparer); err == nil {
			t.Fatal("invalid amount/date accepted")
		}
	}
	if err := financeTransfer(t, h, id); err != nil {
		t.Fatal(err)
	}
	store := &financeSessionStore{records: map[string]ports.SessionRecord{}}
	h.sessions = applicationauth.NewSessionService(store)
	session, err := h.sessions.Start(context.Background(), domainidentity.User{ID: financePreparer, Roles: []domainidentity.Role{domainidentity.RoleAdmin}}, "test", "", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/admin/finance/settlements/status", "/super-admin/finance/settlements/status", "/finance-portal/settlements/status", "/admin/settlements/status"} {
		r := financeRequest(url.Values{"csrf_token": {session.CSRFToken}, "settlement_id": {fmt.Sprint(id)}, "revision": {financeRevision(t, h, id)}, "status": {"paid"}, "verified_transfer": {"yes"}}, "")
		r.URL.Path = path
		r.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code >= 200 && w.Code < 400 {
			t.Fatalf("preparer self-confirmation path %s returned %d", path, w.Code)
		}
		var status string
		if err = h.pool.QueryRow(context.Background(), `SELECT status FROM seller_settlements WHERE id=$1`, id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "approved" {
			t.Fatalf("path%s changed status %s", path, status)
		}
	}
}

func TestManualSettlementRevalidatesStoredEvidenceBeforeFinancialDecisions(t *testing.T) {
	for _, stage := range []string{"approval_missing_payee", "approval_tampered_payee", "paid_missing_payee", "paid_tampered_transfer", "paid_storage_unavailable"} {
		t.Run(stage, func(t *testing.T) {
			h := financeTestHandler(t)
			id := financeBatch(t, h)
			financePrepare(t, h, id)
			approval := strings.HasPrefix(stage, "approval")
			if !approval {
				financeApprove(t, h, id)
				if err := financeTransfer(t, h, id); err != nil {
					t.Fatal(err)
				}
			}
			kind := "payee"
			if strings.Contains(stage, "transfer") {
				kind = "transfer"
			}
			e, err := settlementEvidenceRecord(context.Background(), h.pool, id, kind)
			if err != nil {
				t.Fatal(err)
			}
			storage := h.settlementStorage.(*financeStorage)
			switch {
			case strings.Contains(stage, "missing"):
				delete(storage.objects, e.key)
			case strings.Contains(stage, "tampered"):
				storage.objects[e.key][len(storage.objects[e.key])-1] ^= 1
			case strings.Contains(stage, "unavailable"):
				h.settlementStorage = nil
			}
			status := "paid"
			if approval {
				status = "approved"
			}
			r := financeRequest(url.Values{"revision": {financeRevision(t, h, id)}, "verified_payee": {"yes"}, "verified_transfer": {"yes"}}, "")
			if err := h.updateSettlementStatus(r, id, status, financeApprover); err == nil {
				t.Fatal("financial decision accepted without valid stored evidence")
			}
			var actual string
			if err := h.pool.QueryRow(context.Background(), `SELECT status FROM seller_settlements WHERE id=$1`, id).Scan(&actual); err != nil {
				t.Fatal(err)
			}
			want := "approved"
			if approval {
				want = "pending"
			}
			if actual != want {
				t.Fatalf("status=%s want%s", actual, want)
			}
		})
	}
}

func TestPendingRefundUsesSettlementLockBeforePublishingLiability(t *testing.T) {
	h := financeTestHandler(t)
	id := financeBatch(t, h)
	financePrepare(t, h, id)
	financeApprove(t, h, id)
	if err := financeTransfer(t, h, id); err != nil {
		t.Fatal(err)
	}
	_, err := h.pool.Exec(context.Background(), `INSERT INTO payment_attempts(order_id,provider,provider_payment_id,status,amount_cents,currency) VALUES(900104,'razorpay','pay_finance_lock','captured',1000,'INR'); INSERT INTO return_requests(order_id,user_id,reason,status) VALUES(900104,900104,'Finance lock fixture','received')`)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	holder, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(context.Background())
	if _, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(4856203)`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		_, err := postgresadapter.NewPaymentRepository(h.pool).BeginRefund(ctx, financeCustomer, "FINANCE-ORDER", "Returned order", "finance-lock-refund")
		finished <- err
	}()
	waiting := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		// Observe from an autocommit connection: pg_stat_activity's statistics
		// snapshot would otherwise remain cached in the lock-holder transaction.
		if err = h.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND application_name=$1 AND wait_event='advisory')`, h.pool.Config().ConnConfig.RuntimeParams["application_name"]).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-finished:
			t.Fatalf("refund bypassed finance lock: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	if !waiting {
		t.Fatal("refund did not reach shared finance lock")
	}
	var count int
	if err = holder.QueryRow(ctx, `SELECT count(*) FROM payment_refunds WHERE order_id=900104`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("pending liability published before finance lock: count%d err%v", count, err)
	}
	if err = holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	r := financeRequest(url.Values{"revision": {financeRevision(t, h, id)}, "verified_transfer": {"yes"}}, "")
	if err = h.updateSettlementStatus(r, id, "paid", financeApprover); err == nil {
		t.Fatal("settlement paid despite serialized pending refund")
	}
}
