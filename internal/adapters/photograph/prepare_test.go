package photograph

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"testing"

	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type storedPhoto struct {
	bytes []byte
	info  ports.ObjectInfo
}
type photographFixture struct {
	objects     map[string]storedPhoto
	derivatives map[int]ports.MediaDerivative
	original    domain.MediaRecord
	opens       int
}

func (f *photographFixture) OpenObject(_ context.Context, key string) (io.ReadCloser, ports.ObjectInfo, error) {
	photo, ok := f.objects[key]
	if !ok {
		return nil, ports.ObjectInfo{}, ports.ErrNotFound
	}
	f.opens++
	return io.NopCloser(bytes.NewReader(photo.bytes)), photo.info, nil
}
func (f *photographFixture) PutObject(_ context.Context, key, kind string, size int64, body io.Reader) error {
	data, err := io.ReadAll(body)
	f.objects[key] = storedPhoto{data, ports.ObjectInfo{ContentType: kind, ByteSize: size}}
	return err
}
func (f *photographFixture) PublicDerivative(_ context.Context, _ domain.MediaRecord, width int) (ports.MediaDerivative, error) {
	d, ok := f.derivatives[width]
	if !ok {
		return d, ports.ErrNotFound
	}
	return d, nil
}
func (f *photographFixture) SaveDerivative(_ context.Context, d ports.MediaDerivative) error {
	f.derivatives[d.Width] = d
	return nil
}
func (f *photographFixture) DerivativeCandidates(context.Context, int) ([]domain.MediaRecord, error) {
	if len(f.derivatives) == 3 {
		return nil, nil
	}
	return []domain.MediaRecord{f.original}, nil
}

func TestPrepareStoresResponsiveImagesAndReplaysWithoutSourceReads(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1400, 1000))
	for y := 0; y < 1000; y++ {
		for x := 0; x < 1400; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), uint8(x * y), 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	info := ports.ObjectInfo{ContentType: "image/jpeg", ByteSize: int64(len(data))}
	fixture := &photographFixture{
		original:    domain.MediaRecord{ID: 9, ObjectKey: "products/fixture/original", ContentType: info.ContentType, ByteSize: info.ByteSize},
		objects:     map[string]storedPhoto{"products/fixture/original": {data, info}},
		derivatives: make(map[int]ports.MediaDerivative),
	}
	if err := Prepare(t.Context(), fixture, fixture); err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{320, 640, 960} {
		d, ok := fixture.derivatives[width]
		if !ok {
			t.Fatalf("missing %dpx derivative", width)
		}
		photo := fixture.objects[d.ObjectKey]
		config, _, err := image.DecodeConfig(bytes.NewReader(photo.bytes))
		if err != nil || config.Width != width || len(photo.bytes) >= len(data) || d.SourceKey != fixture.original.ObjectKey {
			t.Fatalf("invalid derivative width=%d info=%+v err=%v", width, config, err)
		}
	}
	if !bytes.Equal(fixture.objects[fixture.original.ObjectKey].bytes, data) {
		t.Fatal("original changed")
	}
	if err := Prepare(t.Context(), fixture, fixture); err != nil || fixture.opens != 1 {
		t.Fatalf("replay reread original: %d %v", fixture.opens, err)
	}
}
