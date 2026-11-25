//go:build windows

package imagestudio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows CGO-free ONNX Runtime C-API bridge (API version 23) using
// LoadLibrary/SyscallN so builds succeed with CGO_ENABLED=0 and no gcc.
type ortAPIBase struct {
	GetAPI           uintptr
	GetVersionString uintptr
}

// Field indices match ONNX Runtime C API v23 (onnxruntime_c_api.h).
type ortWinAPI struct {
	CreateStatus                   uintptr // 0
	GetErrorCode                   uintptr // 1
	GetErrorMessage                uintptr // 2
	CreateEnv                      uintptr // 3
	_                              [4]uintptr
	CreateSessionFromArray         uintptr // 8
	Run                            uintptr // 9
	CreateSessionOptions           uintptr // 10
	_                              [13]uintptr
	SetIntraOpNumThreads           uintptr // 24
	_                              [5]uintptr
	SessionGetInputCount           uintptr // 30
	SessionGetOutputCount          uintptr // 31
	_                              [4]uintptr
	SessionGetInputName            uintptr // 36
	SessionGetOutputName           uintptr // 37
	_                              [11]uintptr
	CreateTensorWithDataAsOrtValue uintptr // 49
	_                              [1]uintptr
	GetTensorMutableData           uintptr // 51
	_                              [8]uintptr
	GetTensorElementType           uintptr // 60
	GetDimensionsCount             uintptr // 61
	GetDimensions                  uintptr // 62
	_                              [1]uintptr
	GetTensorShapeElementCount     uintptr // 64
	GetTensorTypeAndShape          uintptr // 65
	_                              [3]uintptr
	CreateCpuMemoryInfo            uintptr // 69
	_                              [6]uintptr
	AllocatorFree                  uintptr // 76
	_                              [1]uintptr
	GetAllocatorWithDefaultOptions uintptr // 78
	_                              [13]uintptr
	ReleaseEnv                     uintptr // 92
	ReleaseStatus                  uintptr // 93
	ReleaseMemoryInfo              uintptr // 94
	ReleaseSession                 uintptr // 95
	ReleaseValue                   uintptr // 96
	_                              [2]uintptr
	ReleaseTensorTypeAndShapeInfo  uintptr // 99
	ReleaseSessionOptions          uintptr // 100
}

type ortSession struct {
	dll        *windows.LazyDLL
	api        *ortWinAPI
	env        uintptr
	session    uintptr
	alloc      uintptr
	memInfo    uintptr
	inputName  []byte
	outputName []byte
}

func openORTSession(modelPath, libraryPath string, threads int) (*ortSession, error) {
	modelBytes, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, fmt.Errorf("ONNX model file unavailable: %w", err)
	}
	if len(modelBytes) == 0 {
		return nil, errors.New("ONNX model file is empty")
	}

	dll := windows.NewLazyDLL(libraryPath)
	if err := dll.Load(); err != nil {
		return nil, fmt.Errorf("ONNX Runtime DLL unavailable (%s): %w", libraryPath, err)
	}
	proc := dll.NewProc("OrtGetApiBase")
	if err := proc.Find(); err != nil {
		return nil, fmt.Errorf("OrtGetApiBase not found: %w", err)
	}

	basePtr, _, _ := proc.Call()
	if basePtr == 0 {
		return nil, errors.New("OrtGetApiBase returned null")
	}
	base := (*ortAPIBase)(unsafe.Pointer(basePtr))
	apiPtr, _, _ := syscall.SyscallN(base.GetAPI, 23)
	if apiPtr == 0 {
		return nil, errors.New("ONNX Runtime API v23 not supported by DLL")
	}

	api := (*ortWinAPI)(unsafe.Pointer(apiPtr))
	s := &ortSession{dll: dll, api: api}

	logID := append([]byte("weevcrafts-image-studio"), 0)
	if err := s.check(syscall.SyscallN(api.CreateEnv, 2 /* WARNING */, uintptr(unsafe.Pointer(&logID[0])), uintptr(unsafe.Pointer(&s.env)))); err != nil {
		return nil, err
	}
	if err := s.check(syscall.SyscallN(api.GetAllocatorWithDefaultOptions, uintptr(unsafe.Pointer(&s.alloc)))); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.check(syscall.SyscallN(api.CreateCpuMemoryInfo, 1 /* OrtArenaAllocator */, 0 /* OrtMemTypeDefault */, uintptr(unsafe.Pointer(&s.memInfo)))); err != nil {
		s.Close()
		return nil, err
	}

	var opts uintptr
	if err := s.check(syscall.SyscallN(api.CreateSessionOptions, uintptr(unsafe.Pointer(&opts)))); err != nil {
		s.Close()
		return nil, err
	}
	defer syscall.SyscallN(api.ReleaseSessionOptions, opts)

	if err := s.check(syscall.SyscallN(api.SetIntraOpNumThreads, opts, uintptr(int32(threads)))); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.check(syscall.SyscallN(api.CreateSessionFromArray, s.env, uintptr(unsafe.Pointer(&modelBytes[0])), uintptr(len(modelBytes)), opts, uintptr(unsafe.Pointer(&s.session)))); err != nil {
		s.Close()
		return nil, err
	}
	runtime.KeepAlive(modelBytes)

	var inCount, outCount uintptr
	if err := s.check(syscall.SyscallN(api.SessionGetInputCount, s.session, uintptr(unsafe.Pointer(&inCount)))); err != nil || inCount == 0 {
		s.Close()
		return nil, errors.New("ONNX model has no inputs")
	}
	if err := s.check(syscall.SyscallN(api.SessionGetOutputCount, s.session, uintptr(unsafe.Pointer(&outCount)))); err != nil || outCount == 0 {
		s.Close()
		return nil, errors.New("ONNX model has no outputs")
	}

	var inPtr, outPtr *byte
	if err := s.check(syscall.SyscallN(api.SessionGetInputName, s.session, 0, s.alloc, uintptr(unsafe.Pointer(&inPtr)))); err != nil {
		s.Close()
		return nil, err
	}
	s.inputName = append([]byte(cStr(inPtr)), 0)
	_, _, _ = syscall.SyscallN(api.AllocatorFree, s.alloc, uintptr(unsafe.Pointer(inPtr)))

	if err := s.check(syscall.SyscallN(api.SessionGetOutputName, s.session, 0, s.alloc, uintptr(unsafe.Pointer(&outPtr)))); err != nil {
		s.Close()
		return nil, err
	}
	s.outputName = append([]byte(cStr(outPtr)), 0)
	_, _, _ = syscall.SyscallN(api.AllocatorFree, s.alloc, uintptr(unsafe.Pointer(outPtr)))

	return s, nil
}

