package config

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

type ShippingConfig struct {
	Mode, Email, Password string
	WebhookSecret         string
	PickupLocations       map[int64]string
	ParcelProfiles        map[int64]ShippingParcelProfile
	BookingsEnabled       bool
}

type ShippingParcelProfile struct {
	PostalCode   string `json:"postal_code"`
	PackingGrams int    `json:"packing_grams"`
	PackingCM    int    `json:"packing_cm"`
}

func (c ShippingConfig) validate(staging bool, environment string) error {
	if c.Mode == "" || c.Mode == "disabled" {
		return nil
	}
	if c.Mode != "live" && c.Mode != "sandbox" {
		return errors.New("Shiprocket mode must be disabled, sandbox or live")
	}
	if c.Mode == "live" && (staging || environment != "production") {
		return errors.New("live Shiprocket requires a production deployment")
	}
	if c.Mode == "sandbox" && environment == "production" && !staging {
		return errors.New("Shiprocket sandbox requires an isolated deployment")
	}
	if !strings.Contains(c.Email, "@") || c.Password == "" || len(c.PickupLocations) == 0 {
		return errors.New("Shiprocket requires API credentials and configured seller pickup locations")
	}
	if c.WebhookSecret != "" && len(c.WebhookSecret) < 32 {
		return errors.New("Shiprocket webhook secret must contain at least 32 characters")
	}
	if c.Mode == "live" && len(c.WebhookSecret) < 32 {
		return errors.New("live Shiprocket requires a webhook secret")
	}
	if c.Mode == "live" || c.BookingsEnabled {
		for id := range c.PickupLocations {
			profile := c.ParcelProfiles[id]
			if len(profile.PostalCode) != 6 || profile.PostalCode[0] == '0' || strings.Trim(profile.PostalCode, "0123456789") != "" || profile.PackingGrams < 0 || profile.PackingGrams > 10000 || profile.PackingCM < 0 || profile.PackingCM > 50 {
				return errors.New("Shiprocket requires a valid pickup PIN and packing allowance for each mapped seller")
			}
		}
	}
	return nil
}

func (c Config) ValidateShipping() error {
	return c.Shipping.validate(c.Staging, c.Environment)
}

func parseShippingConfig() ShippingConfig {
	config := ShippingConfig{Mode: strings.TrimSpace(env("WECRATFS_SHIPROCKET_MODE", "disabled")), Email: strings.TrimSpace(env("SHIPROCKET_API_EMAIL", "")), Password: env("SHIPROCKET_API_PASSWORD", ""), WebhookSecret: env("SHIPROCKET_WEBHOOK_SECRET", ""), BookingsEnabled: boolEnv("WECRATFS_SHIPROCKET_BOOKINGS_ENABLED", false)}
	if json.Unmarshal([]byte(env("SHIPROCKET_PARCEL_PROFILES", "{}")), &config.ParcelProfiles) != nil {
		config.ParcelProfiles = nil
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(env("SHIPROCKET_PICKUP_LOCATIONS", "{}")), &raw); err != nil {
		return config
	}
	locations := make(map[int64]string, len(raw))
	for key, value := range raw {
		id, err := strconv.ParseInt(key, 10, 64)
		name := strings.TrimSpace(value)
		if err != nil || id < 1 || strconv.FormatInt(id, 10) != key || name == "" || len(name) > 100 {
			return config
		}
		locations[id] = name
	}
	config.PickupLocations = locations
	return config
}
