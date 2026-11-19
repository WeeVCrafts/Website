package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

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
