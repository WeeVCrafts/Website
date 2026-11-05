package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

func TestSettlementEvidenceKeysAndTypesStayPrivate(t *testing.T) {
	key := "finance/settlements/18/" + strings.Repeat("a", 48)
	if !validStoredObjectKey(key) || validProductObjectKey(key) {
		t.Fatal("finance evidence key not private")
	}
	for _, bad := range []string{"finance/settlements/0/" + strings.Repeat("a", 48), "finance/settlements/18/../file", "finance/settlements/18/" + strings.Repeat("a", 32), key + "/extra", key + ".pdf"} {
		if validStoredObjectKey(bad) {
			t.Fatalf("invalid finance key accepted: %s", bad)
		}
	}
	if !allowedStoredObjectType(key, "application/pdf", 100) || !allowedStoredObjectType(key, "image/png", 100) || allowedStoredObjectType(key, "image/webp", 100) || allowedStoredObjectType(key, "application/pdf", (5<<20)+1) {
		t.Fatal("finance types or size boundary wrong")
	}
	if allowedStoredObjectType("products/18/cover", "application/pdf", 100) || allowedStoredMediaType("application/pdf") {
		t.Fatal("PDF allowed in public media namespace")
	}
}

func TestUnavailableSettlementStorageFailsClosed(t *testing.T) {
	var storage *S3
	key := "finance/settlements/18/" + strings.Repeat("a", 48)
	if err := storage.PutObject(context.Background(), key, "application/pdf", 4, bytes.NewReader([]byte("test"))); err == nil {
		t.Fatal("nil storage accepted evidence")
	}
	if body, _, err := storage.OpenObject(context.Background(), key); err == nil {
		body.Close()
		t.Fatal("nil storage opened evidence")
	}
	if err := storage.DeleteObject(context.Background(), key); err == nil {
		t.Fatal("nil storage deleted evidence")
	}
}

func TestSettlementEvidenceAgainstS3CompatibleStorage(t *testing.T) {
	if os.Getenv("S3_INTEGRATION") != "1" {
		t.Skip("S3_INTEGRATION=1 required for isolated object-store integration")
	}
	storage, err := NewS3(envOrDefault("S3_ADDR", "127.0.0.1:3900"), envOrDefault("S3_ACCESS_KEY", "unused"), envOrDefault("S3_SECRET_KEY", "unused"), envOrDefault("S3_BUCKET", "finance-isolated-test"), false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err = storage.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("finance/settlements/18/%048x", time.Now().UnixNano())
	t.Cleanup(func() { _ = storage.DeleteObject(context.Background(), key) })
	payload := []byte("%PDF-1.7\nprivate bank evidence\n%%EOF")
	if err = storage.PutObject(ctx, key, "application/pdf", int64(len(payload)), bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	body, info, err := storage.OpenObject(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil || !bytes.Equal(actual, payload) || info.ContentType != "application/pdf" || info.ByteSize != int64(len(payload)) {
		t.Fatalf("private evidence roundtrip: info=%+v err=%v", info, err)
	}
	if _, err = storage.CreateDownloadURL(ctx, key, time.Minute); err == nil {
		t.Fatal("finance evidence received product presigned download")
	}
	if _, err = storage.CreateUploadURL(ctx, ports.UploadRequest{ObjectKey: key, ContentType: "application/pdf", MaxBytes: 100}); err == nil {
		t.Fatal("finance evidence received product presigned upload")
	}
	if err = storage.PutObject(ctx, "products/public-finance.pdf", "application/pdf", int64(len(payload)), bytes.NewReader(payload)); err == nil {
		t.Fatal("public PDF upload permitted")
	}
	if err = storage.DeleteObject(ctx, key); err != nil {
		t.Fatal(err)
	}
	if reader, _, err := storage.OpenObject(ctx, key); err == nil {
		reader.Close()
		t.Fatal("deleted finance evidence readable")
	}
}
