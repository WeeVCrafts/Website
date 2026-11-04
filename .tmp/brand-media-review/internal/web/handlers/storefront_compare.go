package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"slices"
	"strconv"
	"strings"
)

func storefrontDifferences(saved, proposed storefront.Config) []viewmodels.StorefrontDifference {
	flatten := func(c storefront.Config) map[string]string {
		data, _ := json.Marshal(c)
		var value any
		_ = json.Unmarshal(data, &value)
		result := map[string]string{}
		var walk func(string, any)
		walk = func(path string, v any) {
			switch item := v.(type) {
			case map[string]any:
				for k, value := range item {
					walk(strings.TrimPrefix(path+" / "+strings.ReplaceAll(k, "_", " "), " / "), value)
				}
			case []any:
				for i, value := range item {
					key := fmt.Sprint(i + 1)
					if m, ok := value.(map[string]any); ok {
						for _, candidate := range []string{"id", "slug", "key"} {
							if id, ok := m[candidate].(string); ok && id != "" {
								key = id
								break
							}
						}
					}
					walk(path+" / "+key, value)
				}
			default:
				result[path] = fmt.Sprint(item)
			}
		}
		walk("", value)
		return result
	}
	a, b := flatten(saved), flatten(proposed)
	labels := map[string]string{"homepage": "Homepage", "slides": "Hero slides", "navigation": "Header navigation", "footer groups": "Footer", "settings": "Store settings", "templates": "Page templates", "pages": "Content pages", "theme": "Brand appearance", "promotions": "Promotions"}
	for _, c := range []storefront.Config{saved, proposed} {
		for _, s := range c.Homepage {
			label := s.Title
			if label == "" {
				label = s.Type
			}
			labels[s.ID] = label
		}
		for _, s := range c.Slides {
			labels[s.ID] = s.Title
		}
		for _, s := range c.Categories {
			labels[s.Slug] = s.Name
		}
		for _, s := range c.Templates {
			label := s.Kind + " page"
			if s.Match != "" {
				label += " (" + s.Match + ")"
			}
			labels[s.ID] = label
			for _, section := range s.Sections {
				if section.Title != "" {
					labels[section.ID] = section.Title
				}
			}
		}
		for _, s := range c.Pages {
			labels[s.Slug] = s.Title
		}
	}
	readable := func(key string) string {
		parts := strings.Split(key, " / ")
		for i, part := range parts {
			if name := labels[part]; name != "" {
				parts[i] = name
			}
		}
		return strings.Join(parts, " / ")
	}

	keys := []string{}
	for key := range a {
		keys = append(keys, key)
	}
	for key := range b {
		if _, exists := a[key]; !exists {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	var result []viewmodels.StorefrontDifference
	for _, key := range keys {
		if a[key] != b[key] {
			result = append(result, viewmodels.StorefrontDifference{Field: readable(key), Saved: readableDifference(key, a[key]), Proposed: readableDifference(key, b[key])})
			if len(result) == 100 {
				break
			}
		}
	}
	return result
}

func readableDifference(key, value string) string {
	if strings.Contains(key, "cents") || strings.HasSuffix(key, " / min price") || strings.HasSuffix(key, " / max price") {
		if cents, e := strconv.ParseInt(value, 10, 64); e == nil && cents >= 0 {
			return storefrontMoney(cents)
		}
	}
	switch value {
	case "<nil>":
		return "Not set"
	case "true":
		return "Yes"
	case "false":
		return "No"
	default:
		return value
	}
}
