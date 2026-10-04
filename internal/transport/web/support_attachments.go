package web

import (
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) supportAttachment(w http.ResponseWriter, r *http.Request, rawID string) {
	if h.supportAttachments == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	attachmentID, err := strconv.ParseInt(strings.TrimSpace(rawID), 10, 64)
	if err != nil || attachmentID <= 0 {
		h.renderError(w, http.StatusNotFound, "Attachment unavailable", "That support attachment could not be found.")
		return
	}
	body, info, err := h.supportAttachments.Open(r.Context(), session.UserID, attachmentID)
	if err != nil {
		h.renderError(w, http.StatusNotFound, "Attachment unavailable", "That support attachment could not be found.")
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.ByteSize, 10))
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method == http.MethodGet {
		_, _ = io.Copy(w, body)
	}
}
