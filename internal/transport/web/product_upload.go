package web

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const productUploadDirectory = "web/assets/images/uploads"
const maxProductImageBytes = 25 << 20

// productImageFromForm stores only the explicitly selected cover image. Upload
// failures must never turn into successful saves with an unrelated stock image.
func productImageFromForm(r *http.Request) (string, error) {
	imageURL := strings.TrimSpace(r.FormValue("image_url"))
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return imageURL, nil
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return "", err
	}
	file, header, err := r.FormFile("image_file")
	if errors.Is(err, http.ErrMissingFile) {
		return imageURL, nil
	}
	if err != nil {
		return "", err
	}
	defer file.Close()
	if header.Size <= 0 || header.Size > maxProductImageBytes {
		return "", errors.New("choose an image up to 25 MB")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxProductImageBytes+1))
	if err != nil || len(data) > maxProductImageBytes {
		return "", errors.New("could not read product image")
	}
	var extension string
	switch http.DetectContentType(data) {
	case "image/png":
		extension = ".png"
	case "image/jpeg":
		extension = ".jpg"
	case "image/webp":
		extension = ".webp"
	default:
		return "", errors.New("choose a PNG, JPEG or WebP image")
	}
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
