package pages

import (
	"net/url"
	"strings"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/components"
)

type activeFilter struct{ Label, URL string }

func activeFilters(p viewmodels.CustomerPage) []activeFilter {
	chips := []activeFilter{}
	for _, key := range []string{"q", "category", "brand", "price", "availability"} {
		for _, value := range p.Filters[key] {
			if value == "" {
				continue
			}
			label := value
			switch key {
			case "q":
				label = "Search: " + value
			case "category":
				for _, item := range p.Categories {
					if item.Slug == value || equivalentTextileCategory(item.Slug, value) {
						label = item.Name
						break
					}
				}
				label = "Category: " + label
			case "brand":
				for _, item := range p.Brands {
					if item.Slug == value {
						label = item.Name
						break
					}
				}
				label = "Maker: " + label
			case "price":
				if name := map[string]string{"under-1000": "Under INR 1,000", "1000-2500": "INR 1,000–2,500", "2500-5000": "INR 2,500–5,000", "over-5000": "Above INR 5,000"}[value]; name != "" {
					label = name
				}
			case "availability":
				label = "In stock"
			}
			values := url.Values{}
			for other, entries := range p.Filters {
				if other == "page" {
					continue
				}
				for _, entry := range entries {
					if other != key || entry != value {
						values.Add(other, entry)
					}
				}
			}
			path := "/products"
			if encoded := values.Encode(); encoded != "" {
				path += "?" + encoded
			}
			chips = append(chips, activeFilter{Label: label, URL: path})
		}
	}
	return chips
}

func showVariantChoice(product viewmodels.Product) bool {
	return len(product.Variants) > 1 || len(product.Variants) == 1 && !strings.EqualFold(product.Variants[0].Name, "Default variant")
}

func availableProducts(products []viewmodels.Product, exclude string, limit int) []viewmodels.Product {
	result := []viewmodels.Product{}
	for _, product := range products {
		if product.InStock && product.Slug != exclude {
			result = append(result, product)
			if len(result) == limit {
				break
			}
		}
	}
	return result
}

func accountOrders(orders []viewmodels.Order) []viewmodels.Order {
	result := []viewmodels.Order{}
	for _, order := range orders {
		tone := components.StatusTone(order.Status)
		if tone == "warning" || tone == "danger" && strings.Contains(strings.ToLower(order.Status), "failed") {
			result = append(result, order)
		}
	}
	for _, order := range orders {
		tone := components.StatusTone(order.Status)
		if tone != "warning" && !(tone == "danger" && strings.Contains(strings.ToLower(order.Status), "failed")) {
			result = append(result, order)
		}
	}
	if len(result) > 3 {
		result = result[:3]
	}
	return result
}
