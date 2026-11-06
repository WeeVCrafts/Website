package pages

import (
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func storefrontDisplaySections(p viewmodels.CustomerPage, sections []storefront.Section) []storefront.Section {
	if p.Route == "/" {
		return homepageSections(sections)
	}
	return storefront.OrderedSections(sections)
}

func homepageTrustAnchor(sections []storefront.Section) string {
	var collections string
	for _, s := range storefront.OrderedSections(sections) {
		if s.Hidden {
			continue
		}
		if s.Type == "collections" {
			collections = s.ID
		}
	}
	return collections
}

// Present saved releases in the requested order without rewriting their data.
func homepageSections(sections []storefront.Section) []storefront.Section {
	ordered := storefront.OrderedSections(sections)
	arrivals, featured := -1, -1
	for i, section := range ordered {
		if section.Hidden || section.Type != "products" {
			continue
		}
		if section.ID == "arrivals" {
			arrivals = i
		}
		if section.ID == "featured" {
			featured = i
		}
	}
	if arrivals < 0 || featured < 0 || arrivals == featured-1 {
		return ordered
	}
	item := ordered[arrivals]
	ordered = append(ordered[:arrivals], ordered[arrivals+1:]...)
	if arrivals < featured {
		featured--
	}
	ordered = append(ordered, storefront.Section{})
	copy(ordered[featured+1:], ordered[featured:])
	ordered[featured] = item
	return ordered
}
