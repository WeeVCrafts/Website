package handlers

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

type deliveryReader struct {
	failOriginal bool
	keys         []string
}

func (f *deliveryReader) OpenObject(_ context.Context, key string) (io.ReadCloser, ports.ObjectInfo, error) {
	f.keys = append(f.keys, key)
	if key == "variant" || f.failOriginal {
		return nil, ports.ObjectInfo{}, errors.New("unavailable")
	}
	return io.NopCloser(strings.NewReader("photograph")), ports.ObjectInfo{ByteSize: 10, ContentType: "image/jpeg"}, nil
}
func TestStorefrontPhotographDeliveryRecoveryAndCaching(t *testing.T) {
	for _, tc := range []struct {
		name         string
		fail, public bool
		method       string
		code         int
		cache        string
	}{{"variant recovery", false, true, "GET", 200, "public, max-age=31536000, immutable"}, {"storage outage", true, true, "GET", 503, "no-store"}, {"private preview", false, false, "GET", 200, "no-store"}, {"head", false, true, "HEAD", 200, "public, max-age=31536000, immutable"}} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &deliveryReader{failOriginal: tc.fail}
			w := httptest.NewRecorder()
			serveStorefrontPhotograph(w, httptest.NewRequest(tc.method, "/media/storefront/item-fixture?w=960", nil), reader, "item-fixture", "variant", "original", tc.public)
			if w.Code != tc.code || w.Header().Get("Cache-Control") != tc.cache {
				t.Fatalf("status=%d cache=%q", w.Code, w.Header().Get("Cache-Control"))
			}
			if strings.Join(reader.keys, ",") != "variant,original" {
				t.Fatalf("unexpected retries: %v", reader.keys)
			}
			if tc.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD returned a body")
			}
		})
	}
}
