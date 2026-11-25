package imagestudio

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// RefineAndApplyMask prunes stray disconnected background islands, runs edge-aware
// color-guided refinement on transition bands, decontaminates background color spill
// on semi-transparent border pixels, and keeps opaque product pixels bit-exact.
func RefineAndApplyMask(src image.Image, mask *image.Alpha) (*image.NRGBA, []byte, error) {
	nrgba := toNRGBA(src)
	b := nrgba.Bounds()
	w, h := b.Dx(), b.Dy()
	if mask == nil || mask.Bounds().Dx() != w || mask.Bounds().Dy() != h {
		return nil, nil, ErrInvalidCutout
	}

	pruned := pruneMaskIslands(mask, w, h)
	guided := guidedRefineMask(nrgba, pruned, w, h)
	cutout := image.NewNRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := nrgba.NRGBAAt(x, y)
			a := guided.AlphaAt(x, y).A
			if a <= 16 {
				cutout.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
				continue
			}
			if a >= 242 {
				// Preserve opaque interior foreground pixels bit-exact.
				cutout.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255})
				continue
			}
			r, g, bl := decontaminateEdgePixel(nrgba, guided, x, y, w, h, c, a)
			cutout.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: bl, A: a})
		}
	}

	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&buf, cutout); err != nil {
		return nil, nil, err
	}
	return cutout, buf.Bytes(), nil
}

// pruneMaskIslands drops stray floating artifacts and peripheral border-hugging props
// while keeping the primary product and legitimate multi-object items (e.g. mug pairs).
func pruneMaskIslands(mask *image.Alpha, w, h int) *image.Alpha {
	n := w * h
	out := image.NewAlpha(image.Rect(0, 0, w, h))
	copy(out.Pix, mask.Pix)
	if w < 16 || h < 16 {
		return out
	}

	labels := make([]int32, n)
	type compInfo struct {
		area     int
		minX     int
		maxX     int
		minY     int
		maxY     int
		edgeHits int
	}
	var comps []compInfo
	queue := make([]int, 0, 2048)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if labels[idx] != 0 || mask.AlphaAt(x, y).A < 96 {
				continue
			}
			compID := int32(len(comps) + 1)
			info := compInfo{minX: x, maxX: x, minY: y, maxY: y}
			labels[idx] = compID
			queue = append(queue[:0], idx)

			for head := 0; head < len(queue); head++ {
				cur := queue[head]
				cy, cx := cur/w, cur%w
				info.area++
				if cx < info.minX {
					info.minX = cx
				}
				if cx > info.maxX {
					info.maxX = cx
				}
				if cy < info.minY {
					info.minY = cy
				}
				if cy > info.maxY {
					info.maxY = cy
				}
				if cx <= 2 || cy <= 2 || cx >= w-3 || cy >= h-3 {
					info.edgeHits++
				}

				if cx > 0 {
					ni := cur - 1
					if labels[ni] == 0 && mask.AlphaAt(cx-1, cy).A >= 96 {
						labels[ni] = compID
						queue = append(queue, ni)
					}
				}
				if cx+1 < w {
					ni := cur + 1
					if labels[ni] == 0 && mask.AlphaAt(cx+1, cy).A >= 96 {
						labels[ni] = compID
						queue = append(queue, ni)
					}
				}
				if cy > 0 {
					ni := cur - w
					if labels[ni] == 0 && mask.AlphaAt(cx, cy-1).A >= 96 {
						labels[ni] = compID
						queue = append(queue, ni)
					}
				}
				if cy+1 < h {
					ni := cur + w
					if labels[ni] == 0 && mask.AlphaAt(cx, cy+1).A >= 96 {
						labels[ni] = compID
						queue = append(queue, ni)
					}
				}
			}
			comps = append(comps, info)
		}
	}

	if len(comps) <= 1 {
		return out
	}

	maxArea := 0
	for _, c := range comps {
		if c.area > maxArea {
			maxArea = c.area
		}
	}
	if maxArea == 0 {
		return out
	}

	keep := make([]bool, len(comps)+1)
	marginX := w / 8
	marginY := h / 8
	for i, c := range comps {
		id := i + 1
		if c.area == maxArea {
			keep[id] = true
			continue
		}
		// Keep secondary product items (e.g., matching pair of mugs) only when substantial
		// and not merely a cropped prop clinging to the outer frame border.
		isTiny := c.area < max(250, maxArea/10)
		isEdgeProp := c.edgeHits > 12 && c.area < (maxArea*45)/100 &&
			(c.maxX < marginX*2 || c.minX > w-marginX*2 || c.maxY < marginY*2)
		if !isTiny && !isEdgeProp {
			keep[id] = true
		}
	}

	// Suppress pruned islands along with a small 6px halo around them.
	drop := make([]bool, n)
	for i, lbl := range labels {
		if lbl > 0 && !keep[lbl] {
			drop[i] = true
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if drop[y*w+x] {
				for dy := -6; dy <= 6; dy++ {
					ny := y + dy
					if ny < 0 || ny >= h {
						continue
					}
					for dx := -6; dx <= 6; dx++ {
						nx := x + dx
						if nx < 0 || nx >= w {
							continue
						}
						if labels[ny*w+nx] == 0 {
							out.SetAlpha(nx, ny, color.Alpha{A: 0})
						}
					}
				}
				out.SetAlpha(x, y, color.Alpha{A: 0})
			}
		}
	}
	return out
}

