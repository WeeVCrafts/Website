package imagestudio

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"

	"golang.org/x/image/draw"

	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
)

func ComposeProduct(ctx context.Context, cutoutPNG []byte, presetSlug string, params domainstudio.CompositionParams) ([]byte, string, int, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", 0, 0, err
	}
	decoded, err := png.Decode(bytes.NewReader(cutoutPNG))
	if err != nil {
		return nil, "", 0, 0, ErrInvalidCutout
	}
	cutout := toNRGBA(decoded)
	tight := foregroundBounds(cutout)
	cropped := image.NewNRGBA(image.Rect(0, 0, tight.Dx(), tight.Dy()))
	draw.Draw(cropped, cropped.Bounds(), cutout, tight.Min, draw.Src)

	params = params.Normalize()
	size := domainstudio.StudioCanvasDim
	canvas := renderBackground(presetSlug, size, size)

	// Determine target product box inside the square studio canvas, bounded to prevent OOM on extreme aspect ratios.
	cw, ch := cropped.Bounds().Dx(), cropped.Bounds().Dy()
	maxComposeDim := size * 4
	var targetW, targetH int
	switch params.FitMode {
	case domainstudio.FitCover:
		scale := math.Max(float64(size)/float64(cw), float64(size)/float64(ch)) * params.Scale
		if maxDim := math.Max(float64(cw)*scale, float64(ch)*scale); maxDim > float64(maxComposeDim) {
			scale *= float64(maxComposeDim) / maxDim
		}
		targetW = max(1, int(math.Round(float64(cw)*scale)))
		targetH = max(1, int(math.Round(float64(ch)*scale)))
	case domainstudio.FitFill:
		targetW = min(maxComposeDim, max(1, int(math.Round(float64(size)*params.Scale))))
		targetH = min(maxComposeDim, max(1, int(math.Round(float64(size)*params.Scale))))
	default: // contain
		scale := math.Min(float64(size)/float64(cw), float64(size)/float64(ch)) * params.Scale
		if maxDim := math.Max(float64(cw)*scale, float64(ch)*scale); maxDim > float64(maxComposeDim) {
			scale *= float64(maxComposeDim) / maxDim
		}
		targetW = max(1, int(math.Round(float64(cw)*scale)))
		targetH = max(1, int(math.Round(float64(ch)*scale)))
	}

	scaledProduct := image.NewNRGBA(image.Rect(0, 0, targetW, targetH))
	draw.CatmullRom.Scale(scaledProduct, scaledProduct.Bounds(), cropped, cropped.Bounds(), draw.Src, nil)

	centerX := float64(size)/2 + params.OffsetX*float64(size)
	centerY := float64(size)/2 + params.OffsetY*float64(size)
	dstX := int(math.Round(centerX - float64(targetW)/2))
	dstY := int(math.Round(centerY - float64(targetH)/2))
	dstRect := image.Rect(dstX, dstY, dstX+targetW, dstY+targetH)

	if params.ShadowStrength > 0.01 && presetSlug != "pure-white" {
		drawContactShadow(canvas, dstRect, params.ShadowStrength)
	} else if params.ShadowStrength > 0.01 {
		drawContactShadow(canvas, dstRect, params.ShadowStrength*0.55)
	}

	// Composite product onto background without altering product colors/details.
	draw.Draw(canvas, dstRect, scaledProduct, image.Point{}, draw.Over)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, canvas, &jpeg.Options{Quality: 92}); err != nil {
		return nil, "", 0, 0, err
	}
	return out.Bytes(), "image/jpeg", size, size, nil
}

