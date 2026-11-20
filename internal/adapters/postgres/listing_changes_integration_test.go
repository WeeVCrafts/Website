package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestPublishedVariantProposalCanBeEditedAndWithdrawn(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 1, 3)
	ctx := t.Context()
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller'),($2,'super_admin')`, f.ownerID, f.userIDs[0]); err != nil {
		t.Fatal(err)
	}
	catalog := NewCommerceRepository(pool)
	input := commerce.ProductVariantInput{Name: "New option", SKU: f.brandSlug + "-proposed", PriceCents: 1800, InitialStock: 1}
	option, err := catalog.CreateSellerProductVariant(ctx, f.ownerID, f.productID, input)
	if err != nil || option.Status != "pending_review" {
		t.Fatalf("created option=%+v err=%v", option, err)
	}
	review, err := catalog.ListingChange(ctx, f.userIDs[0], f.productID)
	if err != nil || len(review.Prices) != 1 || review.Prices[0].Proposed != 1800 {
		t.Fatalf("new option review=%+v err=%v", review, err)
	}
	input.PriceCents = 2100
	input.Name = "Edited option"
	if _, err := catalog.UpdateSellerProductVariant(ctx, f.ownerID, f.productID, option.ID, input); err != nil {
		t.Fatal(err)
	}
	if err := catalog.ReviewListingChange(ctx, f.userIDs[0], f.productID, review.Revision, true, "Old version"); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("old revision accepted: %v", err)
	}
	review, err = catalog.ListingChange(ctx, f.userIDs[0], f.productID)
	if err != nil || len(review.Prices) != 1 || review.Prices[0].Proposed != 2100 || review.Prices[0].Option != "Edited option" {
		t.Fatalf("edited option review=%+v err=%v", review, err)
	}
	if err := catalog.ArchiveSellerProductVariant(ctx, f.ownerID, f.productID, option.ID, "Withdraw this option"); err != nil {
		t.Fatal(err)
	}
	if err := catalog.ReviewListingChange(ctx, f.userIDs[0], f.productID, review.Revision, true, "Withdrawn version"); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("withdrawn option approved: %v", err)
	}
	review, err = catalog.ListingChange(ctx, f.userIDs[0], f.productID)
	if err != nil || review.State == "pending" || len(review.Prices) != 0 {
		t.Fatalf("withdrawal review=%+v err=%v", review, err)
	}
}

func TestPublishedListingChangesRequireMatchingSuperAdminReview(t *testing.T) {
	pool := openIntegrationPool(t)
	f := newCommerceFixture(t, pool, 2, 4)
	ctx := t.Context()
	for _, role := range []struct {
		id   int64
		name string
	}{{f.ownerID, "seller"}, {f.userIDs[0], "super_admin"}, {f.userIDs[1], "admin"}} {
		if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,$2)`, role.id, role.name); err != nil {
			t.Fatal(err)
		}
	}
	catalog := NewCommerceRepository(pool)
	media := NewMediaRepository(pool)
	current, err := catalog.GetSellerProduct(ctx, f.ownerID, f.productID)
	if err != nil {
		t.Fatal(err)
	}
	input := commerce.ProductDraftInput{Slug: current.Slug, BrandSlug: current.BrandSlug, BrandName: current.BrandName,
		CategorySlug: current.CategorySlug, CategoryName: current.CategoryName, Name: current.Name, Description: "Updated public description",
		SKU: current.SKU, PriceCents: 1000}
	saved, err := catalog.UpdateDraft(ctx, f.ownerID, f.productID, input)
	if err != nil || saved.Status != "approved" || saved.Description != input.Description {
		t.Fatalf("routine edit did not preserve public listing: %+v %v", saved, err)
	}
	if _, err := catalog.ListingChange(ctx, f.ownerID, f.productID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("routine edit created unexpected review: %v", err)
	}
	input.PriceCents = 1200
	if _, err := catalog.UpdateDraft(ctx, f.ownerID, f.productID, input); err != nil {
		t.Fatal(err)
	}
	review, err := catalog.ListingChange(ctx, f.userIDs[0], f.productID)
	if err != nil || len(review.Prices) != 1 || review.Prices[0].Current != 1000 || review.Prices[0].Proposed != 1200 {
		t.Fatalf("incorrect change comparison: %+v %v", review, err)
	}
	firstReview := review.Revision
	id, err := media.PrepareUpload(ctx, f.ownerID, f.productID, fmt.Sprintf("fixture-review/%d", f.productID), "image/jpeg", 1200)
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/media/products/%d", id)
	if _, err := media.FinalizeMedia(ctx, f.ownerID, id, path, "image/jpeg", 1200); err != nil {
		t.Fatal(err)
	}
	if _, err := media.GetPublicMedia(ctx, id); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("proposed cover is publicly reachable: %v", err)
	}
	checkDecision := func(actor, revision int64, want error) {
		t.Helper()
		if err := catalog.ReviewListingChange(ctx, actor, f.productID, revision, true, "Verified listing changes"); !errors.Is(err, want) {
			t.Fatalf("review returned %v, want %v", err, want)
		}
	}
	checkDecision(f.userIDs[0], firstReview, ports.ErrInvalidState)
	review, err = catalog.ListingChange(ctx, f.userIDs[0], f.productID)
	if err != nil || review.ProposedCover != path || len(review.Media) != 1 || len(review.Prices) != 1 {
		t.Fatalf("approval lost a proposed change: %+v %v", review, err)
	}
	checkDecision(f.userIDs[1], review.Revision, ports.ErrForbidden)
	checkDecision(f.userIDs[0], review.Revision, nil)
	saved, err = catalog.GetSellerProduct(ctx, f.ownerID, f.productID)
	if err != nil || saved.Status != "approved" || saved.PriceCents != 1200 {
		t.Fatalf("review did not apply price: %+v %v", saved, err)
	}
	if _, err := media.GetPublicMedia(ctx, id); err != nil {
		t.Fatal(err)
	}
	record, err := media.ClaimOwnedMedia(ctx, f.ownerID, id)
	if err != nil || record.Status != ports.MediaRemovalInReview {
		t.Fatalf("unexpected removal claim: %+v %v", record, err)
	}
	if _, err := media.GetPublicMedia(ctx, id); err != nil {
		t.Fatal(err)
	}
	review, err = catalog.ListingChange(ctx, f.userIDs[0], f.productID)
	if err != nil {
		t.Fatal(err)
	}
	checkDecision(f.userIDs[0], review.Revision, nil)
	if _, err := media.GetPublicMedia(ctx, id); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("approved removal is still publicly reachable: %v", err)
	}
}
