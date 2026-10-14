package components

import (
	"math"
	"strconv"
	"strings"

	"github.com/wecratfs/commerce/web/assets"
)

// Only maintained static photographs get build-time variants. Existing media
// service srcsets and seller-uploaded URLs pass through without transformation.
func ResponsiveImageSrcSet(src, existing string) string {
	variants, ok := maintainedImageVariants[strings.Split(src, "?")[0]]
	if !ok {
		return existing
	}
	if existing != "" && strings.Split(existing, "?")[0] != strings.Split(src, "?")[0] {
		// Upgrade only a verified maintained-image candidate set. Never replace
		// authenticated/custom media variants with guessed static paths.
		allowed := map[string]bool{}
		for _, candidate := range strings.Split(variants, ",") {
			fields := strings.Fields(candidate)
			if len(fields) == 2 {
				allowed[strings.Split(fields[0], "?")[0]] = true
			}
		}
		for _, candidate := range strings.Split(existing, ",") {
			fields := strings.Fields(candidate)
			if len(fields) != 2 || !allowed[strings.Split(fields[0], "?")[0]] {
				return existing
			}
		}
	}
	parts := strings.Split(variants, ", ")
	for i, candidate := range parts {
		fields := strings.Fields(candidate)
		if len(fields) == 2 {
			parts[i] = assets.URL(strings.TrimPrefix(fields[0], "/assets/")) + " " + fields[1]
		}
	}
	return strings.Join(parts, ", ")
}

func productImageLoading(priority []bool) string {
	if len(priority) > 0 && priority[0] {
		return "eager"
	}
	return "lazy"
}

func productImagePriority(priority []bool) string {
	if len(priority) > 1 && priority[1] {
		return "high"
	}
	return "auto"
}

func wishlistHeaderClass(count int) string {
	if count > 0 {
		return "wishlist-action is-active"
	}
	return "wishlist-action"
}

func wishlistIcon(count int) string {
	if count > 0 {
		return "favorite"
	}
	return "favorite_border"
}

func productHeartClass(saved bool) string {
	if saved {
		return "heart is-active"
	}
	return "heart"
}

func productHeartIcon(saved bool) string {
	if saved {
		return "favorite"
	}
	return "favorite_border"
}

func productHeartLabel(name string, saved bool) string {
	if saved {
		return "Remove " + name + " from wishlist"
	}
	return "Save " + name
}

func productWishlistValues(saved bool) string {
	if saved {
		return `{"source":"wishlist","context":"product-card"}`
	}
	return "{}"
}

func ratingStars(rating string) int {
	value, err := strconv.ParseFloat(rating, 64)
	if err != nil || value <= 0 {
		return 0
	}
	count := int(math.Round(value))
	if count > 5 {
		return 5
	}
	return count
}
