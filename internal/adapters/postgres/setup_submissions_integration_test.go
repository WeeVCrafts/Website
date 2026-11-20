package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

func TestSellerConfirmsSetupForNewOrdersWithAnAuditRecord(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := t.Context()
	x.capture(t)
	invoice, err := x.repo.SellerSetupConfirmation(ctx, x.f.ownerID, 0, "invoice")
	if err != nil || invoice.ReviewRequired || invoice.ProfileRevision <= 0 {
		t.Fatalf("confirmation setup: %+v %v", invoice, err)
	}
	data := ports.SetupSubmissionData{LegalName: "Seller confirmed craft business", Address: "Verified test address Pune 411001", StateCode: "27", Registration: "unregistered"}
	payload, _ := json.Marshal(data)
	in := ports.SetupSubmissionInput{Kind: "invoice", Submit: true, Attest: true, Payload: payload, ExpectedProfileRevision: invoice.ProfileRevision}
	if _, err := x.repo.SaveSetupSubmission(ctx, x.admin, in); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("operator used seller confirmation route: %v", err)
	}
	id, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in)
	if err != nil {
		t.Fatal(err)
	}
	if retry, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in); err != nil || retry != id {
		t.Fatalf("identical retry: id=%d err=%v", retry, err)
	}
	other := in
	data.LegalName = "Stale conflicting business details"
	other.Payload, _ = json.Marshal(data)
	if _, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, other); !errors.Is(err, ports.ErrFulfillmentConflict) {
		t.Fatalf("stale confirmation applied: %v", err)
	}
	var legalName, status string
	var held, attested bool
	var revision int64
	err = x.pool.QueryRow(ctx, `SELECT p.legal_name,m.status,p.checkout_review_hold,
		COALESCE(p.attested_by=$2 AND p.attested_at IS NOT NULL AND p.verified_by IS NULL AND m.reviewed_by IS NULL,false),p.revision
		FROM seller_invoice_profiles p JOIN marketplace_setup_submissions m ON m.seller_id=p.seller_id WHERE m.id=$1`, id, x.f.ownerID).
		Scan(&legalName, &status, &held, &attested, &revision)
	if err != nil || legalName != "Seller confirmed craft business" || status != "attested" || held || !attested || revision != invoice.ProfileRevision+1 {
		t.Fatalf("invoice profile: %q %s hold=%t attested=%t revision=%d error=%v", legalName, status, held, attested, revision, err)
	}
	if err := x.repo.IssueInvoices(ctx, x.order.ID); err != nil {
		t.Fatal(err)
	}
	if err := x.pool.QueryRow(ctx, `SELECT snapshot->'Seller'->>'LegalName' FROM marketplace_invoices WHERE order_id=$1`, x.order.ID).Scan(&legalName); err != nil || legalName != "Verified craft business" {
		t.Fatalf("paid invoice changed: %s %v", legalName, err)
	}
	tax, err := x.repo.SellerSetupConfirmation(ctx, x.f.ownerID, x.f.variantID, "tax")
	if err != nil {
		t.Fatal(err)
	}
	foreign := newMarketplaceFixtureWithPool(t, x.pool)
	if _, err := x.repo.SellerSetupConfirmation(ctx, foreign.f.ownerID, x.f.variantID, "tax"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("other seller read tax confirmation state: %v", err)
	}
	details := ports.SetupSubmissionData{HSN: "4420", RateBasisPoints: 1200, ReturnClass: "fragile_art"}
	payload, _ = json.Marshal(details)
	in = ports.SetupSubmissionInput{Kind: "tax", VariantID: x.f.variantID, Submit: true, Attest: true, Payload: payload, ExpectedProfileRevision: tax.ProfileRevision}
	if _, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in); !errors.Is(err, ports.ErrSetupTaxRegistration) {
		t.Fatalf("unregistered seller recorded a nonzero rate: %v", err)
	}
	details.RateBasisPoints = 0
	in.Payload, _ = json.Marshal(details)
	if _, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in); err != nil {
		t.Fatal(err)
	}
	var classification string
	if err := x.pool.QueryRow(ctx, `SELECT v.return_class,p.attested_by=$2 AND p.verified_by IS NULL
		FROM product_variants v JOIN variant_tax_profiles p ON p.variant_id=v.id WHERE v.id=$1`, x.f.variantID, x.f.ownerID).
		Scan(&classification, &attested); err != nil || classification != details.ReturnClass || !attested {
		t.Fatalf("tax and returns not applied: %q attested=%t err=%v", classification, attested, err)
	}
	rows, _, err := x.repo.SellerReadiness(ctx, x.f.ownerID, 0, x.f.variantID)
	if err != nil || len(rows) != 1 || !rows[0].InvoiceReady || !rows[0].TaxReady || !rows[0].Classified {
		t.Fatalf("seller setup failed to see applied details: %+v %v", rows, err)
	}
	var ready bool
	if err := x.pool.QueryRow(ctx, `SELECT fulfillment_ready AND stock>0 FROM marketplace_checkout_options
		WHERE variant_id=$1`, x.f.variantID).Scan(&ready); err != nil || ready != rows[0].Ready() {
		t.Fatalf("buyer and seller readiness differ: %+v ready=%t err=%v", rows[0], ready, err)
	}
	if err := x.repo.SaveTaxProfile(ctx, x.admin, x.f.variantID, "4420", 0, "Compared to business records"); err != nil {
		t.Fatal(err)
	}
	if err := x.pool.QueryRow(ctx, `SELECT COALESCE(verified_by=$2 AND attested_by IS NULL AND attested_at IS NULL,false)
		FROM variant_tax_profiles WHERE variant_id=$1`, x.f.variantID, x.admin).Scan(&attested); err != nil || !attested {
		t.Fatalf("operator verification provenance not updated: %t %v", attested, err)
	}
	queued, _, err := x.repo.SetupSubmissions(ctx, x.admin, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range queued {
		if record.ID == id {
			t.Fatal("routine confirmation appeared in the review queue")
		}
	}
	history, _, err := x.repo.SetupSubmissions(ctx, x.admin, 0, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range history {
		if record.ID == id && record.Status == "attested" {
			return
		}
	}
	t.Fatal("seller confirmation is absent from Super Admin history")
}

func TestSellerConfirmationCannotClearAnExistingReview(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := t.Context()
	if _, err := x.pool.Exec(ctx, `DELETE FROM seller_invoice_profiles WHERE seller_id=$1`, x.seller); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(ports.SetupSubmissionData{LegalName: "Business in review", Address: "Verified test address Pune 411001", StateCode: "27", Registration: "unregistered"})
	in := ports.SetupSubmissionInput{Kind: "invoice", Payload: payload, Submit: true}
	id, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in)
	if err != nil {
		t.Fatal(err)
	}
	if err := x.repo.ReviewSetupSubmission(ctx, x.admin, id, 1, false, "Correct the billing details"); err != nil {
		t.Fatal(err)
	}
	in.ID, in.ExpectedRevision, in.Submit = id, 2, false
	draft, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in)
	if err != nil || draft == id {
		t.Fatalf("corrected draft failed: %d %v", draft, err)
	}
	in.ID, in.ExpectedRevision, in.Submit, in.Attest = draft, 1, true, true
	if _, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in); !errors.Is(err, ports.ErrSetupReviewRequired) {
		t.Fatalf("seller bypassed review through a correction draft: %v", err)
	}
	in.Attest = false
	if _, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in); err != nil {
		t.Fatal(err)
	}
	if err := x.repo.ReviewSetupSubmission(ctx, x.admin, draft, 2, true, "Reviewed revised billing details"); err != nil {
		t.Fatal(err)
	}
	unheld, err := x.repo.SellerSetupConfirmation(ctx, x.f.ownerID, 0, "invoice")
	if err != nil || unheld.ReviewRequired {
		t.Fatalf("completed review retained hold: %+v %v", unheld, err)
	}
}

