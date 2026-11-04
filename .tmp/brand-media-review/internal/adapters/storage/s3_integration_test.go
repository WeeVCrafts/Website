package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

func TestS3MediaLifecycleAgainstS3CompatibleStorage(t *testing.T) {
	if os.Getenv("S3_INTEGRATION") != "1" {
		t.Skip("S3_INTEGRATION=1 is required for S3-compatible integration tests")
	}
	endpoint := envOrDefault("S3_ADDR", "localhost:3900")
	accessKey := envOrDefault("S3_ACCESS_KEY", "GK0123456789abcdef01234567")
	secretKey := envOrDefault("S3_SECRET_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	bucket := envOrDefault("S3_BUCKET", "wecratfs-media")
	storage, err := NewS3(endpoint, accessKey, secretKey, bucket, false)
	if err != nil {
		t.Fatalf("create S3-compatible storage adapter: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := storage.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure S3-compatible bucket: %v", err)
	}
	key := fmt.Sprintf("products/integration/%d.png", time.Now().UnixNano())
	t.Cleanup(func() { _ = storage.DeleteObject(context.Background(), key) })
	payload := []byte("valid media fixture")
	if _, err := storage.client.PutObject(ctx, storage.bucket, key, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{ContentType: "image/png"}); err != nil {
		t.Fatalf("put S3-compatible fixture: %v", err)
	}

	info, err := storage.InspectObject(ctx, key)
	if err != nil || info.ByteSize != int64(len(payload)) || info.ContentType != "image/png" {
		t.Fatalf("unexpected object info: %+v err=%v", info, err)
	}
	download, err := storage.CreateDownloadURL(ctx, key, 5*time.Minute)
	if err != nil || !ValidateUploadURL(download.URL) {
		t.Fatalf("create signed download URL: %+v err=%v", download, err)
	}
	response, err := http.Get(download.URL)
	if err != nil {
		t.Fatalf("read signed download URL: %v", err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || !bytes.Equal(body, payload) {
		t.Fatalf("signed download failed: status=%d err=%v body=%q", response.StatusCode, readErr, body)
	}
	if err := storage.DeleteObject(ctx, key); err != nil {
		t.Fatalf("delete S3-compatible fixture: %v", err)
	}
	if _, err := storage.InspectObject(ctx, key); err == nil {
		t.Fatal("deleted object remained readable")
	}

	supportKey := fmt.Sprintf("support/340/%032x", time.Now().UnixNano())
	t.Cleanup(func() { _ = storage.DeleteObject(context.Background(), supportKey) })
	if err := storage.PutObject(ctx, supportKey, "image/png", int64(len(payload)), bytes.NewReader(payload)); err != nil {
		t.Fatalf("put private support attachment: %v", err)
	}
	privateBody, privateInfo, err := storage.OpenObject(ctx, supportKey)
	if err != nil {
		t.Fatalf("open private support attachment: %v", err)
	}
	privateBytes, err := io.ReadAll(privateBody)
	_ = privateBody.Close()
	if err != nil || !bytes.Equal(privateBytes, payload) || privateInfo.ByteSize != int64(len(payload)) {
		t.Fatalf("private attachment round trip: info=%+v err=%v", privateInfo, err)
	}
	if _, err := storage.CreateDownloadURL(ctx, supportKey, time.Minute); err == nil {
		t.Fatal("private support attachment must not use product presigned endpoints")
	}

}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
