package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"slices"
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
			result = append(result, viewmodels.StorefrontDifference{Field: key, Saved: a[key], Proposed: b[key]})
			if len(result) == 100 {
				break
			}
		}
	}
	return result
}
