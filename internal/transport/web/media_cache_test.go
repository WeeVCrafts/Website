package web

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type publicCacheRepository struct {
	coverRepository
	public bool
	checks int
}

func (r *publicCacheRepository) GetPublicMedia(context.Context, int64) (domaincommerce.MediaRecord, error) {
	r.checks++
	if !r.public {
		return domaincommerce.MediaRecord{}, ports.ErrNotFound
	}
	return r.record, nil
}

type countedMediaStorage struct {
	coverStorage
	opens int
}

func (s *countedMediaStorage) OpenObject(context.Context, string) (io.ReadCloser, ports.ObjectInfo, error) {
	s.opens++
	return io.NopCloser(bytes.NewReader(s.data)), s.info, nil
}

func cachePhotograph(t *testing.T, blue uint8) []byte {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, 1600, 1000))
	for y := 0; y < 1000; y++ {
		for x := 0; x < 1600; x++ {
			source.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), blue, 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func TestPublicPhotographCacheReauthorizesWithoutOpeningOriginal(t *testing.T) {
	const id = int64(991881)
	data := cachePhotograph(t, 32)
	repo := &publicCacheRepository{public: true, coverRepository: coverRepository{record: domaincommerce.MediaRecord{ID: id, ObjectKey: "products/7/original", ContentType: "image/jpeg", ByteSize: int64(len(data)), Status: "ready", Visibility: "public", CreatedAt: time.Now()}}}
	storage := &countedMediaStorage{coverStorage: coverStorage{data: data, info: ports.ObjectInfo{ContentType: "image/jpeg", ByteSize: int64(len(data))}}}
	handler := &Handler{mediaService: applicationcommerce.NewMediaService(storage, repo)}
	request := func(method string, etags ...string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		r := httptest.NewRequest(method, "/media/products/991881?w=320", nil)
		if len(etags) > 0 {
			r.Header.Set("If-None-Match", etags[0])
		}
		handler.publicMedia(response, r, strconv.FormatInt(id, 10))
		return response
	}
	first := request(http.MethodGet)
	if first.Code != http.StatusOK || storage.opens != 1 || first.Body.Len() >= len(data) {
		t.Fatalf("first derivative: status=%d opens=%d bytes=%d", first.Code, storage.opens, first.Body.Len())
	}
	repeat := request(http.MethodGet)
	head := request(http.MethodHead)
	if repeat.Code != http.StatusOK || !bytes.Equal(repeat.Body.Bytes(), first.Body.Bytes()) || head.Body.Len() != 0 || storage.opens != 1 || repo.checks != 3 {
		t.Fatalf("hot derivative must reauthorize with zero original opens: opens=%d checks=%d statuses=%d/%d", storage.opens, repo.checks, repeat.Code, head.Code)
	}
	// An unpublished/private/deleted DB record must deny even a warm cache.
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("public photograph requires a content validator")
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := request(method, etag)
		if response.Code != http.StatusNotModified || response.Body.Len() != 0 || response.Header().Get("Cache-Control") != "public, max-age=300" || storage.opens != 1 {
			t.Fatalf("conditional %s must revalidate without bytes or storage opens: %d", method, response.Code)
		}
	}
	repo.public = false
	denied := request(http.MethodGet, etag)
	if denied.Code != http.StatusNotFound || storage.opens != 1 || bytes.Equal(denied.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("hidden media used a public derivative: status=%d opens=%d", denied.Code, storage.opens)
	}
	// A replaced immutable object identity cannot reuse the prior derivative.
	repo.public = true
	storage.data = cachePhotograph(t, 220)
	storage.info.ByteSize = int64(len(storage.data))
	repo.record.ObjectKey = "products/7/replacement"
	repo.record.ByteSize = storage.info.ByteSize
	replaced := request(http.MethodGet, etag)
	if replaced.Code != http.StatusOK || storage.opens != 2 || bytes.Equal(replaced.Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("replacement reused stale derivative: status=%d opens=%d", replaced.Code, storage.opens)
	}
	if replaced.Header().Get("ETag") == etag {
		t.Fatal("replacement bytes reused the previous validator")
	}
	// Changed DB metadata must miss the cache and retain storage verification.
	repo.record.ContentType = "image/png"
	mismatch := request(http.MethodGet)
	if mismatch.Code != http.StatusServiceUnavailable || storage.opens != 3 || mismatch.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("metadata mismatch reused cached content: status=%d opens=%d", mismatch.Code, storage.opens)
	}
	// Private previews remain on their own original-content path.
	private := httptest.NewRecorder()
	handler.serveMedia(private, httptest.NewRequest(http.MethodGet, "/media/products/991881/preview?w=320", nil), id, io.NopCloser(bytes.NewReader(storage.data)), storage.info, false)
	if private.Code != http.StatusOK || !bytes.Equal(private.Body.Bytes(), storage.data) || private.Header().Get("Cache-Control") != "private, no-store" || private.Header().Get("ETag") != "" {
		t.Fatal("private preview used a public derivative")
	}
	t.Logf("cold derivative opens=1; repeat GET + HEAD additional opens=0; authorization checks=%d; replacement additional opens=1; changed metadata reverified and denied", repo.checks)
}

func TestPublicOriginalPhotographConditionalDelivery(t *testing.T) {
	data := cachePhotograph(t, 91)
	h := &Handler{}
	serve := func(method, validator string, public bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/media/products/998771", nil)
		r.Header.Set("If-None-Match", validator)
		w := httptest.NewRecorder()
		h.serveMedia(w, r, 998771, io.NopCloser(bytes.NewReader(data)), ports.ObjectInfo{ContentType: "image/jpeg", ByteSize: int64(len(data))}, public)
		return w
	}
	original := serve(http.MethodGet, "", true)
	if original.Code != http.StatusOK || original.Header().Get("ETag") == "" {
		t.Fatal("original photograph requires a validator")
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := serve(method, original.Header().Get("ETag"), true)
		if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
			t.Fatalf("conditional original %s: %d", method, response.Code)
		}
	}
	private := serve(http.MethodGet, original.Header().Get("ETag"), false)
	if private.Code != http.StatusOK || private.Header().Get("ETag") != "" || private.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("private previews must not reuse public validators")
	}
}
