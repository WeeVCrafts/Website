package components

import (
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"slices"
)

func NavigationCategories(p viewmodels.CustomerPage) []viewmodels.Category {
	result := []viewmodels.Category{}
	for _, category := range p.Categories {
		if category.ShowInNavigation || p.Mock {
			result = append(result, category)
		}
	}
	return result
}

// Keep a hidden category's saved label and selected control on an existing
// filtered URL without adding it back to general discovery navigation.
func FilterCategories(p viewmodels.CustomerPage) []viewmodels.Category {
	result := []viewmodels.Category{}
	for _, category := range p.Categories {
		if category.ShowInNavigation || p.Mock || category.Slug == p.Category || slices.Contains(p.Filters["category"], category.Slug) {
			result = append(result, category)
		}
	}
	return result
}
