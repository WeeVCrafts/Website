package web

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductUploadPreservesSelectedBytes(t *testing.T) {
	t.Chdir(t.TempDir())
	data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("image_url", "/old-image.png")
	other, _ := w.CreateFormFile("unrelated_file", "other.png")
	_, _ = other.Write([]byte("not the product image"))
	image, _ := w.CreateFormFile("image_file", "selected.png")
	_, _ = image.Write(data)
	_ = w.Close()
	r := httptest.NewRequest("POST", "/", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	url, err := productImageFromForm(r)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(productUploadDirectory, filepath.Base(url)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, stored) {
		t.Fatal("stored image differs from selected image")
	}
}

func TestProductUploadRejectsInvalidImageAndStorageFailure(t *testing.T) {
	for _, test := range []struct {
		name         string
		data         []byte
		blockStorage bool
	}{
		{name: "invalid image", data: []byte("not an image")},
		{name: "unavailable storage", data: append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...), blockStorage: true},
		{name: "oversized image", data: make([]byte, maxProductImageBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if test.blockStorage {
				if err := os.MkdirAll("web/assets/images", 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(productUploadDirectory, []byte("blocked"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var body bytes.Buffer
			w := multipart.NewWriter(&body)
			_ = w.WriteField("image_url", "/stock.png")
			file, _ := w.CreateFormFile("image_file", "image.png")
			_, _ = file.Write(test.data)
			_ = w.Close()
			r := httptest.NewRequest("POST", "/", &body)
			r.Header.Set("Content-Type", w.FormDataContentType())
			if url, err := productImageFromForm(r); err == nil || url != "" {
				t.Fatalf("failed upload returned %q, %v", url, err)
			}
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		})
	}
}

func TestProductWithoutUploadDoesNotInventImage(t *testing.T) {
	for _, value := range []string{"", "/existing.png"} {
		r := httptest.NewRequest("POST", "/", strings.NewReader("image_url="+value))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		url, err := productImageFromForm(r)
		if err != nil || url != value {
			t.Fatalf("got %q, %v", url, err)
		}
	}
}
