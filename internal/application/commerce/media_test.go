package commerce

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type mediaLifecycleRepository struct {
	record       domaincommerce.MediaRecord
	ownerID      int64
	finalized    bool
	deleted      bool
	finalizePath string
}

func (r *mediaLifecycleRepository) PrepareUpload(context.Context, int64, int64, string, string, int64) (int64, error) {
	return r.record.ID, nil
}

func (r *mediaLifecycleRepository) GetOwnedMedia(_ context.Context, sellerOwnerID, _ int64) (domaincommerce.MediaRecord, error) {
	if r.ownerID > 0 && sellerOwnerID != r.ownerID {
		return domaincommerce.MediaRecord{}, ports.ErrForbidden
	}
	return r.record, nil
}

func (r *mediaLifecycleRepository) FinalizeMedia(_ context.Context, _ int64, _ int64, publicPath, contentType string, byteSize int64) (domaincommerce.MediaRecord, error) {
	r.record.Status = "ready"
	r.record.PublicPath = publicPath
	r.record.ContentType = contentType
	r.record.ByteSize = byteSize
	r.finalized = true
	r.finalizePath = publicPath
	return r.record, nil
}

func (r *mediaLifecycleRepository) GetPublicMedia(context.Context, int64) (domaincommerce.MediaRecord, error) {
	return r.record, nil
}

func (r *mediaLifecycleRepository) ClaimOwnedMedia(context.Context, int64, int64) (domaincommerce.MediaRecord, error) {
	r.record.Status = "deleting"
	return r.record, nil
}

func (r *mediaLifecycleRepository) MarkMediaDeleted(context.Context, int64) error {
	r.deleted = true
	r.record.Status = "deleted"
	return nil
}

func (r *mediaLifecycleRepository) RestoreMedia(context.Context, int64) error {
	r.record.Status = "pending"
	return nil
}

func (r *mediaLifecycleRepository) ClaimExpiredMedia(context.Context, time.Time, int) ([]domaincommerce.MediaRecord, error) {
	r.record.Status = "deleting"
	return []domaincommerce.MediaRecord{r.record}, nil
}

func (r *mediaLifecycleRepository) ListExpiredMedia(context.Context, time.Time, int) ([]domaincommerce.MediaRecord, error) {
	return []domaincommerce.MediaRecord{r.record}, nil
}

type mediaStorage struct {
	info       ports.ObjectInfo
	object     []byte
	deleted    bool
	inspected  int
	downloaded int
}

func (s *mediaStorage) CreateUploadURL(context.Context, ports.UploadRequest) (ports.UploadURL, error) {
	return ports.UploadURL{URL: "https://storage.invalid/upload"}, nil
}

func (s *mediaStorage) InspectObject(context.Context, string) (ports.ObjectInfo, error) {
	s.inspected++
	return s.info, nil
}

func (s *mediaStorage) CreateDownloadURL(context.Context, string, time.Duration) (ports.DownloadURL, error) {
	s.downloaded++
	return ports.DownloadURL{URL: "https://storage.invalid/read"}, nil
}

func (s *mediaStorage) PutObject(_ context.Context, _ string, contentType string, byteSize int64, body io.Reader) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.object = data
	s.info = ports.ObjectInfo{ContentType: contentType, ByteSize: byteSize}
	return nil
}

func (s *mediaStorage) OpenObject(context.Context, string) (io.ReadCloser, ports.ObjectInfo, error) {
	return io.NopCloser(bytes.NewReader(s.object)), s.info, nil
}

func (s *mediaStorage) DeleteObject(context.Context, string) error {
	s.deleted = true
	return nil
}

