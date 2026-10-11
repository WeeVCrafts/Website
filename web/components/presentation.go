package components

import (
	"strings"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

// StatusTone is shared by customer history and operational queues. Text remains
// the primary signal; color supplements it rather than implying every state is success.
func StatusTone(status string) string {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(status), " ", "_")) {
	case "failed", "payment_failed", "cancelled", "canceled", "rejected", "suspended", "closed", "refunded":
		return "danger"
	case "pending", "pending_payment", "pending_review", "processing", "in_progress", "waiting_customer", "requested", "under_review":
		return "warning"
	case "paid", "approved", "active", "available", "delivered", "resolved", "completed", "success":
		return "success"
	default:
		return "neutral"
	}
}

func StatusLabel(status string) string {
	label := strings.ReplaceAll(strings.TrimSpace(status), "_", " ")
	if label == "" {
		return "Not set"
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

func DeliveryLocation(p viewmodels.CustomerPage) string {
	if p.IsCustomer && p.Address.City != "" {
		if p.Address.State != "" {
			return p.Address.City + ", " + p.Address.State
		}
		return p.Address.City
	}
	return "Choose delivery location"
}

func ProductImage(url string) string {
	if strings.TrimSpace(url) == "" {
		return "/assets/images/product-unavailable.svg"
	}
	return url
}

func CustomerOption(name string) string {
	if strings.EqualFold(strings.TrimSpace(name), "Default variant") {
		return "Standard"
	}
	return name
}

type Specification struct{ Name, Value string }

// Product specifications are maintained in the existing seller-authored
// description, avoiding speculative facts and a second competing data source.
func ProductSpecifications(description string) []Specification {
	result := []Specification{}
	for _, line := range strings.Split(description, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(value) == "" {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "Materials" || key == "Dimensions" || key == "Care" || key == "Technique" {
			result = append(result, Specification{key, strings.TrimSpace(value)})
		}
	}
	return result
}

func SpecificationValue(description, key string) string {
	for _, item := range ProductSpecifications(description) {
		if item.Name == key {
			return item.Value
		}
	}
	return ""
}

func ProductStory(description string) string {
	lines := []string{}
	for _, line := range strings.Split(description, "\n") {
		key, _, found := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		if found && (key == "Materials" || key == "Dimensions" || key == "Care" || key == "Technique") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func confirmationTone(status string) string {
	if status == "open" {
		return "warning"
	}
	return "neutral"
}
