package imagestudio

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	JobStatusUploaded        = "uploaded"
	JobStatusProcessing      = "processing"
	JobStatusCompleted       = "completed"
	JobStatusFailed          = "failed"
	JobStatusPendingApproval = "pending_approval"
	JobStatusApproved        = "approved"
	JobStatusRejected        = "rejected"

	VariationStatusPendingApproval = "pending_approval"
	VariationStatusApproved        = "approved"
	VariationStatusRejected        = "rejected"

	FitContain = "contain"
	FitCover   = "cover"
	FitFill    = "fill"

	MaxUploadBytes  = 25 << 20
	MaxDimensionPx  = 8192
	MinDimensionPx  = 64
	MaxTotalPixels  = 24000000
	WorkingMaxEdge  = 1400
	StudioCanvasDim = 1200
)

type CompositionParams struct {
	OffsetX        float64 `json:"offset_x"`
	OffsetY        float64 `json:"offset_y"`
	Scale          float64 `json:"scale"`
	FitMode        string  `json:"fit_mode"`
	ShadowStrength float64 `json:"shadow_strength"`
}

func DefaultComposition() CompositionParams {
	return CompositionParams{
		OffsetX:        0,
		OffsetY:        0,
		Scale:          0.80,
		FitMode:        FitContain,
		ShadowStrength: 0.35,
	}
}

func (p CompositionParams) Normalize() CompositionParams {
	if math.IsNaN(p.OffsetX) || math.IsInf(p.OffsetX, 0) {
		p.OffsetX = 0
	}
	if math.IsNaN(p.OffsetY) || math.IsInf(p.OffsetY, 0) {
		p.OffsetY = 0
	}
	if math.IsNaN(p.Scale) || math.IsInf(p.Scale, 0) || p.Scale == 0 {
		p.Scale = 0.80
	}
	if math.IsNaN(p.ShadowStrength) || math.IsInf(p.ShadowStrength, 0) {
		p.ShadowStrength = 0.35
	}
	p.OffsetX = clampFloat(p.OffsetX, -0.45, 0.45)
	p.OffsetY = clampFloat(p.OffsetY, -0.45, 0.45)
	p.Scale = clampFloat(p.Scale, 0.30, 1.40)
	p.ShadowStrength = clampFloat(p.ShadowStrength, 0, 1)
	switch strings.ToLower(strings.TrimSpace(p.FitMode)) {
	case FitCover:
		p.FitMode = FitCover
	case FitFill:
		p.FitMode = FitFill
	default:
		p.FitMode = FitContain
	}
	return p
}