// guidedRefineMask applies fast joint-bilateral filtering only across transition
// pixels so coarse neural boundaries lock tightly onto true RGB product edges.
func guidedRefineMask(src *image.NRGBA, mask *image.Alpha, w, h int) *image.Alpha {
	out := image.NewAlpha(image.Rect(0, 0, w, h))
	const colorSigma2 = 2.0 * 28.0 * 28.0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			centerA := int(mask.AlphaAt(x, y).A)
			if centerA >= 245 && allNeighborsAbove(mask, x, y, w, h, 240) {
				out.SetAlpha(x, y, color.Alpha{A: 255})
				continue
			}
			if centerA <= 10 && allNeighborsBelow(mask, x, y, w, h, 14) {
				out.SetAlpha(x, y, color.Alpha{A: 0})
				continue
			}

			c0 := src.NRGBAAt(x, y)
			var wSum float64
			var aSum float64

			for dy := -2; dy <= 2; dy++ {
				ny := y + dy
				if ny < 0 || ny >= h {
					continue
				}
				for dx := -2; dx <= 2; dx++ {
					nx := x + dx
					if nx < 0 || nx >= w {
						continue
					}
					nc := src.NRGBAAt(nx, ny)
					dr := float64(int(nc.R) - int(c0.R))
					dg := float64(int(nc.G) - int(c0.G))
					db := float64(int(nc.B) - int(c0.B))
					dist2 := dr*dr + dg*dg + db*db
					spatial := 1.0 / (1.0 + 0.25*float64(dx*dx+dy*dy))
					wgt := spatial * math.Exp(-dist2/colorSigma2)
					wSum += wgt
					aSum += wgt * float64(mask.AlphaAt(nx, ny).A)
				}
			}
			if wSum <= 1e-6 {
				out.SetAlpha(x, y, color.Alpha{A: uint8(centerA)})
				continue
			}
			refined := clampFloat(aSum/wSum, 0, 255)
			// Gently push transition band toward clean foreground/background separation.
			t := refined / 255.0
			t = t * t * (3.0 - 2.0*t)
			out.SetAlpha(x, y, color.Alpha{A: uint8(math.Round(t * 255.0))})
		}
	}
	return out
}

