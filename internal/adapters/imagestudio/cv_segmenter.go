package imagestudio

import (
	"context"
	"image"
	"image/color"
	"math"
	"sort"
)

// CVSegmenter is a pure-Go CPU computer-vision fallback segmenter.
// It models background color clusters from the image perimeter, combines
// perceptual color distance with gradient edges and a centered spatial prior,
// and isolates the primary connected product region.
type CVSegmenter struct{}

func NewCVSegmenter() *CVSegmenter { return &CVSegmenter{} }

func (s *CVSegmenter) Segment(ctx context.Context, src image.Image) (*image.Alpha, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	nrgba := toNRGBA(src)
	b := nrgba.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 16 || h < 16 {
		return nil, "", ErrImageTooSmall
	}

	// If the source image already carries meaningful transparency, preserve it.
	if existing, ok := extractExistingAlpha(nrgba); ok {
		return existing, "alpha-preserved", nil
	}

	bgPalette := sampleBorderPalette(nrgba)
	total := w * h
	prob := make([]float64, total)

	// Compute Lab-like perceptual distance + local gradient cue.
	for y := 0; y < h; y++ {
		if y%64 == 0 && ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		ny := float64(y)/float64(h-1)*2 - 1
		for x := 0; x < w; x++ {
			nx := float64(x)/float64(w-1)*2 - 1
			c := nrgba.NRGBAAt(x, y)
			l1, a1, b1 := rgbToLabApprox(c.R, c.G, c.B)

			minDist := math.MaxFloat64
			for _, bg := range bgPalette {
				dl := l1 - bg.l
				da := a1 - bg.a
				db := b1 - bg.b
				d := math.Sqrt(dl*dl*0.8 + da*da + db*db)
				if d < minDist {
					minDist = d
				}
			}

			grad := localGradient(nrgba, x, y, w, h)
			// Center prior gently favors centered subjects while still letting
			// products extend close to the edges.
			radial := math.Sqrt(nx*nx*0.7 + ny*ny*0.7)
			spatialPrior := math.Max(0, 1.0-radial*0.55)

			colorScore := sigmoid((minDist - 12.0) / 5.5)
			edgeScore := sigmoid((grad - 18.0) / 8.0) * 0.35
			score := colorScore*(0.78+0.22*spatialPrior) + edgeScore*spatialPrior
			if score > 1 {
				score = 1
			}
			prob[y*w+x] = score
		}
	}

	// Flood-fill background from corners/perimeter on low-probability pixels to
	// avoid isolated interior highlights being treated as background.
	fgMask := make([]bool, total)
	for i, p := range prob {
		fgMask[i] = p >= 0.48
	}

	bgVisited := floodFillPerimeterBackground(prob, w, h, 0.56)
	for i := 0; i < total; i++ {
		if bgVisited[i] {
			fgMask[i] = false
		}
	}

	fgMask = keepLargestComponent(fgMask, w, h)
	fgMask = fillSmallHoles(fgMask, w, h)

	// Convert binary + soft probability into smooth alpha mask.
	alpha := image.NewAlpha(image.Rect(0, 0, w, h))
	fgCount := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			var a uint8
			if fgMask[idx] {
				fgCount++
				// Interior pixels are solid 255; border pixels keep soft confidence.
				if isBoundaryPixel(fgMask, x, y, w, h) {
					v := 140 + int(prob[idx]*115)
					if v > 255 {
						v = 255
					}
					a = uint8(v)
				} else {
					a = 255
				}
			}
			alpha.SetAlpha(x, y, color.Alpha{A: a})
		}
	}

	// Guard: if segmentation collapsed (<2% foreground), keep centered elliptical
	// fallback rather than returning an empty cutout.
	if float64(fgCount)/float64(total) < 0.02 {
		return centeredSoftFallback(w, h), "cv-fallback-centered", nil
	}
	return alpha, "cv-fallback", nil
}

