package imagestudio

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
)

func TestONNXU2NetPSegmentationOnRealProductPhotos(t *testing.T) {
	modelPath := filepath.Join("..", "..", "..", "models", "u2netp.onnx")
	libPath := filepath.Join("..", "..", "..", "models", "onnxruntime.dll")
	if runtime.GOOS != "windows" {
		libPath = filepath.Join("..", "..", "..", "models", "libonnxruntime.so")
	}
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("local ONNX model not installed (%s); run scripts/setup-image-model.ps1", modelPath)
	}
	if _, err := os.Stat(libPath); err != nil {
		t.Skipf("local ONNX Runtime library not installed (%s); run scripts/setup-image-model.ps1", libPath)
	}

	engine := NewEngine(modelPath, libPath, 1, 2, 30*time.Second)
	defer engine.Close()

	for _, name := range []string{"brass-elephant.png", "black-cushion.png", "ceramic-mugs.png"} {
		t.Run(name, func(t *testing.T) {
			photoPath := filepath.Join("..", "..", "..", "web", "assets", "images", "customer", name)
			raw, err := os.ReadFile(photoPath)
			if err != nil {
				t.Fatalf("read sample photo: %v", err)
			}

			start := time.Now()
			prepared, mimeType, w, h, err := engine.ValidateAndPrepare(raw, "image/png")
			if err != nil {
				t.Fatalf("ValidateAndPrepare(%s): %v", name, err)
			}
			cutout, err := engine.RemoveBackground(context.Background(), prepared)
			if err != nil {
				t.Fatalf("RemoveBackground(%s): %v", name, err)
			}
			segDuration := time.Since(start)

			if cutout.EngineUsed != "onnx-u2netp" {
				t.Fatalf("expected onnx-u2netp engine, got %q", cutout.EngineUsed)
			}
			decoded, err := png.Decode(bytes.NewReader(cutout.CutoutPNG))
			if err != nil {
				t.Fatalf("decode cutout PNG: %v", err)
			}
			if decoded.Bounds().Dx() != w || decoded.Bounds().Dy() != h {
				t.Fatalf("cutout dimensions mismatch: got %dx%d want %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy(), w, h)
			}

			composed, err := engine.Compose(context.Background(), cutout.CutoutPNG, "pure-white", domainstudio.DefaultComposition())
			if err != nil {
				t.Fatalf("Compose(%s): %v", name, err)
			}
			t.Logf("%s (%dx%d): engine=%s coverage=%.2f%% total_time=%v composed_bytes=%d",
				name, w, h, cutout.EngineUsed, cutout.ForegroundRatio*100, time.Since(start), len(composed.Bytes))
			_ = mimeType
			_ = segDuration
		})
	}
}

func TestONNXISNetSegmentationOnRealProductPhotos(t *testing.T) {
	modelPath := filepath.Join("..", "..", "..", "models", "isnet-general-use.onnx")
	libPath := filepath.Join("..", "..", "..", "models", "onnxruntime.dll")
	if runtime.GOOS != "windows" {
		libPath = filepath.Join("..", "..", "..", "models", "libonnxruntime.so")
	}
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("local IS-Net ONNX model not installed (%s); run scripts/setup-image-model.ps1", modelPath)
	}
	if _, err := os.Stat(libPath); err != nil {
		t.Skipf("local ONNX Runtime library not installed (%s); run scripts/setup-image-model.ps1", libPath)
	}

	engine := NewEngine(modelPath, libPath, 1, 2, 45*time.Second)
	defer engine.Close()

	for _, name := range []string{"brass-elephant.png", "black-cushion.png", "ceramic-mugs.png", "blue-tote.png", "terracotta-planter.png", "wooden-box.png"} {
		t.Run(name, func(t *testing.T) {
			photoPath := filepath.Join("..", "..", "..", "web", "assets", "images", "customer", name)
			raw, err := os.ReadFile(photoPath)
			if err != nil {
				t.Fatalf("read sample photo: %v", err)
			}

			start := time.Now()
			prepared, _, w, h, err := engine.ValidateAndPrepare(raw, "image/png")
			if err != nil {
				t.Fatalf("ValidateAndPrepare(%s): %v", name, err)
			}
			cutout, err := engine.RemoveBackground(context.Background(), prepared)
			if err != nil {
				t.Fatalf("RemoveBackground(%s): %v", name, err)
			}
			if cutout.EngineUsed != "onnx-isnet" {
				t.Fatalf("expected onnx-isnet engine, got %q", cutout.EngineUsed)
			}
			decoded, err := png.Decode(bytes.NewReader(cutout.CutoutPNG))
			if err != nil {
				t.Fatalf("decode cutout PNG: %v", err)
			}
			if decoded.Bounds().Dx() != w || decoded.Bounds().Dy() != h {
				t.Fatalf("cutout dimensions mismatch: got %dx%d want %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy(), w, h)
			}
			composed, err := engine.Compose(context.Background(), cutout.CutoutPNG, "pure-white", domainstudio.DefaultComposition())
			if err != nil {
				t.Fatalf("Compose(%s): %v", name, err)
			}
			t.Logf("%s (%dx%d): engine=%s coverage=%.2f%% total_time=%v composed_bytes=%d",
				name, w, h, cutout.EngineUsed, cutout.ForegroundRatio*100, time.Since(start), len(composed.Bytes))
		})
	}
}
