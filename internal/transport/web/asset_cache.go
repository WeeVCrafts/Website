package web

import (
	"io/fs"
	"net/http"
)

// prepareEmbeddedAsset only grants immutable caching to the exact URL generated
// from the current bytes. Mutable aliases and obsolete hashes must revalidate.
func prepareEmbeddedAsset(w http.ResponseWriter, r *http.Request, files fs.FS, name, versionedURL string) bool {
	info, err := fs.Stat(files, name)
	if err != nil || info.IsDir() {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return false
	}
	w.Header().Set("Cache-Control", "public, no-cache")
	if versionedURL != "" && r.URL.RawQuery != "" && r.URL.Path+"?"+r.URL.RawQuery == versionedURL {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	return true
}
