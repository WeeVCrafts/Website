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
	config := Config{PaymentMode: "razorpay", Environment: "production", PublicURL: "https://shop.example", SecureCookies: true, DatabaseURL: "postgres://app:unique-secret@db/commerce?sslmode=verify-full", S3Secure: true, S3AccessKey: "dedicated", S3SecretKey: "dedicated-secret"}
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

func TestProductionRejectsSandboxPaymentCredentials(t *testing.T) {
	base := Config{Environment: "production", PaymentMode: "razorpay", PublicURL: "https://shop.example", SecureCookies: true, DatabaseURL: "postgres://app:unique-secret@db/commerce?sslmode=verify-full", S3Secure: true, S3AccessKey: "dedicated", S3SecretKey: "dedicated-secret"}
	for _, key := range []string{"rzp_test_fixture", "unknown-provider-key"} {
		cfg := base
		cfg.RazorpayKeyID = key
		if cfg.ValidateSecurity() == nil {
			t.Errorf("production accepted non-live payment key %q", key)
		}
	}
	for _, key := range []string{"", "rzp_live_fixture"} {
		cfg := base
		cfg.RazorpayKeyID = key
		if err := cfg.ValidateSecurity(); err != nil {
			t.Errorf("browsing without configured payment or live provider rejected: %v", err)
		}
	}
}

func TestProductionRejectsSimulatedProviderAndKnownStorageCredentials(t *testing.T) {
	base := Config{Environment: "production", PaymentMode: "razorpay", PublicURL: "https://shop.example", SecureCookies: true, DatabaseURL: "postgres://app:unique-secret@db/commerce?sslmode=verify-full", S3Secure: true, S3AccessKey: "dedicated", S3SecretKey: "dedicated-secret"}
	for _, mode := range []string{"local", "", "unknown"} {
		cfg := base
		cfg.PaymentMode = mode
		if cfg.ValidateSecurity() == nil {
			t.Errorf("production accepted payment provider %q", mode)
		}
	}
	for _, change := range []func(*Config){
		func(c *Config) { c.S3AccessKey = "GK0123456789abcdef01234567" },
		func(c *Config) { c.S3SecretKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" },
	} {
		cfg := base
		change(&cfg)
		if cfg.ValidateSecurity() == nil {
			t.Error("production accepted published storage credentials")
		}
	}
}
func TestPublicURLMustBeExplicitInProductionAndPortRemainsPortable(t *testing.T) {
	t.Setenv("WECRATFS_ENV", " Production ")
	t.Setenv("WECRATFS_PUBLIC_URL", "")
	t.Setenv("WECRATFS_ADDR", "")
	t.Setenv("PORT", "18090")
	cfg := FromEnv()
	if cfg.PublicURL != "" || cfg.Environment != "production" || cfg.Address != ":18090" {
		t.Fatalf("unexpected production config: environment=%q public URL=%q address=%q", cfg.Environment, cfg.PublicURL, cfg.Address)
	}
	if cfg.ValidateSecurity() == nil {
		t.Fatal("production accepted implicit public URL")
	}
}
