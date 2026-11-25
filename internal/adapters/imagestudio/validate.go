package imagestudio

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/wecratfs/commerce/internal/adapters/photograph"
	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
)

var (
	ErrInvalidImage   = errors.New("image file is invalid or unsupported")
	ErrImageTooLarge  = errors.New("image exceeds maximum size or pixel limits")
	ErrImageTooSmall  = errors.New("image dimensions are too small for processing")
	ErrEngineBusy     = errors.New("image processing engine is busy; please retry shortly")
	ErrInvalidCutout  = errors.New("foreground cutout could not be extracted")
)

func DetectAndValidateImage(data []byte, declaredType string) (string, image.Config, error) {
	if len(data) == 0 || len(data) > domainstudio.MaxUploadBytes {
		return "", image.Config{}, ErrImageTooLarge
	}
	contentType := detectImageMIME(data)
	if contentType == "" {
		return "", image.Config{}, ErrInvalidImage
	}
	if declaredType = normalizeMIME(declaredType); declaredType != "" && declaredType != "application/octet-stream" && declaredType != contentType {
		return "", image.Config{}, ErrInvalidImage
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", image.Config{}, ErrInvalidImage
	}
	if cfg.Width < domainstudio.MinDimensionPx || cfg.Height < domainstudio.MinDimensionPx {
		return "", image.Config{}, ErrImageTooSmall
	}
	if cfg.Width > domainstudio.MaxDimensionPx || cfg.Height > domainstudio.MaxDimensionPx || int64(cfg.Width)*int64(cfg.Height) > domainstudio.MaxTotalPixels {
		return "", image.Config{}, ErrImageTooLarge
	}
	return contentType, cfg, nil
}

func DecodeAndPrepare(data []byte, declaredType string) (image.Image, string, int, int, error) {
	contentType, _, err := DetectAndValidateImage(data, declaredType)
	if err != nil {
		return nil, "", 0, 0, err
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", 0, 0, fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}
	if contentType == "image/jpeg" {
		src = photograph.Orient(src, photograph.JPEGOrientation(data))
	}
	prepared := downscaleWorkingImage(src, domainstudio.WorkingMaxEdge)
	bounds := prepared.Bounds()
	return prepared, contentType, bounds.Dx(), bounds.Dy(), nil
}

func downscaleWorkingImage(src image.Image, maxEdge int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxEdge && h <= maxEdge {
		return toNRGBA(src)
	}
	var nw, nh int
	if w >= h {
		nw = maxEdge
		nh = max(1, h*maxEdge/w)
	} else {
		nh = maxEdge
		nw = max(1, w*maxEdge/h)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}

func toNRGBA(src image.Image) *image.NRGBA {
	if n, ok := src.(*image.NRGBA); ok && n.Bounds().Min == (image.Point{}) {
		return n
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

func normalizeMIME(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if idx := strings.Index(v, ";"); idx >= 0 {
		v = strings.TrimSpace(v[:idx])
	}
	if v == "image/jpg" {
		return "image/jpeg"
	}
	return v
}

func detectImageMIME(data []byte) string {
	if len(data) < 12 {
		return ""
	}
	if data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "image/png"
	}
	if string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return ""
}