// decontaminateEdgePixel estimates local clear background color around a semi-transparent
// boundary pixel and removes background color spill so edges do not halo on dark or white scenes.
func decontaminateEdgePixel(src *image.NRGBA, mask *image.Alpha, x, y, w, h int, c color.NRGBA, a uint8) (uint8, uint8, uint8) {
	var br, bg, bb, count float64
	for dy := -6; dy <= 6; dy += 2 {
		ny := y + dy
		if ny < 0 || ny >= h {
			continue
		}
		for dx := -6; dx <= 6; dx += 2 {
			nx := x + dx
			if nx < 0 || nx >= w {
				continue
			}
			if mask.AlphaAt(nx, ny).A <= 18 {
				bc := src.NRGBAAt(nx, ny)
				br += float64(bc.R)
				bg += float64(bc.G)
				bb += float64(bc.B)
				count++
			}
		}
	}
	if count == 0 {
		return c.R, c.G, c.B
	}
	br /= count
	bg /= count
	bb /= count

	alpha := float64(a) / 255.0
	denom := math.Max(0.35, alpha)
	fr := clampFloat((float64(c.R)-(1.0-alpha)*br)/denom, 0, 255)
	fg := clampFloat((float64(c.G)-(1.0-alpha)*bg)/denom, 0, 255)
	fb := clampFloat((float64(c.B)-(1.0-alpha)*bb)/denom, 0, 255)

	blend := math.Min(1.0, (1.0-alpha)*1.15)
	return uint8(math.Round(float64(c.R)*(1.0-blend) + fr*blend)),
		uint8(math.Round(float64(c.G)*(1.0-blend) + fg*blend)),
		uint8(math.Round(float64(c.B)*(1.0-blend) + fb*blend))
}

func allNeighborsAbove(mask *image.Alpha, x, y, w, h int, thresh uint8) bool {
	if x == 0 || y == 0 || x == w-1 || y == h-1 {
		return false
	}
	return mask.AlphaAt(x-1, y).A > thresh &&
		mask.AlphaAt(x+1, y).A > thresh &&
		mask.AlphaAt(x, y-1).A > thresh &&
		mask.AlphaAt(x, y+1).A > thresh
}

func allNeighborsBelow(mask *image.Alpha, x, y, w, h int, thresh uint8) bool {
	if x == 0 || y == 0 || x == w-1 || y == h-1 {
		return true
	}
	return mask.AlphaAt(x-1, y).A < thresh &&
		mask.AlphaAt(x+1, y).A < thresh &&
		mask.AlphaAt(x, y-1).A < thresh &&
		mask.AlphaAt(x, y+1).A < thresh
}

// foregroundBounds finds the tight bounding box of non-transparent pixels in cutout.
func foregroundBounds(img *image.NRGBA) image.Rectangle {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	minX, minY := w, h
	maxX, maxY := -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if img.NRGBAAt(x, y).A > 20 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < minX || maxY < minY {
		return image.Rect(0, 0, w, h)
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// AnalyzeCutoutQuality inspects foreground coverage and edge contact so sellers
// receive actionable guidance on difficult photos (e.g. white-on-white or cropped edges).
func AnalyzeCutoutQuality(img *image.NRGBA) (float64, string) {
	if img == nil {
		return 0, "Cutout unavailable."
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	total := w * h
	if total == 0 {
		return 0, "Empty image."
	}
	fg := 0
	edgeFG := 0
	edgeTotal := w*2 + h*2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if img.NRGBAAt(x, y).A > 128 {
				fg++
				if x == 0 || y == 0 || x == w-1 || y == h-1 {
					edgeFG++
				}
			}
		}
	}
	ratio := float64(fg) / float64(total)
	edgeRatio := float64(edgeFG) / float64(max(1, edgeTotal))
	switch {
	case ratio < 0.04:
		return ratio, "Very small foreground detected. Consider cropping closer or retrying with higher contrast."
	case ratio > 0.93:
		return ratio, "Product fills almost the entire frame; background separation was minimal."
	case edgeRatio > 0.65:
		return ratio, "Product touches most image borders; Contain fit is recommended so edges are not clipped."
	default:
		return ratio, ""
	}
}

func clampFloat(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
