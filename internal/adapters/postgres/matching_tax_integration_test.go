package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestApproveMatchingTaxSubmissionsIsAtomic(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := t.Context()
	submit := func(variant int64, hsn string) ports.SetupSubmission {
		t.Helper()
		payload, err := json.Marshal(ports.SetupSubmissionData{HSN: hsn, ReturnClass: "fragile_art"})
		if err != nil {
			t.Fatal(err)
		}
		id, err := x.repo.SaveSetupSubmission(ctx, x.f.ownerID, ports.SetupSubmissionInput{
			Kind: "tax", SellerID: x.seller, VariantID: variant, Submit: true, Payload: payload,
		})
		if err != nil {
			t.Fatal(err)
		}
		rows, _, err := x.repo.SetupSubmissions(ctx, x.admin, id, 0)
		if err != nil || len(rows) != 1 {
			t.Fatalf("read tax submission: count %d, %v", len(rows), err)
		}
		return rows[0]
	}
	addVariant := func(label string) int64 {
		t.Helper()
		var id int64
		err := x.pool.QueryRow(ctx, `INSERT INTO product_variants(product_id,sku,name,price_cents,status)
			SELECT product_id,$2,$2,price_cents,'active' FROM product_variants WHERE id=$1 RETURNING id`,
			x.f.variantID, fmt.Sprintf("BATCH-TAX-%d-%s", x.f.variantID, label)).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := submit(x.f.variantID, "4420")
	second := submit(addVariant("matching"), "4420")
	different := submit(addVariant("different"), "4414")
	approve := func(a, b ports.SetupSubmission) error {
		return x.repo.ApproveMatchingTaxSubmissions(ctx, x.admin, map[int64]int64{a.ID: a.Revision, b.ID: b.Revision}, "Verified matching option records")
	}
	if err := approve(first, different); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("different classifications were accepted: %v", err)
	}
	stale := second
	stale.Revision--
	if err := approve(first, stale); !errors.Is(err, ports.ErrFulfillmentConflict) {
		t.Fatalf("stale classification was accepted: %v", err)
	}
	if err := approve(first, second); err != nil {
		t.Fatalf("matching tax submissions did not approve together: %v", err)
	}
	rows, _, err := x.repo.SetupSubmissions(ctx, x.admin, different.ID, 0)
	if err != nil || len(rows) != 1 || rows[0].Status != "submitted" {
		t.Fatalf("different submission was changed: %+v, %v", rows, err)
	}
}
