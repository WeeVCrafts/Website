package web

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type coverRepository struct {
	ports.MediaLifecycleRepository
	record domaincommerce.MediaRecord
}

func (r *coverRepository) PrepareUpload(_ context.Context, actor, product int64, key, kind string, size int64) (int64, error) {
	r.record = domaincommerce.MediaRecord{ID: 1, ProductID: product, ObjectKey: key, ContentType: kind, ByteSize: size, Status: "pending"}
	return 1, nil
}
func (r *coverRepository) GetOwnedMedia(_ context.Context, actor, id int64) (domaincommerce.MediaRecord, error) {
	if actor != 7 {
		return domaincommerce.MediaRecord{}, ports.ErrForbidden
	}
	return r.record, nil
}
func (r *coverRepository) GetPublicMedia(context.Context, int64) (domaincommerce.MediaRecord, error) {
	return r.record, nil
}
func (r *coverRepository) FinalizeMedia(_ context.Context, actor, id int64, path, kind string, size int64) (domaincommerce.MediaRecord, error) {
	r.record.Status, r.record.PublicPath = "ready", path
	return r.record, nil
}

type coverStorage struct {
	ports.ObjectStorage
	ports.ObjectStorageReader
	data []byte
	info ports.ObjectInfo
	fail bool
}

func (s *coverStorage) PutObject(_ context.Context, key, kind string, size int64, body io.Reader) error {
	if s.fail {
		return errors.New("bucket unavailable")
	}
	data, err := io.ReadAll(body)
	s.data, s.info = data, ports.ObjectInfo{ContentType: kind, ByteSize: size}
	return err
}
func (s *coverStorage) InspectObject(context.Context, string) (ports.ObjectInfo, error) {
	return s.info, nil
}
func (s *coverStorage) OpenObject(context.Context, string) (io.ReadCloser, ports.ObjectInfo, error) {
	return io.NopCloser(bytes.NewReader(s.data)), s.info, nil
}

func coverForm(t *testing.T, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("image_url", "/media/products/9/preview")
	file, _ := w.CreateFormFile("image_file", "cover.png")
	_, _ = file.Write(data)
	_ = w.Close()
	r := httptest.NewRequest("POST", "/seller/products/1/edit", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	t.Cleanup(func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	})
	return r
}

func TestProductCoverSurvivesHandlerRestartWithoutLocalFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	data := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	repo, storage := &coverRepository{}, &coverStorage{}
	h := &Handler{mediaService: applicationcommerce.NewMediaService(storage, repo)}
	r := coverForm(t, data)
	imageURL, parsed, _, err := readProductImage(r)
	if err != nil || imageURL != "/media/products/9" || !bytes.Equal(parsed, data) {
		t.Fatalf("parse cover: %s %v", imageURL, err)
	}
	if err := h.saveProductCover(r, 7, 21); err != nil {
		t.Fatal(err)
	}
	if repo.record.Status != "ready" || repo.record.PublicPath != "/media/products/1" || repo.record.ProductID != 21 {
		t.Fatalf("cover not finalized: %+v", repo.record)
	}
	if _, err := os.Stat(productUploadDirectory); !os.IsNotExist(err) {
		t.Fatalf("cover wrote ephemeral files: %v", err)
	}
	// A fresh service/handler uses the same durable repository and bucket.
	restarted := &Handler{mediaService: applicationcommerce.NewMediaService(storage, repo)}
	w := httptest.NewRecorder()
	restarted.publicMedia(w, httptest.NewRequest("GET", "/media/products/1", nil), "1")
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), data) {
		t.Fatalf("customer could not read cover after restart: %d", w.Code)
	}
	if _, _, err := restarted.mediaService.OpenOwnedMedia(context.Background(), 8, 1); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("cross-seller preview allowed: %v", err)
	}
}

func TestProductCoverStorageFailureNeverPublishes(t *testing.T) {
	repo, storage := &coverRepository{}, &coverStorage{fail: true}
	h := &Handler{mediaService: applicationcommerce.NewMediaService(storage, repo)}
	r := coverForm(t, append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...))
	if err := h.saveProductCover(r, 7, 21); err == nil {
		t.Fatal("storage failure was hidden")
	}
	if repo.record.Status != "pending" || repo.record.PublicPath != "" {
		t.Fatal("failed image was published")
	}
	if err := (&Handler{}).saveProductCover(r, 7, 21); err == nil {
		t.Fatal("missing persistent storage was ignored")
	}
}

func TestProductCoverWithoutUploadNormalizesPreview(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader("image_url=/media/products/9/preview"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	url, data, _, err := readProductImage(r)
	if err != nil || url != "/media/products/9" || len(data) != 0 {
		t.Fatalf("unexpected unchanged cover: %q %v", url, err)
	}
	if err := (&Handler{}).saveProductCover(r, 7, 21); err != nil {
		t.Fatalf("metadata-only save requires storage: %v", err)
	}
}
