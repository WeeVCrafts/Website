package web

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"sync"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Disposable, bounded derivatives. Authorization is checked on every request
// before this cache is consulted; originals and catalogue records never change.
var productPhotographs = struct {
	sync.Mutex
	entries map[string]productPhotograph
	bytes   int
}{entries: make(map[string]productPhotograph)}

type productPhotograph struct {
	data    []byte
	expires time.Time
}

var photographWorkers = make(chan struct{}, 2)

func cachedProductPhotograph(identity string, width int) []byte {
	productPhotographs.Lock()
	defer productPhotographs.Unlock()
	key := fmt.Sprintf("%s/%d", identity, width)
	entry, ok := productPhotographs.entries[key]
	if ok && time.Now().Before(entry.expires) {
		return entry.data
	}
	if ok {
		delete(productPhotographs.entries, key)
		productPhotographs.bytes -= len(entry.data)
	}
	return nil
}
func rememberProductPhotograph(identity string, width int, data []byte) {
	if len(data) > 1<<20 {
		return
	}
	productPhotographs.Lock()
	defer productPhotographs.Unlock()
	key := fmt.Sprintf("%s/%d", identity, width)
	if _, ok := productPhotographs.entries[key]; ok {
		return
	}
	for k, v := range productPhotographs.entries {
		if time.Now().After(v.expires) || len(productPhotographs.entries) >= 128 || productPhotographs.bytes+len(data) > 32<<20 {
			delete(productPhotographs.entries, k)
			productPhotographs.bytes -= len(v.data)
		}
	}
	productPhotographs.entries[key] = productPhotograph{data: data, expires: time.Now().Add(5 * time.Minute)}
	productPhotographs.bytes += len(data)
}
func productPhotographVariant(input []byte, width int) ([]byte, error) {
	select {
	case photographWorkers <- struct{}{}:
		defer func() { <-photographWorkers }()
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("photograph workers busy")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(input))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 16000000 {
		return nil, fmt.Errorf("photograph exceeds decode limits")
	}
	source, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	source = orientPhotograph(source, jpegOrientation(input))
	w, h := source.Bounds().Dx(), source.Bounds().Dy()
	if w > width {
		h = max(1, h*width/w)
		w = width
	}
	output := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(output, output.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(output, output.Bounds(), source, source.Bounds(), draw.Over, nil)
	var b bytes.Buffer
	err = jpeg.Encode(&b, output, &jpeg.Options{Quality: 82})
	return b.Bytes(), err
}

// Respect phone-camera orientation without trusting arbitrary EXIF offsets.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for offset := 2; offset+4 <= len(data); {
		if data[offset] != 0xff {
			return 1
		}
		marker := data[offset+1]
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		size := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		if size < 2 || offset+2+size > len(data) {
			return 1
		}
		segment := data[offset+4 : offset+2+size]
		offset += size + 2
		if marker != 0xe1 || len(segment) < 14 || string(segment[:6]) != "Exif\x00\x00" {
			continue
		}
		tiff := segment[6:]
		var order binary.ByteOrder
		switch string(tiff[:2]) {
		case "II":
			order = binary.LittleEndian
		case "MM":
			order = binary.BigEndian
		default:
			return 1
		}
		if order.Uint16(tiff[2:4]) != 42 {
			return 1
		}
		start := uint64(order.Uint32(tiff[4:8]))
		if start+2 > uint64(len(tiff)) {
			return 1
		}
		count := uint64(order.Uint16(tiff[start : start+2]))
		for i := uint64(0); i < count; i++ {
			p := start + 2 + i*12
			if p+12 > uint64(len(tiff)) {
				return 1
			}
			entry := tiff[p : p+12]
			if order.Uint16(entry[:2]) == 0x112 && order.Uint16(entry[2:4]) == 3 && order.Uint32(entry[4:8]) == 1 {
				value := int(order.Uint16(entry[8:10]))
				if value >= 1 && value <= 8 {
					return value
				}
				return 1
			}
		}
	}
	return 1
}
func orientPhotograph(src image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	ow, oh := w, h
	if orientation >= 5 {
		ow, oh = h, w
	}
	output := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = w - 1 - x
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dy = h - 1 - y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			output.Set(dx, dy, src.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	return output
}