func TestMediaFinalizeVerifiesStoredObjectBeforePublishing(t *testing.T) {
	repository := &mediaLifecycleRepository{record: domaincommerce.MediaRecord{ID: 7, ObjectKey: "products/9/object", ContentType: "image/png", ByteSize: 12, Status: "pending"}}
	storage := &mediaStorage{info: ports.ObjectInfo{ContentType: "image/png", ByteSize: 12}, object: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}}
	service := NewMediaService(storage, repository)

	record, err := service.FinalizeUpload(context.Background(), 9, 7)
	if err != nil {
		t.Fatalf("finalize media: %v", err)
	}
	if !repository.finalized || record.Status != "ready" || record.PublicPath != "/media/products/7" {
		t.Fatalf("unexpected finalized media: %+v", record)
	}
	if storage.inspected != 1 {
		t.Fatalf("expected one object inspection, got %d", storage.inspected)
	}

	if _, err := service.FinalizeUpload(context.Background(), 9, 7); err != nil {
		t.Fatalf("ready media should be idempotent: %v", err)
	}
	if storage.inspected != 1 {
		t.Fatalf("ready replay re-inspected the object: %d", storage.inspected)
	}
}

func TestMediaFinalizeRejectsStoredContentTypeSpoofing(t *testing.T) {
	payload := []byte("%PDF-1.7 not a PNG")
	repository := &mediaLifecycleRepository{record: domaincommerce.MediaRecord{ID: 73, ObjectKey: "products/9/spoofed", ContentType: "image/png", ByteSize: int64(len(payload)), Status: "pending"}}
	storage := &mediaStorage{info: ports.ObjectInfo{ContentType: "image/png", ByteSize: int64(len(payload))}, object: payload}
	service := NewMediaService(storage, repository)

	if _, err := service.FinalizeUpload(context.Background(), 9, 73); !errors.Is(err, ErrInvalidMedia) {
		t.Fatalf("mislabeled stored object returned %v", err)
	}
	if repository.finalized {
		t.Fatal("mislabeled object was finalized")
	}
}

func TestMediaUploadUsesAuthenticatedPreparedRecordAndCanStreamPublicObject(t *testing.T) {
	payload := append([]byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'}, []byte("verified image bytes")...)
	repository := &mediaLifecycleRepository{record: domaincommerce.MediaRecord{ID: 70, ProductID: 9, ObjectKey: "products/9/image", ContentType: "image/webp", ByteSize: int64(len(payload)), Visibility: "public", Status: "pending"}, ownerID: 9}
	storage := &mediaStorage{}
	service := NewMediaService(storage, repository)
	if err := service.UploadObject(context.Background(), 9, 70, "image/webp", int64(len(payload)), bytes.NewReader(payload)); err != nil {
		t.Fatalf("upload prepared product image: %v", err)
	}
	if !bytes.Equal(storage.object, payload) || storage.info.ContentType != "image/webp" {
		t.Fatalf("uploaded bytes or type differ: bytes=%q info=%+v", storage.object, storage.info)
	}
	if err := service.UploadObject(context.Background(), 10, 70, "image/webp", int64(len(payload)), bytes.NewReader(payload)); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("unrelated seller uploaded media: %v", err)
	}
	if err := service.UploadObject(context.Background(), 9, 70, "image/png", int64(len(payload)), bytes.NewReader(payload)); !errors.Is(err, ErrInvalidMedia) {
		t.Fatalf("mismatched content type upload returned %v", err)
	}

	repository.record.Status = "ready"
	body, info, err := service.OpenPublicMedia(context.Background(), 70)
	if err != nil {
		t.Fatalf("open public product image: %v", err)
	}
	defer body.Close()
	readBack, err := io.ReadAll(body)
	if err != nil || !bytes.Equal(readBack, payload) || info.ContentType != "image/webp" {
		t.Fatalf("streamed public image mismatch: bytes=%q info=%+v err=%v", readBack, info, err)
	}
}

func TestMediaUploadAcceptsAllSupportedFileSignatures(t *testing.T) {
	cases := []struct {
		contentType string
		prefix      []byte
	}{
		{"image/jpeg", []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"image/png", []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}},
		{"image/webp", []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'}},
		{"video/mp4", []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}},
		{"video/webm", []byte{0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x86, 0x81, 0x01, 0x42, 0xf7, 0x81}},
	}
	for _, testCase := range cases {
		t.Run(testCase.contentType, func(t *testing.T) {
			payload := append(append([]byte(nil), testCase.prefix...), []byte("valid media payload")...)
			repository := &mediaLifecycleRepository{record: domaincommerce.MediaRecord{ID: 71, ProductID: 9, ObjectKey: "products/9/media", ContentType: testCase.contentType, ByteSize: int64(len(payload)), Status: "pending"}, ownerID: 9}
			storage := &mediaStorage{}
			service := NewMediaService(storage, repository)
			if err := service.UploadObject(context.Background(), 9, 71, testCase.contentType, int64(len(payload)), bytes.NewReader(payload)); err != nil {
				t.Fatalf("upload supported media: %v", err)
			}
			if !bytes.Equal(storage.object, payload) {
				t.Fatal("uploaded media bytes changed")
			}
		})
	}
}