func clampFloat(v, minVal, maxVal float64) float64 {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

type Preset struct {
	Slug        string `json:"slug"`
	Label       string `json:"label"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Recommended bool   `json:"recommended"`
	SwatchHex   string `json:"swatch_hex"`
	DarkSurface bool   `json:"dark_surface"`
}

func AllPresets() []Preset {
	return []Preset{
		{Slug: "pure-white", Label: "Pure White", Category: "clean", Description: "Crisp e-commerce pure white studio backdrop.", SwatchHex: "#ffffff"},
		{Slug: "light-grey", Label: "Light Grey Studio", Category: "studio", Description: "Soft neutral light-grey sweep with natural falloff.", SwatchHex: "#eceef2"},
		{Slug: "minimalist", Label: "Warm Minimalist", Category: "minimalist", Description: "Warm ivory linen studio matching handcrafted collections.", SwatchHex: "#f5efe6"},
		{Slug: "soft-gradient", Label: "Soft Gradient", Category: "gradient", Description: "Gentle pastel cream-to-blush studio gradient.", SwatchHex: "#f7ebe4"},
		{Slug: "dark-studio", Label: "Dark Premium Studio", Category: "dark", Description: "Charcoal studio vignette for metallic and luxury pieces.", SwatchHex: "#1d2028", DarkSurface: true},
		{Slug: "luxury-studio", Label: "Luxury Editorial", Category: "luxury", Description: "Deep heritage emerald-plum studio with warm rim glow.", SwatchHex: "#1e352f", DarkSurface: true},
		{Slug: "natural", Label: "Natural Sage", Category: "natural", Description: "Sunlit organic sage and limestone studio surface.", SwatchHex: "#dfe7de"},
		{Slug: "lifestyle", Label: "Craft Tabletop", Category: "lifestyle", Description: "Warm artisan wood horizon with sunlit plaster wall.", SwatchHex: "#e6d8c5"},
	}
}

func FindPreset(slug string) (Preset, bool) {
	target := strings.ToLower(strings.TrimSpace(slug))
	for _, preset := range AllPresets() {
		if preset.Slug == target {
			return preset, true
		}
	}
	return Preset{}, false
}

type DominantColor struct {
	Hex       string  `json:"hex"`
	R         uint8   `json:"r"`
	G         uint8   `json:"g"`
	B         uint8   `json:"b"`
	Luminance float64 `json:"luminance"`
	Share     float64 `json:"share"`
}

type Recommendation struct {
	CategorySlug       string   `json:"category_slug"`
	CategoryName       string   `json:"category_name"`
	AspectRatio        string   `json:"aspect_ratio"`
	RecommendedPresets []string `json:"recommended_presets"`
	Reasoning          string   `json:"reasoning"`
}

type Job struct {
	ID                int64           `json:"id"`
	ProductID         int64           `json:"product_id"`
	ProductName       string          `json:"product_name"`
	CategorySlug      string          `json:"category_slug"`
	CategoryName      string          `json:"category_name"`
	SellerID          int64           `json:"seller_id"`
	OwnerUserID       int64           `json:"owner_user_id"`
	SourceObjectKey   string          `json:"source_object_key"`
	CutoutObjectKey   string          `json:"cutout_object_key"`
	SourceContentType string          `json:"source_content_type"`
	SourceByteSize    int64           `json:"source_byte_size"`
	SourceWidth       int             `json:"source_width"`
	SourceHeight      int             `json:"source_height"`
	DominantColors    []DominantColor `json:"dominant_colors"`
	AspectRatio       string          `json:"aspect_ratio"`
	Status            string          `json:"status"`
	EngineUsed        string          `json:"engine_used"`
	ErrorMessage      string          `json:"error_message"`
	SourcePreviewURL  string          `json:"source_preview_url"`
	CutoutPreviewURL  string          `json:"cutout_preview_url"`
	Recommendation    Recommendation  `json:"recommendation"`
	Variations        []Variation     `json:"variations"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type Variation struct {
	ID                  int64             `json:"id"`
	JobID               int64             `json:"job_id"`
	PresetSlug          string            `json:"preset_slug"`
	PresetLabel         string            `json:"preset_label"`
	CategoryRecommended bool              `json:"category_recommended"`
	ObjectKey           string            `json:"object_key"`
	ContentType         string            `json:"content_type"`
	ByteSize            int64             `json:"byte_size"`
	Width               int               `json:"width"`
	Height              int               `json:"height"`
	Composition         CompositionParams `json:"composition"`
	Status              string            `json:"status"`
	PublishedMediaID    *int64            `json:"published_media_id,omitempty"`
	PreviewURL          string            `json:"preview_url"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

type ProductOption struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	SKU          string `json:"sku"`
	CategorySlug string `json:"category_slug"`
	CategoryName string `json:"category_name"`
	Status       string `json:"status"`
	ImageURL     string `json:"image_url"`
}

// RecommendPresets picks rule-based background presets from product category,
// dominant product luminance/colors, and aspect ratio.
func RecommendPresets(categorySlug, categoryName, aspectRatio string, colors []DominantColor) Recommendation {
	cat := strings.ToLower(strings.TrimSpace(categorySlug))
	avgLum := 0.5
	if len(colors) > 0 {
		var weightSum, lumSum float64
		for _, c := range colors {
			w := c.Share
			if w <= 0 {
				w = 0.1
			}
			lumSum += c.Luminance * w
			weightSum += w
		}
		if weightSum > 0 {
			avgLum = lumSum / weightSum
		}
	}

	var picks []string
	var reason string

	switch {
	case strings.Contains(cat, "saree") || strings.Contains(cat, "textile") || strings.Contains(cat, "fashion") || strings.Contains(cat, "apparel"):
		picks = []string{"minimalist", "lifestyle", "light-grey", "pure-white"}
		reason = "Handloom & textile pieces pair best with warm minimalist linen and editorial lifestyle surfaces that preserve weave colors."
	case strings.Contains(cat, "brass") || strings.Contains(cat, "metal") || strings.Contains(cat, "electronic") || strings.Contains(cat, "jewel"):
		picks = []string{"dark-studio", "pure-white", "luxury-studio", "light-grey"}
		reason = "Metallic and precision items gain crisp specular contrast on dark studio sweeps and clean white catalog frames."
	case strings.Contains(cat, "pottery") || strings.Contains(cat, "ceramic") || strings.Contains(cat, "furniture") || strings.Contains(cat, "home"):
		picks = []string{"lifestyle", "natural", "minimalist", "light-grey"}
		reason = "Home & ceramic pieces read naturally on craft tabletop and sunlit organic surfaces."
	case strings.Contains(cat, "gift") || strings.Contains(cat, "cosmetic") || strings.Contains(cat, "beauty"):
		picks = []string{"luxury-studio", "soft-gradient", "minimalist", "pure-white"}
		reason = "Gift and care products stand out on soft pastel gradients and luxury studio sweeps."
	case strings.Contains(cat, "art"):
		picks = []string{"light-grey", "minimalist", "pure-white", "lifestyle"}
		reason = "Artworks remain color-accurate on neutral light-grey and warm minimalist gallery walls."
	default:
		picks = []string{"pure-white", "minimalist", "light-grey", "lifestyle"}
		reason = "Balanced catalog white and warm minimalist studio backdrops suit general handcrafted goods."
	}

	// Contrast safeguard: avoid recommending a bright white primary preset for very light products,
	// or a dark studio primary preset for very dark products.
	if avgLum > 0.78 && picks[0] == "pure-white" {
		picks = []string{"light-grey", "minimalist", "dark-studio", "pure-white"}
		reason += " Adjusted primary backdrop to light grey for stronger edge separation against light product tones."
	} else if avgLum < 0.22 && (picks[0] == "dark-studio" || picks[0] == "luxury-studio") {
		picks = []string{"minimalist", "light-grey", "pure-white", "soft-gradient"}
		reason += " Adjusted primary backdrop to warm minimalist for clear silhouette contrast around dark product tones."
	}

	if aspectRatio == "portrait" {
		reason += " Portrait framing works well with centered 80% scale."
	} else if aspectRatio == "landscape" {
		reason += " Wide framing uses contained margins to preserve edge details."
	}

	return Recommendation{
		CategorySlug:       categorySlug,
		CategoryName:       categoryName,
		AspectRatio:        aspectRatio,
		RecommendedPresets: picks,
		Reasoning:          fmt.Sprintf("%s", reason),
	}
}
