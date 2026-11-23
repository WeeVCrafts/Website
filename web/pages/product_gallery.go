package pages

import (
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/components"
)

// Match the rendered media list, including the cover only once. Single-image
// pages omit navigation space before scripts run as well as after enhancement.
func productGalleryCount(p viewmodels.CustomerPage) int {
	count := 1
	for _, media := range p.Product.Media {
		if media.URL != p.Product.ImageURL {
			count++
		}
	}
	if components.HasProductDimensions(p.Product) {
		count++
	}
	return count
}
