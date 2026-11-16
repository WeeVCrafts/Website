package postgres

import (
	"context"
	"fmt"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
	"time"
)

func policyFixture(t *testing.T) (marketplaceFixture, int64, int64) {
	t.Helper()
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	if _, err := x.pool.Exec(ctx, `UPDATE marketplace_policy_settings SET published_at=now(),shipping_mode='off',pilot_user_id=NULL,pilot_variant_id=NULL,pilot_order_id=NULL WHERE id`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = x.pool.Exec(context.Background(), `UPDATE marketplace_policy_settings SET published_at=NULL,shipping_mode='off',pilot_user_id=NULL,pilot_variant_id=NULL,pilot_order_id=NULL WHERE id`)
	})
	if _, err := x.pool.Exec(ctx, `UPDATE product_variants SET price_cents=30000,return_class='textile_craft' WHERE id=$1`, x.f.variantID); err != nil {
		t.Fatal(err)
	}
	var cart int64
	if err := x.pool.QueryRow(ctx, `INSERT INTO carts(user_id,expires_at) VALUES($1,now()+interval '1 day') RETURNING id`, x.f.userIDs[0]).Scan(&cart); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `INSERT INTO cart_items(cart_id,variant_id,quantity) VALUES($1,$2,2)`, cart, x.f.variantID); err != nil {
		t.Fatal(err)
	}
	var err error
	x.order, err = x.commerce.CreateOrder(ctx, x.f.userIDs[0], cart, fmt.Sprintf("policy-checkout-%d", time.Now().UnixNano()), domain.AddressInput{RecipientName: "Fixture buyer", Phone: "9000000000", Line1: "Fixture lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	x.capture(t)
	var item, parcel int64
	if err = x.pool.QueryRow(ctx, `SELECT id FROM order_items WHERE order_id=$1`, x.order.ID).Scan(&item); err != nil {
		t.Fatal(err)
	}
	if err = x.pool.QueryRow(ctx, `INSERT INTO marketplace_packages(order_id,seller_id,reference,pickup_location,weight_grams,length_cm,width_cm,height_cm,requested_by,status,delivered_at) VALUES($1,$2,$3,'Fixture Depot',500,15,12,8,$4,'delivered',now()-interval '1 hour') RETURNING id`, x.order.ID, x.seller, fmt.Sprintf("fixture-delivered-%d", x.order.ID), x.f.ownerID).Scan(&parcel); err != nil {
		t.Fatal(err)
	}
	if _, err = x.pool.Exec(ctx, `INSERT INTO marketplace_package_items(package_id,order_item_id,quantity) VALUES($1,$2,2)`, parcel, item); err != nil {
		t.Fatal(err)
	}
	if _, err = x.pool.Exec(ctx, `UPDATE seller_fulfillments SET status='delivered',lifecycle_status='delivered',delivered_at=now()-interval '1 hour' WHERE order_id=$1`, x.order.ID); err != nil {
		t.Fatal(err)
	}
	return x, item, parcel
}

func submitPolicyClaim(t *testing.T, x marketplaceFixture, item, parcel int64, kind string) ports.MarketplaceCommand {
	t.Helper()
	c := ports.MarketplaceCommand{Action: "submit_claim", Number: x.order.OrderNumber, SellerID: x.seller, ItemID: item, PackageID: parcel, Quantity: 1, Kind: kind, Reason: "Fixture claim with sufficient inspection context", Key: fmt.Sprintf("policy-claim-%d-%s", x.order.ID, kind)}
	if err := x.repo.Act(context.Background(), x.f.userIDs[0], c); err != nil {
		t.Fatal(err)
	}
	if err := x.pool.QueryRow(context.Background(), `SELECT id FROM marketplace_claims WHERE request_key=$1`, c.Key).Scan(&c.CaseID); err != nil {
		t.Fatal(err)
	}
	c.Action = "claim_status"
	return c
}
func inspectPolicyClaim(t *testing.T, x marketplaceFixture, c ports.MarketplaceCommand) {
	t.Helper()
	ctx := context.Background()
	c.Status = "approved"
	c.EvidenceException = "Fixture staff reviewed alternative evidence and verified this claim"
	if err := x.repo.Act(ctx, x.admin, c); err != nil {
		t.Fatal(err)
	}
	if _, err := x.pool.Exec(ctx, `UPDATE marketplace_claims SET status='pickup_pending',return_reference='WC-RET-'||id,return_status='DELIVERED' WHERE id=$1`, c.CaseID); err != nil {
		t.Fatal(err)
	}
	c.Status = "received"
	if err := x.repo.Act(ctx, x.admin, c); err != nil {
		t.Fatal(err)
	}
	c.Status = "inspected"
	c.InspectionPassed = true
	c.Restock = true
	if err := x.repo.Act(ctx, x.admin, c); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyCreditIssuanceInspectionAndIdempotency(t *testing.T) {
	x, item, parcel := policyFixture(t)
	ctx := context.Background()
	c := submitPolicyClaim(t, x, item, parcel, "change_of_mind")
	c.Status = "completed"
	if err := x.repo.Act(ctx, x.admin, c); err == nil {
		t.Fatal("issued before inspection")
	}
	inspectPolicyClaim(t, x, c)
	c.Status = "completed"
	for i := 0; i < 2; i++ {
		if err := x.repo.Act(ctx, x.admin, c); err != nil {
			t.Fatal(err)
		}
	}
	var grants int
	var amount int64
	if err := x.pool.QueryRow(ctx, `SELECT count(*),COALESCE(sum(amount_cents),0) FROM store_credit_grants WHERE claim_id=$1`, c.CaseID).Scan(&grants, &amount); err != nil || grants != 1 || amount != 10000 {
		t.Fatalf("credit issuance %d %d %v", grants, amount, err)
	}
	if err := x.repo.IssueInvoices(ctx, x.order.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := x.repo.IssueClaimCreditNote(ctx, c.CaseID); err != nil {
			t.Fatal(err)
		}
	}
	var notes int
	if err := x.pool.QueryRow(ctx, `SELECT count(*) FROM marketplace_credit_notes WHERE claim_id=$1`, c.CaseID).Scan(&notes); err != nil || notes != 1 {
		t.Fatalf("credit note %d %v", notes, err)
	}
}

func TestPolicyClaimsBoundToDeliveredPackageAndWindow(t *testing.T) {
	x, item, parcel := policyFixture(t)
	ctx := context.Background()
	if _, err := x.pool.Exec(ctx, `UPDATE marketplace_packages SET delivered_at=now()-interval '49 hours' WHERE id=$1`, parcel); err != nil {
		t.Fatal(err)
	}
	c := ports.MarketplaceCommand{Action: "submit_claim", Number: x.order.OrderNumber, SellerID: x.seller, ItemID: item, PackageID: parcel, Quantity: 1, Kind: "damage", Reason: "Fixture attempted claim outside the deadline", Key: fmt.Sprintf("late-policy-claim-%d", x.order.ID)}
	if err := x.repo.Act(ctx, x.f.userIDs[0], c); err == nil {
		t.Fatal("late damage claim accepted")
	}
	c.Kind = "change_of_mind"
	if err := x.repo.Act(ctx, x.f.userIDs[0], c); err != nil {
		t.Fatal(err)
	}
	c.PackageID = parcel + 99999
	c.Key += "other"
	if err := x.repo.Act(ctx, x.f.userIDs[0], c); err == nil {
		t.Fatal("unrelated package accepted")
	}
}

func TestPolicyReplacementRequiresApprovedStockAndInspection(t *testing.T) {
	x, item, parcel := policyFixture(t)
	ctx := context.Background()
	c := submitPolicyClaim(t, x, item, parcel, "damage")
	inspectPolicyClaim(t, x, c)
	c.Action = "prepare_replacement"
	c.Parcel = ports.Parcel{WeightGrams: 500, LengthCM: 15, WidthCM: 12, HeightCM: 8}
	c.QCEvidence = "Fixture private quality check confirms correct replacement"
	if err := x.repo.Act(ctx, x.f.userIDs[1], c); err == nil {
		t.Fatal("another customer prepared replacement")
	}
	for i := 0; i < 2; i++ {
		if err := x.repo.Act(ctx, x.f.ownerID, c); err != nil {
			t.Fatal(err)
		}
	}
	var packages int
	if err := x.pool.QueryRow(ctx, `SELECT count(*) FROM marketplace_packages WHERE replacement_claim_id=$1`, c.CaseID).Scan(&packages); err != nil || packages != 1 {
		t.Fatalf("replacement duplicated %d %v", packages, err)
	}
	var fakeCaptures int
	if err := x.pool.QueryRow(ctx, `SELECT count(*) FROM payment_attempts WHERE order_id=$1`, x.order.ID).Scan(&fakeCaptures); err != nil || fakeCaptures != 1 {
		t.Fatalf("replacement fabricated payment %d %v", fakeCaptures, err)
	}
}

func TestPolicyEvidenceStaysPrivateUntilExplicitSellerSharing(t *testing.T) {
	x, item, parcel := policyFixture(t)
	c := submitPolicyClaim(t, x, item, parcel, "damage")
	ctx := context.Background()
	if err := x.repo.SaveClaimEvidence(ctx, x.f.userIDs[0], c.CaseID, fmt.Sprintf("private/fixture-%d", c.CaseID), "unboxing.mp4", "video/mp4", 24); err != nil {
		t.Fatal(err)
	}
	var evidence int64
	if err := x.pool.QueryRow(ctx, `SELECT id FROM marketplace_claim_evidence WHERE claim_id=$1`, c.CaseID).Scan(&evidence); err != nil {
		t.Fatal(err)
	}
	for _, user := range []int64{x.f.ownerID, x.f.userIDs[1]} {
		if _, _, _, err := x.repo.ClaimEvidenceObject(ctx, user, evidence); err == nil {
			t.Fatal("private evidence exposed")
		}
	}
	for _, user := range []int64{x.f.userIDs[0], x.admin} {
		if _, _, _, err := x.repo.ClaimEvidenceObject(ctx, user, evidence); err != nil {
			t.Fatal(err)
		}
	}
	c.Action = "share_claim_evidence"
	if err := x.repo.Act(ctx, x.admin, c); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := x.repo.ClaimEvidenceObject(ctx, x.f.ownerID, evidence); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := x.repo.ClaimEvidenceObject(ctx, x.f.userIDs[1], evidence); err == nil {
		t.Fatal("sharing exposed evidence to other buyer")
	}
}
