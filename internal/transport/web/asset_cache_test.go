package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	applicationcatalog "github.com/wecratfs/commerce/internal/application/catalog"
	previewassets "github.com/wecratfs/commerce/web/assets"
)

func TestAssetCachingRequiresCurrentContentVersion(t *testing.T) {
	h, err := NewHandler(applicationcatalog.NewService(repository{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, cache string
		status      int
	}{
		{"/assets/js/htmx.min.js", "public, no-cache", 200},
		{"/assets/fonts/inter-latin.woff2", "public, no-cache", 200},
		{"/assets/js/htmx.min.js?v=old", "public, no-cache", 200},
		{previewassets.URL("js/htmx.min.js"), "public, max-age=31536000, immutable", 200},
		{"/static/app.css", "public, no-cache", 200},
		{"/assets/does-not-exist.js?v=old", "no-store", 404},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != tc.status || w.Header().Get("Cache-Control") != tc.cache {
			t.Errorf("%s: status=%d cache=%q", tc.path, w.Code, w.Header().Get("Cache-Control"))
		}
	}
}