func TestMediaUploadRejectsContentTypeSpoofing(t *testing.T) {
	payload := []byte("%PDF-1.7 not a PNG file")
	repository := &mediaLifecycleRepository{record: domaincommerce.MediaRecord{ID: 72, ProductID: 9, ObjectKey: "products/9/spoofed", ContentType: "image/png", ByteSize: int64(len(payload)), Status: "pending"}, ownerID: 9}
	storage := &mediaStorage{}
	service := NewMediaService(storage, repository)

	if err := service.UploadObject(context.Background(), 9, 72, "image/png", int64(len(payload)), bytes.NewReader(payload)); !errors.Is(err, ErrInvalidMedia) {
		t.Fatalf("mislabeled file returned %v", err)
	}
	if len(storage.object) != 0 {
		t.Fatal("mislabeled file was stored")
	}
}

func TestMediaFinalizeRejectsObjectMetadataMismatch(t *testing.T) {
	repository := &mediaLifecycleRepository{record: domaincommerce.MediaRecord{ID: 8, ObjectKey: "products/9/object", ContentType: "image/jpeg", ByteSize: 20, Status: "pending"}}
	storage := &mediaStorage{info: ports.ObjectInfo{ContentType: "image/png", ByteSize: 20}}
	service := NewMediaService(storage, repository)

	if _, err := service.FinalizeUpload(context.Background(), 9, 8); !errors.Is(err, ErrInvalidMedia) {
		t.Fatalf("expected metadata mismatch rejection, got %v", err)
	}
	if repository.finalized {
		t.Fatal("metadata mismatch finalized media")
	}
}

func TestMediaCleanupDeletesOnlyObjectsThatWereRemoved(t *testing.T) {
	repository := &mediaLifecycleRepository{record: domaincommerce.MediaRecord{ID: 9, ObjectKey: "products/9/object", Status: "pending"}}
	storage := &mediaStorage{}
	service := NewMediaService(storage, repository)

	deleted, err := service.CleanupExpired(context.Background(), 1)
	if err != nil || deleted != 1 || !storage.deleted || !repository.deleted {
		t.Fatalf("unexpected cleanup result: deleted=%d err=%v storage_deleted=%v repository_deleted=%v", deleted, err, storage.deleted, repository.deleted)
	}
}

func TestMediaDeleteClaimsReadyMediaBeforeRemovingObject(t *testing.T) {
	repository := &mediaLifecycleRepository{record: domaincommerce.MediaRecord{ID: 10, ObjectKey: "products/9/object", Status: "ready", PublicPath: "/media/products/10"}}
	storage := &mediaStorage{}
	service := NewMediaService(storage, repository)

	if err := service.DeleteUpload(context.Background(), 9, 10); err != nil {
		t.Fatalf("delete ready media: %v", err)
	}
	if !storage.deleted || !repository.deleted || repository.record.Status != "deleted" {
		t.Fatalf("ready media was not deleted safely: storage=%v repository=%v status=%s", storage.deleted, repository.deleted, repository.record.Status)
	}
}

func TestAllowedMediaTypeSupportsCustomerGalleryImagesAndVideos(t *testing.T) {
	for _, contentType := range []string{"image/jpeg", "image/png", "image/webp", "video/mp4", "video/webm", " VIDEO/WEBM "} {
		if !allowedMediaType(contentType) {
			t.Errorf("supported product media type %q was rejected", contentType)
		}
	}
	for _, contentType := range []string{"image/svg+xml", "video/quicktime", "application/octet-stream", "text/html"} {
		if allowedMediaType(contentType) {
			t.Errorf("unsupported product media type %q was accepted", contentType)
		}
	}
}
