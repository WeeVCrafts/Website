package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestMerchantPhotographsStayInsideInventoryDirectory(t *testing.T) {
	root := t.TempDir()
	photos := filepath.Join(root, "photos")
	if err := os.Mkdir(photos, 0700); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(photos, "cup.png"), encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outside.png"), encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	body, kind, err := photo(photos, "cup.png")
	if err != nil || kind != "image/png" || !bytes.Equal(body, encoded.Bytes()) {
		t.Fatalf("matching photo: %s %v", kind, err)
	}
	for _, name := range []string{"../outside.png", filepath.Join(root, "outside.png"), "missing.png"} {
		if _, _, err := photo(photos, name); err == nil {
			t.Fatalf("unsafe/missing photo accepted: %q", name)
		}
	}
	if err := os.WriteFile(filepath.Join(photos, "invalid.png"), []byte("this is not a product photograph"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := photo(photos, "invalid.png"); err == nil {
		t.Fatal("invalid photograph accepted")
	}
}