func renderBackground(presetSlug string, w, h int) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	switch presetSlug {
	case "pure-white":
		draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)
	case "light-grey":
		paintRadialStudio(canvas, color.RGBA{247, 248, 250, 255}, color.RGBA{222, 226, 232, 255}, 0.5, 0.44)
	case "dark-studio":
		paintRadialStudio(canvas, color.RGBA{56, 61, 72, 255}, color.RGBA{20, 22, 27, 255}, 0.5, 0.40)
	case "soft-gradient":
		paintLinearGradient(canvas, color.RGBA{251, 244, 238, 255}, color.RGBA{241, 223, 214, 255})
	case "luxury-studio":
		paintRadialStudio(canvas, color.RGBA{52, 82, 74, 255}, color.RGBA{21, 35, 31, 255}, 0.5, 0.42)
		paintHorizonFloor(canvas, color.RGBA{18, 28, 25, 90}, 0.72)
	case "natural":
		paintLinearGradient(canvas, color.RGBA{234, 241, 232, 255}, color.RGBA{210, 222, 208, 255})
		paintHorizonFloor(canvas, color.RGBA{190, 200, 184, 120}, 0.75)
	case "lifestyle":
		paintLinearGradient(canvas, color.RGBA{247, 241, 233, 255}, color.RGBA{235, 224, 208, 255})
		paintHorizonFloor(canvas, color.RGBA{214, 194, 168, 175}, 0.72)
	default: // minimalist
		paintRadialStudio(canvas, color.RGBA{252, 249, 243, 255}, color.RGBA{237, 228, 215, 255}, 0.5, 0.45)
	}
	return canvas
}

func paintRadialStudio(img *image.RGBA, inner, outer color.RGBA, cxRatio, cyRatio float64) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	cx, cy := float64(w)*cxRatio, float64(h)*cyRatio
	maxR := math.Hypot(float64(w)*0.65, float64(h)*0.65)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := math.Min(1, math.Hypot(float64(x)-cx, float64(y)-cy)/maxR)
			t = t * t * (3 - 2*t)
			img.SetRGBA(x, y, lerpRGBA(inner, outer, t))
		}
	}
}

func paintLinearGradient(img *image.RGBA, top, bottom color.RGBA) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	for y := 0; y < h; y++ {
		t := float64(y) / float64(max(1, h-1))
		c := lerpRGBA(top, bottom, t)
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func paintHorizonFloor(img *image.RGBA, floorColor color.RGBA, horizonRatio float64) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	startY := int(float64(h) * horizonRatio)
	for y := startY; y < h; y++ {
		fade := math.Min(1, float64(y-startY)/float64(max(1, h/18)))
		alpha := float64(floorColor.A) / 255.0 * fade
		for x := 0; x < w; x++ {
			bg := img.RGBAAt(x, y)
			img.SetRGBA(x, y, blendAlphaRGBA(bg, floorColor, alpha))
		}
	}
}

func drawContactShadow(img *image.RGBA, productRect image.Rectangle, strength float64) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	cx := float64(productRect.Min.X+productRect.Max.X) / 2
	cy := float64(productRect.Max.Y) - float64(productRect.Dy())*0.03
	rx := math.Max(20, float64(productRect.Dx())*0.42)
	ry := math.Max(10, float64(productRect.Dy())*0.065)

	minX := max(0, int(cx-rx*1.3))
	maxX := min(w-1, int(cx+rx*1.3))
	minY := max(0, int(cy-ry*1.4))
	maxY := min(h-1, int(cy+ry*1.4))

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx := (float64(x) - cx) / rx
			dy := (float64(y) - cy) / ry
			d2 := dx*dx + dy*dy
			if d2 >= 1.5 {
				continue
			}
			falloff := math.Exp(-d2 * 2.4)
			alpha := strength * 0.28 * falloff
			bg := img.RGBAAt(x, y)
			img.SetRGBA(x, y, blendAlphaRGBA(bg, color.RGBA{18, 18, 22, 255}, alpha))
		}
	}
}

func lerpRGBA(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 255,
	}
}

func blendAlphaRGBA(dst, src color.RGBA, alpha float64) color.RGBA {
	if alpha <= 0 {
		return dst
	}
	if alpha >= 1 {
		return src
	}
	return color.RGBA{
		R: uint8(float64(dst.R)*(1-alpha) + float64(src.R)*alpha),
		G: uint8(float64(dst.G)*(1-alpha) + float64(src.G)*alpha),
		B: uint8(float64(dst.B)*(1-alpha) + float64(src.B)*alpha),
		A: 255,
	}
}
