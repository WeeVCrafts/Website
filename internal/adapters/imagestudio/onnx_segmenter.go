package imagestudio

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/image/draw"
)

type onnxModelProfile struct {
	engineName string
	inputSize  int
	mean       [3]float32
	std        [3]float32
}

func resolveModelProfile(modelPath string) onnxModelProfile {
	base := strings.ToLower(filepath.Base(modelPath))
	if strings.Contains(base, "isnet") {
		return onnxModelProfile{
			engineName: "onnx-isnet",
			inputSize:  1024,
			mean:       [3]float32{0.5, 0.5, 0.5},
			std:        [3]float32{1.0, 1.0, 1.0},
		}
	}
	return onnxModelProfile{
		engineName: "onnx-u2netp",
		inputSize:  320,
		mean:       [3]float32{0.485, 0.456, 0.406},
		std:        [3]float32{0.229, 0.224, 0.225},
	}
}

type ONNXSegmenter struct {
	modelPath   string
	libraryPath string
	threads     int
	profile     onnxModelProfile

	mu          sync.Mutex
	initialized bool
	initErr     error
	ort         *ortSession
}

func NewONNXSegmenter(modelPath, libraryPath string, threads int) *ONNXSegmenter {
	if threads <= 0 || threads > 4 {
		threads = 2
	}
	return &ONNXSegmenter{
		modelPath:   modelPath,
		libraryPath: resolveLibraryPath(libraryPath),
		threads:     threads,
		profile:     resolveModelProfile(modelPath),
	}
}

func resolveLibraryPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	name := "libonnxruntime.so"
	if runtime.GOOS == "windows" {
		name = "onnxruntime.dll"
	} else if runtime.GOOS == "darwin" {
		name = "libonnxruntime.dylib"
	}
	for _, candidate := range []string{
		filepath.Join("models", name),
		filepath.Join("bin", name),
		name,
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return name
}

func (s *ONNXSegmenter) Available() bool {
	if s.modelPath == "" {
		return false
	}
	if _, err := os.Stat(s.modelPath); err != nil {
		return false
	}
	return s.ensureLoaded() == nil
}

func (s *ONNXSegmenter) ensureLoaded() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialized {
		return s.initErr
	}
	s.initialized = true

	session, err := openORTSession(s.modelPath, s.libraryPath, s.threads)
	if err != nil {
		s.initErr = err
		return err
	}
	s.ort = session
	slog.Info("local ONNX segmentation model loaded", "model", s.modelPath, "engine", s.profile.engineName, "input_size", s.profile.inputSize, "threads", s.threads)
	return nil
}

func (s *ONNXSegmenter) Segment(ctx context.Context, src image.Image) (*image.Alpha, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if err := s.ensureLoaded(); err != nil {
		return nil, "", err
	}

	size := s.profile.inputSize
	resized := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.BiLinear.Scale(resized, resized.Bounds(), src, src.Bounds(), draw.Src, nil)

	plane := size * size
	inputData := make([]float32, 3*plane)
	mean := s.profile.mean
	std := s.profile.std
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := resized.NRGBAAt(x, y)
			idx := y*size + x
			inputData[idx] = (float32(c.R)/255.0 - mean[0]) / std[0]
			inputData[plane+idx] = (float32(c.G)/255.0 - mean[1]) / std[1]
			inputData[2*plane+idx] = (float32(c.B)/255.0 - mean[2]) / std[2]
		}
	}

	s.mu.Lock()
	rawMask, err := s.ort.runU2Net(ctx, inputData, size)
	s.mu.Unlock()
	if err != nil {
		return nil, "", err
	}
	if len(rawMask) < plane {
		return nil, "", fmt.Errorf("unexpected ONNX mask length %d", len(rawMask))
	}

	minV, maxV := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, v := range rawMask[:plane] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}

	// Preserve calibrated sigmoid probabilities directly when already in [0, 1];
	// only min-max normalize when outputs are outside [0, 1].
	useDirectProb := minV >= -0.05 && maxV <= 1.05 && maxV > 0.25
	span := maxV - minV
	if span < 1e-5 {
		span = 1
	}

	smallAlpha := image.NewAlpha(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			val := rawMask[y*size+x]
			var norm float32
			if useDirectProb {
				norm = val
			} else {
				norm = (val - minV) / span
			}
			if norm <= 0.04 {
				norm = 0
			} else if norm >= 0.98 {
				norm = 1
			}
			smallAlpha.SetAlpha(x, y, color.Alpha{A: uint8(norm * 255)})
		}
	}

	b := src.Bounds()
	fullAlpha := image.NewAlpha(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.CatmullRom.Scale(fullAlpha, fullAlpha.Bounds(), smallAlpha, smallAlpha.Bounds(), draw.Src, nil)
	return fullAlpha, s.profile.engineName, nil
}

func (s *ONNXSegmenter) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ort != nil {
		s.ort.Close()
		s.ort = nil
	}
}