func TestSetupSubmissionHoldReviewAndImmutableInvoice(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	x.capture(t)
	data := ports.SetupSubmissionData{LegalName: "Replacement verified business", Address: "Updated merchant address Pune 411001", StateCode: "27", Registration: "unregistered"}
	body, _ := json.Marshal(data)
	in := ports.SetupSubmissionInput{SellerID: x.seller, Kind: "invoice", Payload: body, Submit: true}
	id, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in)
	if err != nil {
		t.Fatal(err)
	}
	var held bool
	if err = x.pool.QueryRow(ctx, `SELECT checkout_review_hold FROM seller_invoice_profiles WHERE seller_id=$1`, x.seller).Scan(&held); err != nil || !held {
		t.Fatalf("seller checkout must pause: %v", err)
	}
	if err = x.repo.IssueInvoices(ctx, x.order.ID); err != nil {
		t.Fatal("paid order invoice must still issue from snapshot", err)
	}
	var original string
	if err = x.pool.QueryRow(ctx, `SELECT snapshot->'Seller'->>'LegalName' FROM marketplace_invoices WHERE order_id=$1`, x.order.ID).Scan(&original); err != nil || original != "Verified craft business" {
		t.Fatalf("invoice identity changed: %s %v", original, err)
	}
	if err = x.repo.ReviewSetupSubmission(ctx, x.f.ownerID, id, 1, true, "Seller cannot approve own changes"); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("self approval: %v", err)
	}
	if err = x.repo.ReviewSetupSubmission(ctx, x.admin, id, 1, true, "Verified replacement merchant evidence"); err != nil {
		t.Fatal(err)
	}
	if err = x.pool.QueryRow(ctx, `SELECT checkout_review_hold FROM seller_invoice_profiles WHERE seller_id=$1`, x.seller).Scan(&held); err != nil || held {
		t.Fatalf("approved hold remains: %v", err)
	}
	if err = x.repo.ReviewSetupSubmission(ctx, x.admin, id, 1, true, "Verified replacement merchant evidence"); err != nil {
		t.Fatal("identical approval retry", err)
	}
	if err = x.repo.IssueInvoices(ctx, x.order.ID); err != nil {
		t.Fatal(err)
	}
	if err = x.pool.QueryRow(ctx, `SELECT snapshot->'Seller'->>'LegalName' FROM marketplace_invoices WHERE order_id=$1`, x.order.ID).Scan(&original); err != nil || original != "Verified craft business" {
		t.Fatal("issued invoice changed")
	}
}

