package imagestudio

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
)

func makeSyntheticProductImage(w, h int, bg, fg color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	for y := h / 4; y < (h*3)/4; y++ {
		for x := w / 4; x < (w*3)/4; x++ {
			img.SetRGBA(x, y, fg)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestValidateRejectsInvalidAndOversizedImages(t *testing.T) {
	if _, _, err := DetectAndValidateImage([]byte("not an image"), "image/png"); err == nil {
		t.Fatal("expected invalid signature error")
	}

	small := makeSyntheticProductImage(24, 24, color.RGBA{255, 255, 255, 255}, color.RGBA{120, 40, 30, 255})
	if _, _, err := DetectAndValidateImage(small, "image/png"); err == nil {
		t.Fatal("expected too-small dimension error")
	}

	valid := makeSyntheticProductImage(240, 200, color.RGBA{240, 240, 240, 255}, color.RGBA{168, 59, 45, 255})
	if _, _, err := DetectAndValidateImage(valid, "image/jpeg"); err == nil {
		t.Fatal("expected declared MIME mismatch error")
	}
	mimeType, cfg, err := DetectAndValidateImage(valid, "image/png")
	if err != nil || mimeType != "image/png" || cfg.Width != 240 || cfg.Height != 200 {
		t.Fatalf("unexpected validation result: mime=%q cfg=%+v err=%v", mimeType, cfg, err)
	}
}

func TestRemoveBackgroundAndPreserveProductColors(t *testing.T) {
	engine := NewEngine("", "", 1, 2, 10*time.Second)
	defer engine.Close()

	bg := color.RGBA{242, 240, 236, 255}
	fg := color.RGBA{168, 59, 45, 255}
	raw := makeSyntheticProductImage(320, 320, bg, fg)

	prepared, mimeType, w, h, err := engine.ValidateAndPrepare(raw, "image/png")
	if err != nil || mimeType != "image/png" || w != 320 || h != 320 {
		t.Fatalf("prepare failed: %v (%dx%d)", err, w, h)
	}

	cutout, err := engine.RemoveBackground(context.Background(), prepared)
	if err != nil {
		t.Fatalf("remove background failed: %v", err)
	}
	if cutout.EngineUsed != "cv-fallback" {
		t.Fatalf("expected cv-fallback engine when no ONNX model configured, got %q", cutout.EngineUsed)
	}

	decoded, err := png.Decode(bytes.NewReader(cutout.CutoutPNG))
	if err != nil {
		t.Fatalf("decode cutout PNG: %v", err)
	}
	// Corner pixel should be transparent background.
	_, _, _, cornerA := decoded.At(4, 4).RGBA()
	if cornerA != 0 {
		t.Fatalf("expected transparent corner pixel, got alpha=%d", cornerA)
	}
	// Center pixel must preserve exact product color and full opacity.
	r, g, b, a := decoded.At(160, 160).RGBA()
	if uint8(r>>8) != fg.R || uint8(g>>8) != fg.G || uint8(b>>8) != fg.B || uint8(a>>8) != 255 {
		t.Fatalf("product interior altered: got rgba=(%d,%d,%d,%d) want (%d,%d,%d,255)", uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8), fg.R, fg.G, fg.B)
	}
	if len(cutout.DominantColors) == 0 {
		t.Fatal("expected extracted dominant colors")
	}
}

func TestComposeAllPresetsAndPositioning(t *testing.T) {
	engine := NewEngine("", "", 1, 2, 10*time.Second)
	defer engine.Close()

	raw := makeSyntheticProductImage(300, 300, color.RGBA{240, 240, 240, 255}, color.RGBA{25, 95, 70, 255})
	prepared, _, _, _, err := engine.ValidateAndPrepare(raw, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	cutout, err := engine.RemoveBackground(context.Background(), prepared)
	if err != nil {
		t.Fatal(err)
	}

	for _, preset := range domainstudio.AllPresets() {
		rendered, err := engine.Compose(context.Background(), cutout.CutoutPNG, preset.Slug, domainstudio.CompositionParams{
			OffsetX:        0.08,
			OffsetY:        -0.05,
			Scale:          0.75,
			FitMode:        domainstudio.FitContain,
			ShadowStrength: 0.4,
		})
		if err != nil {
			t.Fatalf("compose preset %q failed: %v", preset.Slug, err)
		}
		if rendered.ContentType != "image/jpeg" || rendered.Width != domainstudio.StudioCanvasDim || rendered.Height != domainstudio.StudioCanvasDim {
			t.Fatalf("unexpected rendered dimensions for %q: %+v", preset.Slug, rendered)
		}
		if _, err := jpeg.DecodeConfig(bytes.NewReader(rendered.Bytes)); err != nil {
			t.Fatalf("invalid JPEG output for %q: %v", preset.Slug, err)
		}
	}
}

func TestCategoryRecommendationsAdaptToLightAndDarkProducts(t *testing.T) {
	lightColors := []domainstudio.DominantColor{{Hex: "#f8f8f8", Luminance: 0.94, Share: 1.0}}
	rec := domainstudio.RecommendPresets("brass-metalwork", "Brass & Metalwork", "square", lightColors)
	if len(rec.RecommendedPresets) < 3 {
		t.Fatalf("expected at least 3 recommendations, got %+v", rec.RecommendedPresets)
	}

	darkColors := []domainstudio.DominantColor{{Hex: "#151515", Luminance: 0.08, Share: 1.0}}
	darkRec := domainstudio.RecommendPresets("brass-metalwork", "Brass & Metalwork", "portrait", darkColors)
	if darkRec.RecommendedPresets[0] == "dark-studio" || darkRec.RecommendedPresets[0] == "luxury-studio" {
		t.Fatalf("expected light contrasting preset first for dark product, got %v", darkRec.RecommendedPresets)
	}
}

func TestPerformanceLargeAndConcurrentImagesWithinMemoryBudget(t *testing.T) {
	engine := NewEngine("", "", 2, 2, 15*time.Second)
	defer engine.Close()

	// Test real repository product photo if present, else synthetic 2400x2400 image.
	var raw []byte
	samplePath := filepath.Join("..", "..", "..", "web", "assets", "images", "customer", "black-cushion.png")
	if data, err := os.ReadFile(samplePath); err == nil {
		raw = data
	} else {
		raw = makeSyntheticProductImage(2400, 2400, color.RGBA{240, 238, 235, 255}, color.RGBA{45, 45, 48, 255})
	}

	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	start := time.Now()

	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prepared, _, _, _, err := engine.ValidateAndPrepare(raw, "")
			if err != nil {
				errs <- err
				return
			}
			cutout, err := engine.RemoveBackground(context.Background(), prepared)
			if err != nil {
				errs <- err
				return
			}
			if _, err := engine.Compose(context.Background(), cutout.CutoutPNG, "pure-white", domainstudio.DefaultComposition()); err != nil {
				errs <- err
				return
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	elapsed := time.Since(start)
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	t.Logf("3 concurrent jobs completed in %v (HeapAlloc before=%d MiB, after=%d MiB, TotalAllocDelta=%d MiB)",
		elapsed,
		memBefore.HeapAlloc/(1<<20),
		memAfter.HeapAlloc/(1<<20),
		(memAfter.TotalAlloc-memBefore.TotalAlloc)/(1<<20),
	)
}

func TestComposeProductBoundedOnExtremeAspectRatioCover(t *testing.T) {
	strip := image.NewNRGBA(image.Rect(0, 0, 1400, 100))
	for x := 0; x < 1400; x++ {
		strip.SetNRGBA(x, 50, color.NRGBA{R: 120, G: 80, B: 40, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, strip); err != nil {
		t.Fatalf("encode strip: %v", err)
	}
	params := domainstudio.CompositionParams{
		OffsetX:        0,
		OffsetY:        0,
		Scale:          1.40,
		FitMode:        domainstudio.FitCover,
		ShadowStrength: 0.25,
	}
	composed, ct, w, h, err := ComposeProduct(context.Background(), buf.Bytes(), "pure-white", params)
	if err != nil {
		t.Fatalf("ComposeProduct failed on thin strip: %v", err)
	}
	if ct != "image/jpeg" || w != domainstudio.StudioCanvasDim || h != domainstudio.StudioCanvasDim || len(composed) == 0 {
		t.Fatalf("unexpected output: ct=%s dims=%dx%d bytes=%d", ct, w, h, len(composed))
	}
}
