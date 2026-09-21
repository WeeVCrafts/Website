package api

import (
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"

	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) MediaUploadURL(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if r.Method != http.MethodPost || !ok || h.media == nil || !h.validCSRF(r, session) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Authentication and CSRF validation are required.")
		return
	}
	var input struct {
		ProductID   int64  `json:"product_id"`
		ContentType string `json:"content_type"`
		ByteSize    int64  `json:"byte_size"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	media, err := h.media.PrepareUpload(r.Context(), session.UserID, input.ProductID, input.ContentType, input.ByteSize)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "MEDIA_UPLOAD_INVALID", "Media upload request is invalid.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": media})
}

func (h *Handler) MediaUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use PUT to upload product media.")
		return
	}
	session, ok := h.session(r)
	if !ok || h.media == nil || !h.validCSRF(r, session) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Authentication and CSRF validation are required.")
		return
	}
	mediaIDText := strings.TrimPrefix(r.URL.Path, "/api/v1/seller/products/media-upload/")
	mediaID, err := strconv.ParseInt(mediaIDText, 10, 64)
	if err != nil || mediaID <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "MEDIA_INVALID", "A valid media upload is required.")
		return
	}
	if r.ContentLength <= 0 || r.ContentLength > 25<<20 {
		writeError(w, http.StatusRequestEntityTooLarge, "MEDIA_TOO_LARGE", "Product media must be smaller than 25 MB.")
		return
	}
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		writeError(w, http.StatusUnsupportedMediaType, "MEDIA_TYPE_UNSUPPORTED", "Choose a JPEG, PNG, WebP, MP4 or WebM file.")
		return
	}
	body := http.MaxBytesReader(w, r.Body, 25<<20)
	if err := h.media.UploadObject(r.Context(), session.UserID, mediaID, contentType, r.ContentLength, body); err != nil {
		h.writeMediaError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"uploaded": true}})
}

func (h *Handler) MediaFinalize(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if r.Method != http.MethodPost || !ok || h.media == nil || !h.validCSRF(r, session) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Authentication and CSRF validation are required.")
		return
	}
	var input struct {
		MediaID int64 `json:"media_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	media, err := h.media.FinalizeUpload(r.Context(), session.UserID, input.MediaID)
	if err != nil {
		h.writeMediaError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": media})
}

func (h *Handler) MediaDelete(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if r.Method != http.MethodPost || !ok || h.media == nil || !h.validCSRF(r, session) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Authentication and CSRF validation are required.")
		return
	}
	var input struct {
		MediaID int64 `json:"media_id"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := h.media.DeleteUpload(r.Context(), session.UserID, input.MediaID); err != nil {
		h.writeMediaError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"status": "deleted"}})
}

func (h *Handler) writeMediaError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ports.ErrForbidden):
		writeError(w, http.StatusForbidden, "FORBIDDEN", "This media does not belong to the authenticated seller.")
	case errors.Is(err, applicationcommerce.ErrMediaNotReady):
		writeError(w, http.StatusConflict, "MEDIA_NOT_READY", "The media upload is not in a finalizable state.")
	case errors.Is(err, ports.ErrInvalidState):
		writeError(w, http.StatusConflict, "MEDIA_NOT_READY", "The media upload is not in a finalizable state.")
	case errors.Is(err, applicationcommerce.ErrInvalidMedia):
		writeError(w, http.StatusUnprocessableEntity, "MEDIA_INVALID", "The stored object did not match the validated upload.")
	default:
		writeError(w, http.StatusInternalServerError, "MEDIA_UNAVAILABLE", "Media lifecycle processing failed.")
	}
}
