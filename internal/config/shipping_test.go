package config

import "testing"

func TestShiprocketConfigurationSeparatesSandboxAndLive(t *testing.T) {
	t.Setenv("SHIPROCKET_API_EMAIL", "qa@example.com")
	t.Setenv("SHIPROCKET_API_PASSWORD", "fixture-only")
	t.Setenv("SHIPROCKET_PICKUP_LOCATIONS", `{"12":"Seller Depot"}`)
	t.Setenv("WECRATFS_SHIPROCKET_MODE", "sandbox")
	config := parseShippingConfig()
	if config.PickupLocations[12] != "Seller Depot" || config.validate(true, "production") != nil || config.validate(false, "production") == nil {
		t.Fatal("sandbox configuration was not isolated from live production")
	}
	config.Mode = "live"
	if config.validate(false, "production") == nil {
		t.Fatal("accepted live mode without webhook secret")
	}
	config.WebhookSecret = "fixture-only-not-an-actual-secret"
	if config.validate(false, "production") == nil {
		t.Fatal("accepted live mode without pickup profile")
	}
	config.ParcelProfiles = map[int64]ShippingParcelProfile{12: {PostalCode: "110030"}}
	if config.validate(true, "production") == nil || config.validate(false, "development") == nil || config.validate(false, "production") != nil {
		t.Fatal("live Shiprocket escaped the production configuration gate")
	}
}
