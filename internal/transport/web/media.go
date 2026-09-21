package web

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) publicMedia(w http.ResponseWriter, r *http.Request, mediaID string) {
	if r.Method != http.MethodGet || h.mediaService == nil {
		h.notFound(w, r)
		return
	}
	id, err := strconv.ParseInt(mediaID, 10, 64)
	if err != nil || id <= 0 {
		h.notFound(w, r)
		return
	}
	body, info, err := h.mediaService.OpenPublicMedia(r.Context(), id)
	if errors.Is(err, ports.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		h.renderError(w, http.StatusServiceUnavailable, "Media unavailable", "This product image is temporarily unavailable.")
		return
	}
	defer body.Close()
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.ByteSize, 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	if _, err := io.Copy(w, body); err != nil {
		return
	}
}
