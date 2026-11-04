package pages

import "github.com/wecratfs/commerce/internal/domain/storefront"

func homepageTrustAnchor(sections []storefront.Section) string {
	var firstProduct string
	for _, s := range storefront.OrderedSections(sections) {
		if s.Hidden {
			continue
		}
		if s.Type == "categories" {
			return s.ID
		}
		if firstProduct == "" && s.Type == "products" {
			firstProduct = s.ID
		}
	}
	return firstProduct
}
