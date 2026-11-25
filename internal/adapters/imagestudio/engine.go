package imagestudio

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"log/slog"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
)

type Engine struct {
	onnx      *ONNXSegmenter
	cv        *CVSegmenter
	semaphore chan struct{}
	timeout   time.Duration
}

func NewEngine(modelPath, libraryPath string, maxConcurrent, onnxThreads int, timeout time.Duration) *Engine {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	var onnx *ONNXSegmenter
	if modelPath != "" {
		onnx = NewONNXSegmenter(modelPath, libraryPath, onnxThreads)
	}
	return &Engine{
		onnx:      onnx,
		cv:        NewCVSegmenter(),
		semaphore: make(chan struct{}, maxConcurrent),
		timeout:   timeout,
	}
}

func (e *Engine) Close() {
	if e.onnx != nil {
		e.onnx.Close()
	}
}

func (e *Engine) ValidateAndPrepare(data []byte, declaredType string) (image.Image, string, int, int, error) {
	return DecodeAndPrepare(data, declaredType)
}

func (e *Engine) RemoveBackground(ctx context.Context, src image.Image) (ports.ProcessedCutout, error) {
	if src == nil {
		return ports.ProcessedCutout{}, ErrInvalidImage
	}
	waitCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	select {
	case e.semaphore <- struct{}{}:
		defer func() { <-e.semaphore }()
	case <-waitCtx.Done():
		return ports.ProcessedCutout{}, ErrEngineBusy
	}

	var mask *image.Alpha
	var engineUsed string
	var err error

	if e.onnx != nil {
		mask, engineUsed, err = e.onnx.Segment(waitCtx, src)
		if err != nil {
			slog.Debug("ONNX segmentation unavailable, using pure-Go CV segmenter", "reason", err)
			mask = nil
		}
	}
	if mask == nil {
		mask, engineUsed, err = e.cv.Segment(waitCtx, src)
		if err != nil {
			return ports.ProcessedCutout{}, err
		}
	}

	cutout, cutoutPNG, err := RefineAndApplyMask(src, mask)
	if err != nil {
		return ports.ProcessedCutout{}, err
	}
	b := cutout.Bounds()
	fgRatio, warning := AnalyzeCutoutQuality(cutout)
	return ports.ProcessedCutout{
		CutoutPNG:       cutoutPNG,
		ContentType:     "image/png",
		Width:           b.Dx(),
		Height:          b.Dy(),
		AspectRatio:     ClassifyAspectRatio(b.Dx(), b.Dy()),
		DominantColors:  ExtractDominantColors(cutout, 4),
		EngineUsed:      engineUsed,
		ForegroundRatio: fgRatio,
		QualityWarning:  warning,
	}, nil
}

func (e *Engine) Compose(ctx context.Context, cutoutPNG []byte, presetSlug string, params domainstudio.CompositionParams) (ports.RenderedComposition, error) {
	waitCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	select {
	case e.semaphore <- struct{}{}:
		defer func() { <-e.semaphore }()
	case <-waitCtx.Done():
		return ports.RenderedComposition{}, ErrEngineBusy
	}

	data, contentType, w, h, err := ComposeProduct(waitCtx, cutoutPNG, presetSlug, params)
	if err != nil {
		return ports.RenderedComposition{}, err
	}
	return ports.RenderedComposition{
		Bytes:       data,
		ContentType: contentType,
		Width:       w,
		Height:      h,
	}, nil
}

// SanitizeCutout validates a seller-edited transparent PNG cutout, strips unsafe metadata,
// and recomputes foreground coverage and dominant color insights.
func (e *Engine) SanitizeCutout(data []byte) (ports.ProcessedCutout, error) {
	if len(data) == 0 || len(data) > domainstudio.MaxUploadBytes {
		return ports.ProcessedCutout{}, ErrInvalidCutout
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < domainstudio.MinDimensionPx || cfg.Height < domainstudio.MinDimensionPx ||
		cfg.Width > domainstudio.MaxDimensionPx || cfg.Height > domainstudio.MaxDimensionPx ||
		int64(cfg.Width)*int64(cfg.Height) > domainstudio.MaxTotalPixels {
		return ports.ProcessedCutout{}, ErrInvalidCutout
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return ports.ProcessedCutout{}, ErrInvalidCutout
	}
	nrgba := toNRGBA(decoded)
	var clean bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&clean, nrgba); err != nil {
		return ports.ProcessedCutout{}, err
	}
	b := nrgba.Bounds()
	fgRatio, warning := AnalyzeCutoutQuality(nrgba)
	return ports.ProcessedCutout{
		CutoutPNG:       clean.Bytes(),
		ContentType:     "image/png",
		Width:           b.Dx(),
		Height:          b.Dy(),
		AspectRatio:     ClassifyAspectRatio(b.Dx(), b.Dy()),
		DominantColors:  ExtractDominantColors(nrgba, 4),
		EngineUsed:      "manual-edit",
		ForegroundRatio: fgRatio,
		QualityWarning:  warning,
	}, nil
}
