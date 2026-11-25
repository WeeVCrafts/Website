//go:build !windows && !onnx

package imagestudio

import (
	"context"
	"errors"
)

type ortSession struct{}

func openORTSession(_, _ string, _ int) (*ortSession, error) {
	return nil, errors.New("ONNX runtime support disabled in static build; using pure-Go CV segmenter")
}

func (o *ortSession) runU2Net(_ context.Context, _ []float32, _ int) ([]float32, error) {
	return nil, errors.New("ONNX session unavailable")
}

func (o *ortSession) Close() {}