func extractExistingAlpha(img *image.NRGBA) (*image.Alpha, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	transparent := 0
	opaque := 0
	total := w * h
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := img.NRGBAAt(x, y).A
			if a < 20 {
				transparent++
			} else if a > 235 {
				opaque++
			}
		}
	}
	if float64(transparent)/float64(total) > 0.05 && float64(opaque)/float64(total) > 0.03 {
		alpha := image.NewAlpha(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				alpha.SetAlpha(x, y, color.Alpha{A: img.NRGBAAt(x, y).A})
			}
		}
		return alpha, true
	}
	return nil, false
}

type labColor struct {
	l, a, b float64
}

func sampleBorderPalette(img *image.NRGBA) []labColor {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	marginX := max(2, w/24)
	marginY := max(2, h/24)
	var samples []labColor
	stepX := max(1, w/32)
	stepY := max(1, h/32)

	addSample := func(x, y int) {
		c := img.NRGBAAt(x, y)
		l, a, bv := rgbToLabApprox(c.R, c.G, c.B)
		samples = append(samples, labColor{l: l, a: a, b: bv})
	}
	for x := 0; x < w; x += stepX {
		addSample(x, 0)
		addSample(x, marginY/2)
		addSample(x, h-1)
		addSample(x, h-1-marginY/2)
	}
	for y := 0; y < h; y += stepY {
		addSample(0, y)
		addSample(marginX/2, y)
		addSample(w-1, y)
		addSample(w-1-marginX/2, y)
	}

	// Quantize into up to 6 representative clusters using greedy farthest-first.
	if len(samples) <= 6 {
		return samples
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].l < samples[j].l })
	clusters := []labColor{samples[len(samples)/2]}
	for len(clusters) < 6 {
		bestIdx := -1
		bestDist := -1.0
		for i, s := range samples {
			minD := math.MaxFloat64
			for _, c := range clusters {
				d := (s.l-c.l)*(s.l-c.l) + (s.a-c.a)*(s.a-c.a) + (s.b-c.b)*(s.b-c.b)
				if d < minD {
					minD = d
				}
			}
			if minD > bestDist {
				bestDist = minD
				bestIdx = i
			}
		}
		if bestIdx < 0 || bestDist < 18.0 {
			break
		}
		clusters = append(clusters, samples[bestIdx])
	}
	return clusters
}

func rgbToLabApprox(r, g, b uint8) (float64, float64, float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0
	l := (0.299*rf + 0.587*gf + 0.114*bf) * 100.0
	a := (rf - gf) * 128.0
	bv := (0.5*(rf+gf) - bf) * 128.0
	return l, a, bv
}

func localGradient(img *image.NRGBA, x, y, w, h int) float64 {
	x0, x1 := max(0, x-1), min(w-1, x+1)
	y0, y1 := max(0, y-1), min(h-1, y+1)
	cL := img.NRGBAAt(x0, y)
	cR := img.NRGBAAt(x1, y)
	cU := img.NRGBAAt(x, y0)
	cD := img.NRGBAAt(x, y1)
	dx := math.Abs(float64(cR.R)-float64(cL.R)) + math.Abs(float64(cR.G)-float64(cL.G)) + math.Abs(float64(cR.B)-float64(cL.B))
	dy := math.Abs(float64(cD.R)-float64(cU.R)) + math.Abs(float64(cD.G)-float64(cU.G)) + math.Abs(float64(cD.B)-float64(cU.B))
	return (dx + dy) / 6.0
}

