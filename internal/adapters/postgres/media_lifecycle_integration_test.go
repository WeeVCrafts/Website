package postgres

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestMediaLifecyclePersistsOnlyVerifiedReadyReferences(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 1, 1)
	if _, err := pool.Exec(context.Background(), `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller')`, fixture.ownerID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repository := NewMediaRepository(pool)

	mediaID, err := repository.PrepareUpload(ctx, fixture.ownerID, fixture.productID, "products/media-fixture/object", "image/png", 12)
	if err != nil {
		t.Fatalf("prepare media: %v", err)
	}
	var secondMediaID int64
	var videoMediaID int64
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM audit_logs WHERE action = 'catalog.media_finalized' AND resource_id = $1`, strconv.FormatInt(mediaID, 10))
		if secondMediaID > 0 {
			_, _ = pool.Exec(cleanupContext, `DELETE FROM audit_logs WHERE action = 'catalog.media_finalized' AND resource_id = $1`, strconv.FormatInt(secondMediaID, 10))
		}
		if videoMediaID > 0 {
			_, _ = pool.Exec(cleanupContext, `DELETE FROM audit_logs WHERE action = 'catalog.media_finalized' AND resource_id = $1`, strconv.FormatInt(videoMediaID, 10))
		}
	})
	if _, err := repository.GetOwnedMedia(ctx, fixture.ownerID, mediaID); err != nil {
		t.Fatalf("read owned pending media: %v", err)
	}
	if _, err := repository.GetOwnedMedia(ctx, fixture.userIDs[0]+1000, mediaID); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("expected cross-seller media denial, got %v", err)
	}

	ready, err := repository.FinalizeMedia(ctx, fixture.ownerID, mediaID, "/media/products/"+strconv.FormatInt(mediaID, 10), "image/png", 12)
	if err != nil {
		t.Fatalf("finalize media: %v", err)
	}
	if ready.Status != "ready" || ready.Visibility != "public" || ready.PublicPath == "" {
		t.Fatalf("unexpected ready media: %+v", ready)
	}
	public, err := repository.GetPublicMedia(ctx, mediaID)
	if err != nil || public.PublicPath != ready.PublicPath {
		t.Fatalf("read public media: record=%+v err=%v", public, err)
	}
	var productImage string
	if err := pool.QueryRow(ctx, `SELECT image_url FROM products WHERE id = $1`, fixture.productID).Scan(&productImage); err != nil {
		t.Fatalf("read product media reference: %v", err)
	}
	if productImage != ready.PublicPath {
		t.Fatalf("product image reference was not finalized: %q", productImage)
	}
	if _, err := pool.Exec(ctx, `UPDATE products SET status='draft' WHERE id=$1`, fixture.productID); err != nil {
		t.Fatalf("hide draft product media: %v", err)
	}
	if _, err := repository.GetPublicMedia(ctx, mediaID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("draft product media must not be public, got %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE products SET status='approved' WHERE id=$1`, fixture.productID); err != nil {
		t.Fatalf("republish fixture product: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sellers SET status='suspended' WHERE owner_user_id=$1`, fixture.ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetPublicMedia(ctx, mediaID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("suspended seller media must not be public: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sellers SET status='active' WHERE owner_user_id=$1`, fixture.ownerID); err != nil {
		t.Fatal(err)
	}
	var productSlug string
	if err := pool.QueryRow(ctx, `SELECT slug FROM products WHERE id = $1`, fixture.productID).Scan(&productSlug); err != nil {
		t.Fatalf("read product slug: %v", err)
	}
	catalog := NewProductRepository(pool)
	detail, err := catalog.GetApprovedBySlug(ctx, productSlug)
	if err != nil || detail.ImageURL != ready.PublicPath {
		t.Fatalf("detail page did not use latest image: image=%q err=%v", detail.ImageURL, err)
	}

	secondMediaID, err = repository.PrepareUpload(ctx, fixture.ownerID, fixture.productID, "products/media-fixture/latest-object", "image/png", 16)
	if err != nil {
		t.Fatalf("prepare latest media: %v", err)
	}
	latestPath := "/media/products/" + strconv.FormatInt(secondMediaID, 10)
	if _, err := repository.FinalizeMedia(ctx, fixture.ownerID, secondMediaID, latestPath, "image/png", 16); err != nil {
		t.Fatalf("finalize latest media: %v", err)
	}
	detail, err = catalog.GetApprovedBySlug(ctx, productSlug)
	if err != nil || detail.ImageURL != latestPath {
		t.Fatalf("detail page did not select newest image: image=%q want=%q err=%v", detail.ImageURL, latestPath, err)
	}

	videoMediaID, err = repository.PrepareUpload(ctx, fixture.ownerID, fixture.productID, "products/media-fixture/video-object", "video/mp4", 20)
	if err != nil {
		t.Fatalf("prepare video media: %v", err)
	}
	videoPath := "/media/products/" + strconv.FormatInt(videoMediaID, 10)
	if _, err := repository.FinalizeMedia(ctx, fixture.ownerID, videoMediaID, videoPath, "video/mp4", 20); err != nil {
		t.Fatalf("finalize video media: %v", err)
	}
	detail, err = catalog.GetApprovedBySlug(ctx, productSlug)
	if err != nil || detail.ImageURL != latestPath {
		t.Fatalf("newer video replaced primary image: image=%q want=%q err=%v", detail.ImageURL, latestPath, err)
	}
	claimedVideo, err := repository.ClaimOwnedMedia(ctx, fixture.ownerID, videoMediaID)
	if err != nil || claimedVideo.Status != "deleting" {
		t.Fatalf("claim video deletion: record=%+v err=%v", claimedVideo, err)
	}
	if err := repository.MarkMediaDeleted(ctx, videoMediaID); err != nil {
		t.Fatalf("mark video deleted: %v", err)
	}
	// Editing metadata/cover must not rewrite every ready gallery object's URL.
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'admin')`, fixture.userIDs[0]); err != nil {
		t.Fatal(err)
	}
	adminCatalog := NewCommerceRepository(pool)
	var sku string
	if err := pool.QueryRow(ctx, `SELECT sku FROM product_variants WHERE id=$1`, fixture.variantID).Scan(&sku); err != nil {
		t.Fatal(err)
	}
	_, err = adminCatalog.UpdateAdminProduct(ctx, fixture.userIDs[0], fixture.productID, domaincommerce.ProductDraftInput{Slug: productSlug, Name: "Fixture Product", Description: "Updated metadata", BrandSlug: fixture.brandSlug, BrandName: "Fixture Brand", CategorySlug: fixture.category, CategoryName: "Fixture Category", PriceCents: 1000, SKU: sku, ImageURL: latestPath})
	if err != nil {
		t.Fatalf("edit product with persistent cover: %v", err)
	}
	prior, err := repository.GetPublicMedia(ctx, mediaID)
	if err != nil || prior.PublicPath != ready.PublicPath {
		t.Fatalf("editing product corrupted earlier gallery URL: %+v %v", prior, err)
	}

	replayed, err := repository.FinalizeMedia(ctx, fixture.ownerID, mediaID, "/media/products/ignored", "image/png", 12)
	if err != nil || replayed.PublicPath != ready.PublicPath {
		t.Fatalf("finalize replay changed media: record=%+v err=%v", replayed, err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action = 'catalog.media_finalized' AND resource_id = $1`, strconv.FormatInt(mediaID, 10)).Scan(&auditCount); err != nil {
		t.Fatalf("count media audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected one media finalization audit, got %d", auditCount)
	}
	claimed, err := repository.ClaimOwnedMedia(ctx, fixture.ownerID, secondMediaID)
	if err != nil || claimed.Status != "deleting" {
		t.Fatalf("claim latest media deletion: record=%+v err=%v", claimed, err)
	}
	if err := repository.MarkMediaDeleted(ctx, secondMediaID); err != nil {
		t.Fatalf("mark latest media deleted: %v", err)
	}
	if _, err := repository.GetPublicMedia(ctx, secondMediaID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("deleted latest media remained public: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT image_url FROM products WHERE id = $1`, fixture.productID).Scan(&productImage); err != nil {
		t.Fatalf("read fallback product media reference: %v", err)
	}
	if productImage != ready.PublicPath {
		t.Fatalf("product image did not fall back to prior ready image: %q", productImage)
	}
	detail, err = catalog.GetApprovedBySlug(ctx, productSlug)
	if err != nil || detail.ImageURL != ready.PublicPath {
		t.Fatalf("detail page did not fall back to prior image: image=%q err=%v", detail.ImageURL, err)
	}
	claimed, err = repository.ClaimOwnedMedia(ctx, fixture.ownerID, mediaID)
	if err != nil || claimed.Status != "deleting" {
		t.Fatalf("claim remaining media deletion: record=%+v err=%v", claimed, err)
	}
	if err := repository.MarkMediaDeleted(ctx, mediaID); err != nil {
		t.Fatalf("mark remaining media deleted: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT image_url FROM products WHERE id = $1`, fixture.productID).Scan(&productImage); err != nil {
		t.Fatalf("read empty product media reference: %v", err)
	}
	if productImage != "" {
		t.Fatalf("deleted media reference remained on product: %q", productImage)
	}
}
