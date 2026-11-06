package web

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) publicMedia(w http.ResponseWriter, r *http.Request, mediaID string) {
	w.Header().Set("Cache-Control", "no-store")
	if (r.Method != http.MethodGet && r.Method != http.MethodHead) || h.mediaService == nil {
		h.notFound(w, r)
		return
	}
	id, err := strconv.ParseInt(mediaID, 10, 64)
	if err != nil || id <= 0 {
		h.notFound(w, r)
		return
	}
	media, err := h.mediaService.ResolvePublicMedia(r.Context(), id)
	if errors.Is(err, ports.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		h.renderError(w, http.StatusServiceUnavailable, "Media unavailable", "This product image is temporarily unavailable.")
		return
	}
	info := media.Info()
	if width := photographWidth(r, info, true); width != 0 {
		if data := cachedProductPhotograph(media.CacheKey(), width); data != nil {
			mediaHeaders(w, "image/jpeg", true)
			photographETag(w, data)
			http.ServeContent(w, r, "product-photograph", time.Time{}, bytes.NewReader(data))
			return
		}
	}
	body, info, err := media.Open(r.Context())
	if err != nil {
		h.renderError(w, http.StatusServiceUnavailable, "Media unavailable", "This product image is temporarily unavailable.")
		return
	}
	h.serveMedia(w, r, id, body, info, true, media.CacheKey())
}

func (h *Handler) ownedMediaPreview(w http.ResponseWriter, r *http.Request, mediaID string) {
	w.Header().Set("Cache-Control", "private, no-store")
	if (r.Method != http.MethodGet && r.Method != http.MethodHead) || h.mediaService == nil {
		h.notFound(w, r)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.notFound(w, r)
		return
	}
	id, err := strconv.ParseInt(mediaID, 10, 64)
	if err != nil || id <= 0 {
		h.notFound(w, r)
		return
	}
	body, info, err := h.mediaService.OpenOwnedMedia(r.Context(), session.UserID, id)
	if errors.Is(err, ports.ErrForbidden) || errors.Is(err, ports.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		h.notFound(w, r)
		return
	}
	h.serveMedia(w, r, id, body, info, false)
}

func (h *Handler) serveMedia(w http.ResponseWriter, r *http.Request, mediaID int64, body io.ReadCloser, info ports.ObjectInfo, public bool, identity ...string) {
	defer body.Close()
	w.Header().Set("Cache-Control", "no-store")
	if info.ByteSize <= 0 || info.ByteSize > 25<<20 {
		h.notFound(w, r)
		return
	}
	// S3 objects implement ReadSeeker. ServeContent requests only the byte range
	// needed by the browser, so a video preview never buffers a whole 25 MiB file.
	if strings.HasPrefix(info.ContentType, "video/") {
		if seeker, ok := body.(io.ReadSeeker); ok {
			mediaHeaders(w, info.ContentType, public)
			http.ServeContent(w, r, "product-media-"+strconv.FormatInt(mediaID, 10), time.Time{}, seeker)
			return
		}
	}
	key := strconv.FormatInt(mediaID, 10)
	if len(identity) > 0 {
		key = identity[0]
	}
	width := photographWidth(r, info, public)
	variant := width != 0
	if variant {
		if data := cachedProductPhotograph(key, width); data != nil {
			mediaHeaders(w, "image/jpeg", true)
			photographETag(w, data)
			http.ServeContent(w, r, "product-photograph", time.Time{}, bytes.NewReader(data))
			return
		}
	}
	data, err := io.ReadAll(io.LimitReader(body, info.ByteSize+1))
	if err != nil || int64(len(data)) != info.ByteSize {
		h.renderError(w, http.StatusServiceUnavailable, "Media unavailable", "This product media is temporarily unavailable.")
		return
	}
	if variant {
		if resized, err := productPhotographVariant(data, width); err == nil && len(resized) < len(data) {
			data = resized
			info.ContentType = "image/jpeg"
			rememberProductPhotograph(key, width, data)
		}
	}
	mediaHeaders(w, info.ContentType, public)
	if public && strings.HasPrefix(info.ContentType, "image/") {
		photographETag(w, data)
	}
	http.ServeContent(w, r, "product-media-"+strconv.FormatInt(mediaID, 10), time.Time{}, bytes.NewReader(data))
}

// Hash the delivered representation, so original and resized bytes never share
// a validator. ServeContent handles conditional GET, HEAD, and range requests.
func photographETag(w http.ResponseWriter, data []byte) {
	w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sha256.Sum256(data)))
}

func photographWidth(r *http.Request, info ports.ObjectInfo, public bool) int {
	width, _ := strconv.Atoi(r.URL.Query().Get("w"))
	if public && info.ByteSize > 0 && info.ByteSize <= 25<<20 && strings.HasPrefix(info.ContentType, "image/") && (width == 320 || width == 640 || width == 960) {
		return width
	}
	return 0
}

func mediaHeaders(w http.ResponseWriter, contentType string, public bool) {
	if public {
		w.Header().Set("Cache-Control", "public, max-age=300")
	} else {
		w.Header().Set("Cache-Control", "private, no-store")
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Accept-Ranges", "bytes")
}
