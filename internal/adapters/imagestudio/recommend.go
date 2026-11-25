package imagestudio

import (
	"fmt"
	"image"
	"sort"

	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
)

func ClassifyAspectRatio(w, h int) string {
	if w <= 0 || h <= 0 {
		return "square"
	}
	ratio := float64(w) / float64(h)
	switch {
	case ratio < 0.85:
		return "portrait"
	case ratio > 1.18:
		return "landscape"
	default:
		return "square"
	}
}

func ExtractDominantColors(cutout *image.NRGBA, limit int) []domainstudio.DominantColor {
	if cutout == nil || limit <= 0 {
		return nil
	}
	b := cutout.Bounds()
	w, h := b.Dx(), b.Dy()
	stepX := max(1, w/64)
	stepY := max(1, h/64)

	type bucket struct {
		rSum, gSum, bSum int64
		count            int64
	}
	buckets := make(map[uint16]*bucket, 64)
	var total int64

	for y := 0; y < h; y += stepY {
		for x := 0; x < w; x += stepX {
			c := cutout.NRGBAAt(x, y)
			if c.A < 180 {
				continue
			}
			// Quantize RGB into 4-bit per channel bins (4096 max buckets).
			key := (uint16(c.R>>4) << 8) | (uint16(c.G>>4) << 4) | uint16(c.B>>4)
			bk := buckets[key]
			if bk == nil {
				bk = &bucket{}
				buckets[key] = bk
			}
			bk.rSum += int64(c.R)
			bk.gSum += int64(c.G)
			bk.bSum += int64(c.B)
			bk.count++
			total++
		}
	}
	if total == 0 {
		return nil
	}

	sorted := make([]*bucket, 0, len(buckets))
	for _, bk := range buckets {
		sorted = append(sorted, bk)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	out := make([]domainstudio.DominantColor, 0, limit)
	for _, bk := range sorted {
		if len(out) >= limit {
			break
		}
		r := uint8(bk.rSum / bk.count)
		g := uint8(bk.gSum / bk.count)
		bv := uint8(bk.bSum / bk.count)
		lum := (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(bv)) / 255.0
		out = append(out, domainstudio.DominantColor{
			Hex:       fmt.Sprintf("#%02x%02x%02x", r, g, bv),
			R:         r,
			G:         g,
			B:         bv,
			Luminance: lum,
			Share:     float64(bk.count) / float64(total),
		})
	}
	return out
}
