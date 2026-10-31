package config

import "testing"

func TestStagingRequiresIsolatedAccessAndSandbox(t *testing.T) {
	c := Config{Environment: "production", Staging: true, StagingUsername: "qa", StagingPassword: "dedicated-password-long-enough", PublicURL: "https://staging.example", SecureCookies: true, DatabaseURL: "postgres://app:dedicated-password@db/qa?sslmode=verify-full", S3Secure: true, S3AccessKey: "staging", S3SecretKey: "dedicated", PaymentMode: "razorpay", RazorpayKeyID: "rzp_test_fixture"}
	if err := c.ValidateSecurity(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.StagingPassword = "" },
		func(c *Config) { c.RazorpayKeyID = "rzp_live_fixture" },
		func(c *Config) { c.Environment = "development" },
		func(c *Config) { c.SMTPAddress = "smtp.example:587" },
		func(c *Config) { c.ResendAPIKey = "fixture" },
	} {
		unsafe := c
		mutate(&unsafe)
		if unsafe.ValidateSecurity() == nil {
			t.Fatal("unsafe staging accepted")
		}
	}
}

func TestProductionRejectsDevelopmentConfiguration(t *testing.T) {
	config := Config{Environment: "production", PublicURL: "https://shop.example", SecureCookies: true, DatabaseURL: "postgres://app:unique-secret@db/commerce?sslmode=verify-full", S3Secure: true, S3AccessKey: "dedicated", S3SecretKey: "dedicated-secret"}
	if err := config.ValidateSecurity(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Config){func(c *Config) { c.SecureCookies = false }, func(c *Config) { c.PublicURL = "http://shop.example" }, func(c *Config) { c.LocalMode = true }, func(c *Config) { c.DatabaseURL = "postgres://wecratfs:wecratfs@db/commerce" }, func(c *Config) { c.DatabaseURL = "postgres://app:unique-secret@db/commerce?sslmode=disable" }, func(c *Config) { c.S3Secure = false }, func(c *Config) { c.S3SecretKey = "wecratfs-local-only" }} {
		altered := config
		change(&altered)
		if altered.ValidateSecurity() == nil {
			t.Fatal("unsafe configuration accepted")
		}
	}
}
func TestProductionDisablesDemoFlags(t *testing.T) {
	t.Setenv("WECRATFS_ENV", "production")
	t.Setenv("WECRATFS_LOCAL_MODE", "true")
	t.Setenv("WECRATFS_NO_PAYMENT_DEMO", "true")
	cfg := FromEnv()
	if cfg.LocalMode || cfg.NoPaymentDemo {
		t.Fatal("production enabled demo features")
	}
}