func (o *ortSession) runU2Net(ctx context.Context, inputData []float32, inputSize int) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if o == nil || o.session == 0 {
		return nil, errors.New("ONNX session closed")
	}
	shape := [4]int64{1, 3, int64(inputSize), int64(inputSize)}
	var inVal uintptr
	if err := o.check(syscall.SyscallN(
		o.api.CreateTensorWithDataAsOrtValue,
		o.memInfo,
		uintptr(unsafe.Pointer(&inputData[0])),
		uintptr(len(inputData)*4),
		uintptr(unsafe.Pointer(&shape[0])),
		4,
		1, // ONNX_TENSOR_ELEMENT_DATA_TYPE_FLOAT
		uintptr(unsafe.Pointer(&inVal)),
	)); err != nil {
		return nil, err
	}
	defer syscall.SyscallN(o.api.ReleaseValue, inVal)

	inNames := [1]*byte{&o.inputName[0]}
	outNames := [1]*byte{&o.outputName[0]}
	inVals := [1]uintptr{inVal}
	var outVal uintptr

	if err := o.check(syscall.SyscallN(
		o.api.Run,
		o.session,
		0,
		uintptr(unsafe.Pointer(&inNames[0])),
		uintptr(unsafe.Pointer(&inVals[0])),
		1,
		uintptr(unsafe.Pointer(&outNames[0])),
		1,
		uintptr(unsafe.Pointer(&outVal)),
	)); err != nil {
		return nil, err
	}
	defer syscall.SyscallN(o.api.ReleaseValue, outVal)
	runtime.KeepAlive(inputData)
	runtime.KeepAlive(shape)

	var shapeInfo uintptr
	if err := o.check(syscall.SyscallN(o.api.GetTensorTypeAndShape, outVal, uintptr(unsafe.Pointer(&shapeInfo)))); err != nil {
		return nil, err
	}
	defer syscall.SyscallN(o.api.ReleaseTensorTypeAndShapeInfo, shapeInfo)

	var elemType uint32
	if err := o.check(syscall.SyscallN(o.api.GetTensorElementType, shapeInfo, uintptr(unsafe.Pointer(&elemType)))); err != nil {
		return nil, err
	}
	if elemType != 1 {
		return nil, fmt.Errorf("unexpected ONNX output element type %d", elemType)
	}
	var count uintptr
	if err := o.check(syscall.SyscallN(o.api.GetTensorShapeElementCount, shapeInfo, uintptr(unsafe.Pointer(&count)))); err != nil {
		return nil, err
	}
	var dataPtr unsafe.Pointer
	if err := o.check(syscall.SyscallN(o.api.GetTensorMutableData, outVal, uintptr(unsafe.Pointer(&dataPtr)))); err != nil {
		return nil, err
	}

	slice := unsafe.Slice((*float32)(dataPtr), int(count))
	copied := make([]float32, len(slice))
	copy(copied, slice)
	return copied, nil
}

func (o *ortSession) check(status uintptr, _ uintptr, _ syscall.Errno) error {
	if status == 0 {
		return nil
	}
	defer syscall.SyscallN(o.api.ReleaseStatus, status)
	msgPtr, _, _ := syscall.SyscallN(o.api.GetErrorMessage, status)
	return errors.New(cStr((*byte)(unsafe.Pointer(msgPtr))))
}

func (o *ortSession) Close() {
	if o == nil || o.api == nil {
		return
	}
	if o.session != 0 {
		_, _, _ = syscall.SyscallN(o.api.ReleaseSession, o.session)
		o.session = 0
	}
	if o.memInfo != 0 {
		_, _, _ = syscall.SyscallN(o.api.ReleaseMemoryInfo, o.memInfo)
		o.memInfo = 0
	}
	if o.env != 0 {
		_, _, _ = syscall.SyscallN(o.api.ReleaseEnv, o.env)
		o.env = 0
	}
}

func cStr(p *byte) string {
	if p == nil {
		return ""
	}
	var n int
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}
