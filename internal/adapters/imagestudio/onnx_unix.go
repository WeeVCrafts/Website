//go:build !windows && onnx

package imagestudio

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/shota3506/onnxruntime-purego/onnxruntime"
)

type ortSession struct {
	ort        *onnxruntime.Runtime
	env        *onnxruntime.Env
	session    *onnxruntime.Session
	inputName  string
	outputName string
}

func openORTSession(modelPath, libraryPath string, threads int) (*ortSession, error) {
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("ONNX model file unavailable: %w", err)
	}
	rt, err := onnxruntime.NewRuntime(libraryPath, 23)
	if err != nil {
		return nil, fmt.Errorf("ONNX Runtime library unavailable: %w", err)
	}
	env, err := rt.NewEnv("weevcrafts-image-studio", onnxruntime.LoggingLevelWarning)
	if err != nil {
		_ = rt.Close()
		return nil, err
	}
	session, err := rt.NewSession(env, modelPath, &onnxruntime.SessionOptions{
		IntraOpNumThreads: threads,
	})
	if err != nil {
		env.Close()
		_ = rt.Close()
		return nil, err
	}
	inputs := session.InputNames()
	outputs := session.OutputNames()
	if len(inputs) == 0 || len(outputs) == 0 {
		session.Close()
		env.Close()
		_ = rt.Close()
		return nil, errors.New("ONNX model has no input or output tensors")
	}
	return &ortSession{
		ort:        rt,
		env:        env,
		session:    session,
		inputName:  inputs[0],
		outputName: outputs[0],
	}, nil
}

func (o *ortSession) runU2Net(ctx context.Context, inputData []float32, inputSize int) ([]float32, error) {
	if o == nil || o.session == nil || o.ort == nil {
		return nil, errors.New("ONNX session closed")
	}
	inputTensor, err := onnxruntime.NewTensorValue(o.ort, inputData, []int64{1, 3, int64(inputSize), int64(inputSize)})
	if err != nil {
		return nil, err
	}
	defer inputTensor.Close()

	outputs, err := o.session.Run(ctx, map[string]*onnxruntime.Value{o.inputName: inputTensor}, onnxruntime.WithOutputNames(o.outputName))
	if err != nil {
		return nil, err
	}
	outVal := outputs[o.outputName]
	if outVal == nil {
		return nil, errors.New("missing ONNX output tensor")
	}
	defer outVal.Close()

	rawMask, _, err := onnxruntime.GetTensorData[float32](outVal)
	return rawMask, err
}

func (o *ortSession) Close() {
	if o == nil {
		return
	}
	if o.session != nil {
		o.session.Close()
		o.session = nil
	}
	if o.env != nil {
		o.env.Close()
		o.env = nil
	}
	if o.ort != nil {
		_ = o.ort.Close()
		o.ort = nil
	}
}
