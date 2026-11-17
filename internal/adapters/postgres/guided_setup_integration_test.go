package postgres

import (
	"context"
	"errors"
	"github.com/wecratfs/commerce/internal/ports"
	"testing"
)

func TestGuidedSetupWritesAreScopedAtomicAndPreserveCommerce(t *testing.T) {
	x := newMarketplaceFixture(t)
	ctx := context.Background()
	var beforeMode string
	if err := x.pool.QueryRow(ctx, `SELECT shipping_mode FROM marketplace_policy_settings WHERE id`).Scan(&beforeMode); err != nil {
		t.Fatal(err)
	}
	parcel := ports.Parcel{WeightGrams: 750, LengthCM: 30, WidthCM: 20, HeightCM: 10}
	var price int64
	var status string
	if err := x.pool.QueryRow(ctx, `SELECT price_cents,status FROM product_variants WHERE id=$1`, x.f.variantID).Scan(&price, &status); err != nil {
		t.Fatal(err)
	}
	if err := x.repo.SaveSetupParcel(ctx, x.f.ownerID, x.seller, x.f.variantID, parcel); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("seller write allowed: %v", err)
	}
	if err := x.repo.SaveSetupParcel(ctx, x.admin, x.seller+99999, x.f.variantID, parcel); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("cross seller write: %v", err)
	}
	invalid := parcel
	invalid.WeightGrams = 0
	if err := x.repo.SaveSetupParcel(ctx, x.admin, x.seller, x.f.variantID, invalid); !errors.Is(err, ports.ErrShippingInput) {
		t.Fatalf("invalid measurements: %v", err)
	}
	if err := x.repo.SaveSetupParcel(ctx, x.admin, x.seller, x.f.variantID, parcel); err != nil {
		t.Fatal(err)
	}
	var weight int
	var afterPrice int64
	var afterStatus string
	if err := x.pool.QueryRow(ctx, `SELECT shipping_weight_g,price_cents,status FROM product_variants WHERE id=$1`, x.f.variantID).Scan(&weight, &afterPrice, &afterStatus); err != nil {
		t.Fatal(err)
	}
	if weight != 750 || price != afterPrice || status != afterStatus {
		t.Fatal("parcel save changed unrelated catalog fields")
	}
	if _, err := x.pool.Exec(ctx, `UPDATE seller_invoice_profiles SET registration='unregistered',gstin='' WHERE seller_id=$1`, x.seller); err != nil {
		t.Fatal(err)
	}
	var beforeHSN, beforeClass string
	var beforeRate int
	if err := x.pool.QueryRow(ctx, `SELECT t.hsn,t.rate_basis_points,v.return_class FROM variant_tax_profiles t JOIN product_variants v ON v.id=t.variant_id WHERE v.id=$1`, x.f.variantID).Scan(&beforeHSN, &beforeRate, &beforeClass); err != nil {
		t.Fatal(err)
	}
	if err := x.repo.SaveSetupTaxAndClass(ctx, x.admin, x.seller, x.f.variantID, "9999", 550, "fragile_art", "Test actual verification"); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("unregistered tax accepted: %v", err)
	}
	var hsn, class string
	var rate int
	if err := x.pool.QueryRow(ctx, `SELECT t.hsn,t.rate_basis_points,v.return_class FROM variant_tax_profiles t JOIN product_variants v ON v.id=t.variant_id WHERE v.id=$1`, x.f.variantID).Scan(&hsn, &rate, &class); err != nil {
		t.Fatal(err)
	}
	if hsn != beforeHSN || rate != beforeRate || class != beforeClass {
		t.Fatal("failed combined save partially changed tax or return category")
	}
	if err := x.repo.SaveSetupTaxAndClass(ctx, x.admin, x.seller, x.f.variantID, "4420", 0, "fragile_art", "Test actual verification"); err != nil {
		t.Fatal(err)
	}
	if err := x.pool.QueryRow(ctx, `SELECT t.hsn,t.rate_basis_points,v.return_class FROM variant_tax_profiles t JOIN product_variants v ON v.id=t.variant_id WHERE v.id=$1`, x.f.variantID).Scan(&hsn, &rate, &class); err != nil {
		t.Fatal(err)
	}
	if hsn != "4420" || rate != 0 || class != "fragile_art" {
		t.Fatal("combined save did not persist both fields")
	}
	var mode string
	if err := x.pool.QueryRow(ctx, `SELECT shipping_mode FROM marketplace_policy_settings WHERE id`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != beforeMode {
		t.Fatal("setup write changed launch mode")
	}
}
