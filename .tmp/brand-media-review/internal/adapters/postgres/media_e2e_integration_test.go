package postgres

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	storageadapter "github.com/wecratfs/commerce/internal/adapters/storage"
	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
)

func TestMediaUploadFinalizeAndPublicReadAgainstPostgresAndS3CompatibleStorage(t *testing.T) {
	if os.Getenv("S3_INTEGRATION") != "1" {
		t.Skip("S3_INTEGRATION=1 is required for PostgreSQL and S3-compatible media integration")
	}
	pool := openIntegrationPool(t)
	fixture := newCommerceFixture(t, pool, 1, 1)
	if _, err := pool.Exec(context.Background(), `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'seller')`, fixture.ownerID); err != nil {
		t.Fatal(err)
	}
	storage, err := storageadapter.NewS3(envOrDefault("S3_ADDR", "localhost:3900"), envOrDefault("S3_ACCESS_KEY", "GK0123456789abcdef01234567"), envOrDefault("S3_SECRET_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"), envOrDefault("S3_BUCKET", "wecratfs-media"), false)
	if err != nil {
		t.Fatalf("create storage adapter: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := storage.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure storage bucket: %v", err)
	}
	repository := NewMediaRepository(pool)
	service := applicationcommerce.NewMediaService(storage, repository)
	payload := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}, []byte("real local media bytes")...)
	upload, err := service.PrepareUpload(ctx, fixture.ownerID, fixture.productID, "image/png", int64(len(payload)))
	if err != nil {
		t.Fatalf("prepare upload: %v", err)
	}
	t.Cleanup(func() { _ = storage.DeleteObject(context.Background(), upload.ObjectKey) })
	if err := storage.PutObject(ctx, upload.ObjectKey, "image/png", int64(len(payload)), bytes.NewReader(payload)); err != nil {
		t.Fatalf("put media object into S3-compatible storage: %v", err)
	}
	ready, err := service.FinalizeUpload(ctx, fixture.ownerID, upload.ID)
	if err != nil {
		t.Fatalf("finalize uploaded media: %v", err)
	}
	if ready.Status != "ready" || ready.PublicPath != "/media/products/"+strconv.FormatInt(upload.ID, 10) {
		t.Fatalf("unexpected finalized media: %+v", ready)
	}
	publicURL, err := service.ResolvePublicURL(ctx, upload.ID)
	if err != nil {
		t.Fatalf("resolve public media: %v", err)
	}
	publicResponse, err := http.Get(publicURL)
	if err != nil {
		t.Fatalf("read public media: %v", err)
	}
	readBack, readErr := io.ReadAll(publicResponse.Body)
	_ = publicResponse.Body.Close()
	if readErr != nil || publicResponse.StatusCode != http.StatusOK || !bytes.Equal(readBack, payload) {
		t.Fatalf("public media read failed: status=%d err=%v body=%q", publicResponse.StatusCode, readErr, readBack)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