func TestSetupDraftAndRequestedChangesRetainScopedHold(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	body, _ := json.Marshal(ports.SetupSubmissionData{HSN: "4420", RateBasisPoints: 0, ReturnClass: "fragile_art"})
	in := ports.SetupSubmissionInput{SellerID: x.seller, VariantID: x.f.variantID, Kind: "tax", Payload: body}
	id, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in)
	if err != nil {
		t.Fatal(err)
	}
	var held bool
	if err = x.pool.QueryRow(ctx, `SELECT checkout_review_hold FROM variant_tax_profiles WHERE variant_id=$1`, x.f.variantID).Scan(&held); err != nil || held {
		t.Fatal("draft must not pause checkout", err)
	}
	in.ID = id
	in.ExpectedRevision = 1
	in.Submit = true
	if _, err = x.repo.SaveSetupSubmission(ctx, x.f.userIDs[0], in); !errors.Is(err, ports.ErrForbidden) {
		t.Fatal("customer edited seller data", err)
	}
	if _, err = x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in); err != nil {
		t.Fatal(err)
	}
	if err = x.repo.ReviewSetupSubmission(ctx, x.admin, id, 2, false, "Please attach the correct classification evidence"); err != nil {
		t.Fatal(err)
	}
	if err = x.pool.QueryRow(ctx, `SELECT checkout_review_hold FROM variant_tax_profiles WHERE variant_id=$1`, x.f.variantID).Scan(&held); err != nil || !held {
		t.Fatal("requesting changes removed hold", err)
	}
	if err = x.repo.ReviewSetupSubmission(ctx, x.admin, id, 2, true, "Stale approval must not publish"); !errors.Is(err, ports.ErrFulfillmentConflict) {
		t.Fatalf("stale approval: %v", err)
	}
}

func TestSetupTaxDraftPreservesUnspecifiedRateAndClearsDraftOnSubmission(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	raw := ""
	d := ports.SetupSubmissionData{HSN: "4420", ReturnClass: "fragile_art", DraftRatePercent: &raw}
	body, _ := json.Marshal(d)
	in := ports.SetupSubmissionInput{SellerID: x.seller, VariantID: x.f.variantID, Kind: "tax", Payload: body}
	id, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in)
	if err != nil {
		t.Fatal(err)
	}
	list, _, err := x.repo.SetupSubmissions(ctx, x.f.ownerID, id, 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("draft missing: %v", err)
	}
	var saved ports.SetupSubmissionData
	if json.Unmarshal(list[0].Payload, &saved) != nil || saved.DraftRatePercent == nil || *saved.DraftRatePercent != "" {
		t.Fatal("missing draft rate changed to zero")
	}
	d.DraftRatePercent = nil
	body, _ = json.Marshal(d)
	in.ID, in.ExpectedRevision, in.Payload, in.Submit = id, list[0].Revision, body, true
	if _, err = x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in); err != nil {
		t.Fatal(err)
	}
	if _, err = x.repo.SaveSetupSubmission(ctx, x.f.ownerID, in); err != nil {
		t.Fatal("duplicate submission", err)
	}
}
