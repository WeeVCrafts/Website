package web

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const productUploadDirectory = "web/assets/images/uploads"
const maxProductImageBytes = 25 << 20

// productImageFromForm retains the legacy local upload path for maker portraits.
// Product covers must use saveProductCover and persistent object storage.
func productImageFromForm(r *http.Request) (string, error) {
	imageURL, data, contentType, err := readProductImage(r)
	if err != nil || len(data) == 0 {
		return imageURL, err
	}
	extension := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}[contentType]
	if err := os.MkdirAll(productUploadDirectory, 0755); err != nil {
		return "", err
	}
	out, err := os.CreateTemp(productUploadDirectory, "upload_*"+extension)
	if err != nil {
		return "", err
	}
	_, writeErr := out.Write(data)
	closeErr := out.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(out.Name())
		return "", err
	}
	return "/assets/images/uploads/" + filepath.Base(out.Name()), nil
}

// readProductImage validates the selected bytes without writing into an
// ephemeral application container. Product covers use the media lifecycle.
func readProductImage(r *http.Request) (string, []byte, string, error) {
	imageURL := strings.TrimSpace(r.FormValue("image_url"))
	if strings.HasPrefix(imageURL, "/media/products/") {
		imageURL = strings.TrimSuffix(imageURL, "/preview")
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return imageURL, nil, "", nil
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return "", nil, "", err
	}
	file, header, err := r.FormFile("image_file")
	if errors.Is(err, http.ErrMissingFile) {
		return imageURL, nil, "", nil
	}
	if err != nil {
		return "", nil, "", err
	}
	defer file.Close()
	if header.Size <= 0 || header.Size > maxProductImageBytes {
		return "", nil, "", errors.New("choose an image up to 25 MB")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxProductImageBytes+1))
	if err != nil || len(data) > maxProductImageBytes {
		return "", nil, "", errors.New("could not read product image")
	}
	contentType := http.DetectContentType(data)
	switch contentType {
	case "image/png", "image/jpeg", "image/webp":
	default:
		return "", nil, "", errors.New("choose a PNG, JPEG or WebP image")
	}
	return imageURL, data, contentType, nil
}

func (h *Handler) saveProductCover(r *http.Request, actorID, productID int64) error {
	_, data, contentType, err := readProductImage(r)
	if err != nil || len(data) == 0 {
		return err
	}
	if h.mediaService == nil {
		return errors.New("persistent product image storage is unavailable")
	}
	upload, err := h.mediaService.PrepareUpload(r.Context(), actorID, productID, contentType, int64(len(data)))
	if err != nil {
		return err
	}
	if err := h.mediaService.UploadObject(r.Context(), actorID, upload.ID, contentType, int64(len(data)), bytes.NewReader(data)); err != nil {
		return err
	}
	_, err = h.mediaService.FinalizeUpload(r.Context(), actorID, upload.ID)
	return err
}
