package handlers

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/wecratfs/commerce/internal/ports"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
)

func (h *MarketplaceHandler) uploadClaimEvidence(ctx context.Context, actor, claim int64, header *multipart.FileHeader) error {
	writer, ok := h.Storage.(ports.ObjectStorageWriter)
	if !ok || header == nil || header.Size <= 0 || header.Size > 200<<20 {
		return ports.ErrInvalidState
	}
	if err := h.Repository.AuthorizeClaimEvidence(ctx, actor, claim, true); err != nil {
		return err
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 4096)
	sample, _ := reader.Peek(int(min(header.Size, 4096)))
	kind := http.DetectContentType(sample)
	if len(sample) >= 12 && string(sample[4:8]) == "ftyp" {
		kind = "video/mp4"
		if string(sample[8:12]) == "qt  " {
			kind = "video/quicktime"
		}
	}
	if len(sample) >= 4 && string(sample[:4]) == "\x1a\x45\xdf\xa3" && strings.Contains(string(sample), "webm") {
		kind = "video/webm"
	}
	if len(sample) >= 12 && string(sample[:4]) == "RIFF" && string(sample[8:12]) == "WEBP" {
		kind = "image/webp"
	}
	if kind != "video/mp4" && kind != "video/quicktime" && kind != "video/webm" && kind != "image/jpeg" && kind != "image/png" && kind != "image/webp" {
		return ports.ErrInvalidState
	}
	if strings.HasPrefix(kind, "image/") && header.Size > 10<<20 {
		return ports.ErrInvalidState
	}
	var nonce [24]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return err
	}
	key := "private/claim-evidence/" + hex.EncodeToString(nonce[:])
	if err = writer.PutObject(ctx, key, kind, header.Size, io.LimitReader(reader, header.Size)); err != nil {
		return err
	}
	err = h.Repository.SaveClaimEvidence(ctx, actor, claim, key, filepath.Base(header.Filename), kind, header.Size)
	if err != nil {
		if cleaner, ok := h.Storage.(ports.ObjectStorageReader); ok {
			_ = cleaner.DeleteObject(ctx, key)
		}
	}
	return err
}

func (h *MarketplaceHandler) serveClaimEvidence(w http.ResponseWriter, r *http.Request, actor, id int64) {
	key, name, kind, err := h.Repository.ClaimEvidenceObject(r.Context(), actor, id)
	if err != nil {
		marketplaceHTTPError(w, err)
		return
	}
	reader, ok := h.Storage.(ports.ObjectStorageContentReader)
	if !ok {
		http.Error(w, "Evidence storage unavailable", 503)
		return
	}
	body, _, err := reader.OpenObject(r.Context(), key)
	if err != nil {
		http.Error(w, "Evidence temporarily unavailable", 503)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", kind)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	if r.Method != "HEAD" {
		_, _ = io.Copy(w, body)
	}
}