func floodFillPerimeterBackground(prob []float64, w, h int, fgThreshold float64) []bool {
	visited := make([]bool, w*h)
	queue := make([]int, 0, w*2+h*2)
	push := func(x, y int) {
		idx := y*w + x
		if !visited[idx] && prob[idx] < fgThreshold {
			visited[idx] = true
			queue = append(queue, idx)
		}
	}
	for x := 0; x < w; x++ {
		push(x, 0)
		push(x, h-1)
	}
	for y := 0; y < h; y++ {
		push(0, y)
		push(w-1, y)
	}
	for head := 0; head < len(queue); head++ {
		idx := queue[head]
		x, y := idx%w, idx/w
		if x > 0 {
			push(x-1, y)
		}
		if x+1 < w {
			push(x+1, y)
		}
		if y > 0 {
			push(x, y-1)
		}
		if y+1 < h {
			push(x, y+1)
		}
	}
	return visited
}

func keepLargestComponent(mask []bool, w, h int) []bool {
	total := w * h
	label := make([]int, total)
	bestLabel := 0
	bestCount := 0
	currentLabel := 0
	queue := make([]int, 0, 1024)

	for i := 0; i < total; i++ {
		if !mask[i] || label[i] != 0 {
			continue
		}
		currentLabel++
		label[i] = currentLabel
		queue = append(queue[:0], i)
		for head := 0; head < len(queue); head++ {
			idx := queue[head]
			x, y := idx%w, idx/w
			neighbors := [4]int{-1, -1, -1, -1}
			if x > 0 {
				neighbors[0] = idx - 1
			}
			if x+1 < w {
				neighbors[1] = idx + 1
			}
			if y > 0 {
				neighbors[2] = idx - w
			}
			if y+1 < h {
				neighbors[3] = idx + w
			}
			for _, n := range neighbors {
				if n >= 0 && mask[n] && label[n] == 0 {
					label[n] = currentLabel
					queue = append(queue, n)
				}
			}
		}
		if len(queue) > bestCount {
			bestCount = len(queue)
			bestLabel = currentLabel
		}
	}
	out := make([]bool, total)
	if bestLabel == 0 {
		return out
	}
	for i := 0; i < total; i++ {
		if label[i] == bestLabel {
			out[i] = true
		}
	}
	return out
}

func fillSmallHoles(mask []bool, w, h int) []bool {
	total := w * h
	outside := make([]bool, total)
	queue := make([]int, 0, w*2+h*2)
	push := func(x, y int) {
		idx := y*w + x
		if !mask[idx] && !outside[idx] {
			outside[idx] = true
			queue = append(queue, idx)
		}
	}
	for x := 0; x < w; x++ {
		push(x, 0)
		push(x, h-1)
	}
	for y := 0; y < h; y++ {
		push(0, y)
		push(w-1, y)
	}
	for head := 0; head < len(queue); head++ {
		idx := queue[head]
		x, y := idx%w, idx/w
		if x > 0 {
			push(x-1, y)
		}
		if x+1 < w {
			push(x+1, y)
		}
		if y > 0 {
			push(x, y-1)
		}
		if y+1 < h {
			push(x, y+1)
		}
	}
	out := make([]bool, total)
	for i := 0; i < total; i++ {
		out[i] = mask[i] || !outside[i]
	}
	return out
}

func isBoundaryPixel(mask []bool, x, y, w, h int) bool {
	if x == 0 || y == 0 || x == w-1 || y == h-1 {
		return true
	}
	idx := y*w + x
	return !mask[idx-1] || !mask[idx+1] || !mask[idx-w] || !mask[idx+w]
}

func centeredSoftFallback(w, h int) *image.Alpha {
	alpha := image.NewAlpha(image.Rect(0, 0, w, h))
	cx, cy := float64(w)/2, float64(h)/2
	rx, ry := float64(w)*0.38, float64(h)*0.40
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := (float64(x) - cx) / rx
			dy := (float64(y) - cy) / ry
			d := math.Sqrt(dx*dx + dy*dy)
			var a uint8
			if d < 0.85 {
				a = 255
			} else if d < 1.05 {
				a = uint8((1.05 - d) / 0.20 * 255)
			}
			alpha.SetAlpha(x, y, color.Alpha{A: a})
		}
	}
	return alpha
}

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}
