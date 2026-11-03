package components

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/assets"
)

// MediaDeliveryURL changes only delivery caching, never stored release references.
func MediaDeliveryURL(src string) string {
	if !strings.HasPrefix(src, "/media/storefront/") {
		return src
	}
	u, err := url.Parse(src)
	if err != nil {
		return src
	}
	q := u.Query()
	q.Set("v", "2")
	u.RawQuery = q.Encode()
	return u.String()
}

// Maintained static photographs use build-time variants; published storefront
// photographs use their saved delivery variants. Custom candidate sets stay intact.
func ResponsiveImageSrcSet(src, existing string) string {
	if strings.HasPrefix(src, "/media/storefront/") && existing == "" {
		u, err := url.Parse(MediaDeliveryURL(src))
		if err != nil {
			return ""
		}
		original := u.String()
		q := u.Query()
		q.Set("w", "480")
		u.RawQuery = q.Encode()
		small := u.String()
		q.Set("w", "960")
		u.RawQuery = q.Encode()
		return small + " 480w, " + u.String() + " 960w, " + original + " 1920w"
	}
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
	return "Save " + name + " to wishlist"
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

// Named options require explicit selection before purchase.
func ProductCardCanAdd(product viewmodels.Product) bool {
	return product.InStock && len(product.Variants) == 1 && product.Variants[0].InStock && strings.EqualFold(product.Variants[0].Name, "Default variant")
}

// Keep compact homepage cards compact after a wishlist fragment update.
func productCardWishlistValues(saved, actions bool) string {
	if saved {
		return fmt.Sprintf(`{"source":"wishlist","context":"product-card","card_actions":"%t"}`, actions)
	}
	return fmt.Sprintf(`{"card_actions":"%t"}`, actions)
}

func LegacyCategoryLabel(category string) string {
	switch category {
	case "natural-care":
		return "Natural care"
	case "slow-living":
		return "Slow living"
	case "home-wellbeing":
		return "Home wellbeing"
	}
	return "Categories"
}

func storefrontLogo(p viewmodels.CustomerPage) string {
	if p.Storefront != nil && p.Storefront.Theme.Logo != "" {
		return p.Storefront.Theme.Logo
	}
	return "/assets/images/brand/weevcrafts-logo.png"
}

func storefrontTrustIcon(title string) string {
	if icon := map[string]string{"Crafted in India": "handyman", "Secure Payments": "verified_user", "Returns & Refunds": "assignment_return", "Domestic Delivery": "local_shipping", "Artisan made": "groups"}[title]; icon != "" {
		return icon
	}
	return "verified_user"
}
