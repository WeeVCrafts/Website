package web

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/wecratfs/commerce/internal/ports"
)

type seekMedia struct {
	*bytes.Reader
	read int
}

func (s *seekMedia) Close() error               { return nil }
func (s *seekMedia) Read(p []byte) (int, error) { n, e := s.Reader.Read(p); s.read += n; return n, e }

func TestVideoRangeAndHeadNeverBufferWholeObject(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		body := &seekMedia{Reader: bytes.NewReader(bytes.Repeat([]byte{1}, 1<<20))}
		request := httptest.NewRequest(method, "/media/products/77", nil)
		request.Header.Set("Range", "bytes=0-1")
		response := httptest.NewRecorder()
		(&Handler{}).serveMedia(response, request, 77, body, ports.ObjectInfo{ContentType: "video/mp4", ByteSize: 1 << 20}, true)
		if response.Code != 206 || response.Header().Get("Content-Range") != "bytes 0-1/1048576" {
			t.Fatalf("range response: %d %v", response.Code, response.Header())
		}
		if body.read > 2 {
			t.Fatalf("%s read %d bytes for a two-byte range", method, body.read)
		}
		if method == "HEAD" && response.Body.Len() != 0 {
			t.Fatal("HEAD body")
		}
	}
}
func TestProductPhotographVariantsAreSmallerAndPreserveOriginal(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 1600, 1000))
	for y := 0; y < 1000; y++ {
		for x := 0; x < 1600; x++ {
			source.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var original bytes.Buffer
	if err := jpeg.Encode(&original, source, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		query string
		width int
	}{{"?w=320", 320}, {"?w=640", 640}, {"?w=960", 960}, {"", 1600}} {
		response := httptest.NewRecorder()
		data := original.Bytes()
		(&Handler{}).serveMedia(response, httptest.NewRequest("GET", "/media/products/881"+test.query, nil), 881, io.NopCloser(bytes.NewReader(data)), ports.ObjectInfo{ContentType: "image/jpeg", ByteSize: int64(len(data))}, true)
		config, _, err := image.DecodeConfig(bytes.NewReader(response.Body.Bytes()))
		if err != nil || config.Width != test.width || response.Code != 200 {
			t.Fatalf("%s width=%d status=%d error=%v", test.query, config.Width, response.Code, err)
		}
		if test.query != "" && response.Body.Len() >= len(data) {
			t.Fatal("variant did not reduce transfer")
		}
		if test.query == "" && !bytes.Equal(response.Body.Bytes(), data) {
			t.Fatal("original changed")
		}
	}
	response := httptest.NewRecorder()
	(&Handler{}).serveMedia(response, httptest.NewRequest("GET", "/media/products/881?w=320", nil), 881, io.NopCloser(bytes.NewReader([]byte("broken"))), ports.ObjectInfo{ContentType: "image/jpeg", ByteSize: 100}, false)
	if response.Code != 503 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private or failed media used public derivative/cache")
	}
}
func TestPhonePhotographOrientationIsRetained(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 3))
	source.Set(0, 0, color.RGBA{255, 0, 0, 255})
	for orientation := 2; orientation <= 8; orientation++ {
		rotated := orientPhotograph(source, orientation)
		if orientation >= 5 && (rotated.Bounds().Dx() != 3 || rotated.Bounds().Dy() != 2) {
			t.Fatalf("orientation %d has wrong dimensions", orientation)
		}
	}
	for _, data := range [][]byte{nil, []byte("bad"), {0xff, 0xd8, 0xff, 0xe1, 0xff, 0xff}} {
		if jpegOrientation(data) != 1 {
			t.Fatal("malformed EXIF accepted")
		}
	}
}
