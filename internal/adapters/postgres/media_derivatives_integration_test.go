package postgres

import (
	"context"
	"testing"
	"time"

	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestPreparedMediaBindsDerivativeToCurrentSource(t *testing.T) {
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 1, 1)
	ctx := t.Context()
	var mediaID, seller int64
	if err := pool.QueryRow(ctx, `SELECT seller_id FROM products WHERE id=$1`, fixture.productID).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	source := domain.MediaRecord{ProductID: fixture.productID, SellerID: seller, ObjectKey: "products/derivative-fixture", ContentType: "image/jpeg", ByteSize: 1000, CreatedAt: time.Now()}
	err := pool.QueryRow(ctx, `INSERT INTO product_media(product_id,seller_id,object_key,content_type,byte_size,status,visibility,public_url) VALUES($1,$2,$3,$4,$5,'ready','public','/media/products/fixture') RETURNING id`, source.ProductID, source.SellerID, source.ObjectKey, source.ContentType, source.ByteSize).Scan(&mediaID)
	if err != nil {
		t.Fatal(err)
	}
	source.ID = mediaID
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM product_media WHERE id=$1`, mediaID) })
	repository := NewMediaRepository(pool)
	rows, err := repository.DerivativeCandidates(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.ID == mediaID {
			found = true
		}
	}
	if !found {
		t.Fatal("existing public image excluded from backfill")
	}
	d := ports.MediaDerivative{MediaID: mediaID, Width: 640, SourceKey: source.ObjectKey, SourceType: source.ContentType, SourceBytes: 1000, ObjectKey: "products/derived/fixture/640", ContentType: "image/jpeg", ByteSize: 400}
	if err := repository.SaveDerivative(ctx, d); err != nil {
		t.Fatal(err)
	}
	saved, err := repository.PublicDerivative(ctx, source, 640)
	if err != nil || saved.ObjectKey != d.ObjectKey || saved.ByteSize != 400 {
		t.Fatalf("derivative %+v err=%v", saved, err)
	}
	source.ObjectKey = "products/replaced"
	if _, err := repository.PublicDerivative(ctx, source, 640); err == nil {
		t.Fatal("replacement reused stale derivative")
	}
	if _, err := pool.Exec(ctx, `UPDATE product_media SET object_key=$2 WHERE id=$1`, mediaID, source.ObjectKey); err != nil {
		t.Fatal(err)
	}
	d.ObjectKey = "products/derived/stale/640"
	if err := repository.SaveDerivative(ctx, d); err != nil {
		t.Fatal(err)
	}
	var current string
	if err := pool.QueryRow(ctx, `SELECT object_key FROM product_image_derivatives WHERE media_id=$1 AND width=640`, mediaID).Scan(&current); err != nil || current != "products/derived/fixture/640" {
		t.Fatal("stale worker overwrote derivative")
	}
}
