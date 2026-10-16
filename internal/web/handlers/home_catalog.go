package handlers

import (
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/components"
	"net/http"
)

func (h *LiveStorefrontHandler) loadHomeCatalog(r *http.Request, p *viewmodels.CustomerPage) error {
	selected := r.URL.Query().Get("home_category")
	discovery := components.NavigationCategories(*p)
	if len(discovery) > 0 {
		p.HomeSelectedCategory = discovery[0].Slug
		for _, c := range discovery {
			if c.Slug == selected {
				p.HomeSelectedCategory = selected
			}
		}
	}
	sections := []struct {
		options ports.ProductSearchOptions
		target  *[]viewmodels.Product
	}{
		{ports.ProductSearchOptions{Limit: 6}, &p.Products},
		{ports.ProductSearchOptions{Limit: 8, Sort: "newest"}, &p.HomeNewArrivals},
		{ports.ProductSearchOptions{Limit: 5, CategorySlug: p.HomeSelectedCategory}, &p.HomeCategoryProducts},
		{ports.ProductSearchOptions{Limit: 6, CategorySlug: "gifts", PriceRange: "under-1500"}, &p.HomeGifts},
	}
	for i, section := range sections {
		if i == 2 && p.HomeSelectedCategory == "" {
			continue
		}
		section.options.InStock = true
		section.options.DatabaseOnly = true
		products, _, _, err := h.search.SearchWithFilters(r.Context(), section.options)
		if err != nil {
			return err
		}
		*section.target = h.productViews(r.Context(), products)
	}
	return nil
}
