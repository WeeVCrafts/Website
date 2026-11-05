package config

import "testing"

func TestCheckoutIsExplicitAndRequiresCompleteProductionConfiguration(t *testing.T) {
	t.Setenv("WECRATFS_CHECKOUT_ENABLED", "")
	if FromEnv().CheckoutEnabled {
		t.Fatal("checkout enabled by default")
	}
	t.Setenv("WECRATFS_CHECKOUT_ENABLED", "true")
	if !FromEnv().CheckoutEnabled {
		t.Fatal("explicit switch ignored")
	}
	base := Config{CheckoutEnabled: true, PublicContentApproved: true, Environment: "production", PaymentMode: "razorpay", RazorpayKeyID: "rzp_live_fixture", RazorpayKeySecret: "fixture-private", RazorpayWebhookSecret: "fixture-webhook", PublicURL: "https://shop.example", SecureCookies: true, DatabaseURL: "postgres://app:unique-secret@db/commerce?sslmode=verify-full", S3Secure: true, S3AccessKey: "dedicated", S3SecretKey: "dedicated-secret"}
	if err := base.ValidateSecurity(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Config){func(c *Config) { c.PublicContentApproved = false }, func(c *Config) { c.RazorpayKeyID = "" }, func(c *Config) { c.RazorpayKeySecret = "" }, func(c *Config) { c.RazorpayWebhookSecret = "" }, func(c *Config) { c.RazorpayKeySecret = " " }} {
		c := base
		mutate(&c)
		if c.ValidateSecurity() == nil {
			t.Fatal("incomplete enabled checkout accepted")
		}
	}
	stage := base
	stage.Staging = true
	stage.StagingUsername = "qa"
	stage.StagingPassword = "dedicated-staging-password-123"
	stage.RazorpayKeyID = "rzp_test_fixture"
	if err := stage.ValidateSecurity(); err != nil {
		t.Fatal(err)
	}
}
