package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/wecratfs/commerce/web/components"
)

// rupeesToPaise never uses floating point or silently rounds user-entered money.
func rupeesToPaise(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	whole, fraction, decimal := strings.Cut(raw, ".")
	if whole == "" || len(fraction) > 2 || decimal && fraction == "" {
		return 0, errors.New("enter rupees with up to two decimal places")
	}
	for _, r := range whole + fraction {
		if r < '0' || r > '9' {
			return 0, errors.New("enter a non-negative rupee amount")
		}
	}
	if decimal {
		fraction += strings.Repeat("0", 2-len(fraction))
	} else {
		fraction = "00"
	}
	value, err := strconv.ParseInt(whole+fraction, 10, 64)
	if err != nil {
		return 0, errors.New("amount is too large")
	}
	return value, nil
}

func moneyFormRaw(r *http.Request, key string) string {
	rupeeKey := strings.TrimSuffix(key, "_cents") + "_rupees"
	raw := r.FormValue(rupeeKey)
	if _, provided := r.Form[rupeeKey]; provided {
		return strings.TrimSpace(raw)
	}
	return strings.TrimSpace(r.FormValue(key))
}

func moneyFromForm(r *http.Request, key string) (int64, error) {
	rupeeKey := strings.TrimSuffix(key, "_cents") + "_rupees"
	raw := r.FormValue(rupeeKey)
	if _, provided := r.Form[rupeeKey]; provided {
		return rupeesToPaise(raw)
	}
	return strconv.ParseInt(strings.TrimSpace(r.FormValue(key)), 10, 64)
}

func productDescriptionFromForm(r *http.Request) string {
	// Fields are optional for existing clients; retain their descriptions verbatim.
	if _, supplied := r.Form["materials"]; !supplied {
		return r.FormValue("description")
	}
	story := components.ProductStory(r.FormValue("description"))
	for _, spec := range []struct{ key, label string }{{"materials", "Materials"}, {"dimensions", "Dimensions"}, {"care", "Care"}, {"technique", "Technique"}} {
		value := strings.TrimSpace(r.FormValue(spec.key))
		value = strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " ")
		if value != "" {
			story += "\n" + spec.label + ": " + value
		}
	}
	return strings.TrimSpace(story)
}

// Use the draft parser's exact arithmetic to identify a recoverable field.
func sellerProductInputErrorField(r *http.Request) string {
	price, err := moneyFromForm(r, "price_cents")
	if err != nil || price <= 0 {
		return "price_rupees"
	}
	if raw := moneyFormRaw(r, "compare_at_cents"); raw != "" {
		value, err := moneyFromForm(r, "compare_at_cents")
		if err != nil || value < 0 || value > 0 && value < price {
			return "compare_at_rupees"
		}
	}
	if raw := moneyFormRaw(r, "cost_price_cents"); raw != "" {
		value, err := moneyFromForm(r, "cost_price_cents")
		if err != nil || value < 0 {
			return "cost_price_rupees"
		}
	}
	return "image_file"
}
