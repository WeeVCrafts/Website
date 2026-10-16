package storefront

import (
	"fmt"
	"regexp"
	"slices"
	"time"
)

type Promotion struct {
	ID          int64      `json:"id"`
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Kind        string     `json:"kind"`
	Value       int64      `json:"value"`
	Status      string     `json:"status"`
	StartsAt    *time.Time `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at"`
	UsageLimit  int64      `json:"usage_limit"`
	TargetMode  string     `json:"target_mode"`
	ProductIDs  []int64    `json:"product_ids"`
	Coupons     []Coupon   `json:"coupons"`
}
type Coupon struct {
	ID         int64  `json:"id"`
	Code       string `json:"code"`
	UsageLimit int64  `json:"usage_limit"`
	Active     bool   `json:"active"`
}

var couponPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{2,39}$`)

func validatePromotions(promotions []Promotion) error {
	keys := map[string]bool{}
	codes := map[string]bool{}
	if len(promotions) > 300 {
		return fmt.Errorf("%w: too many promotions", ErrValidation)
	}
	for _, p := range promotions {
		if p.Key == "" || keys[p.Key] || len(p.Name) < 3 || len(p.Name) > 100 || !slices.Contains([]string{"percentage", "fixed", "free_shipping"}, p.Kind) || !slices.Contains([]string{"draft", "active", "archived"}, p.Status) || p.Value < 0 || p.Kind == "percentage" && (p.Value < 1 || p.Value > 100) || p.Kind == "fixed" && p.Value < 1 || p.UsageLimit < 0 || !slices.Contains([]string{"all", "products"}, p.TargetMode) || p.TargetMode == "products" && len(p.ProductIDs) == 0 {
			return fmt.Errorf("%w: check promotion name, amount, status and eligibility", ErrValidation)
		}
		keys[p.Key] = true
		if p.StartsAt != nil && p.EndsAt != nil && !p.StartsAt.Before(*p.EndsAt) {
			return fmt.Errorf("%w: promotion end must follow its start", ErrValidation)
		}
		for _, c := range p.Coupons {
			if !couponPattern.MatchString(c.Code) || codes[c.Code] || c.UsageLimit < 0 {
				return fmt.Errorf("%w: use unique 3–40 character coupon codes and valid limits", ErrValidation)
			}
			codes[c.Code] = true
		}
	}
	return nil
}
