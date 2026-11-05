package config

import (
	"errors"
	"net/url"
	"strings"
)

// ValidateSecurity rejects development defaults before connecting or serving.
func (c Config) ValidateSecurity() error {
	if c.OperationalMetricsToken != "" && len(c.OperationalMetricsToken) < 32 {
		return errors.New("operational metrics token must contain at least 32 characters")
	}
	if c.Staging {
		if c.Environment != "production" || c.StagingUsername == "" || len(c.StagingPassword) < 24 {
			return errors.New("staging requires production safeguards and dedicated access credentials of at least 24 characters")
		}
		if c.PaymentMode != "razorpay" || !strings.HasPrefix(c.RazorpayKeyID, "rzp_test_") {
			return errors.New("staging requires Razorpay test credentials")
		}
		if c.ResendAPIKey != "" || c.ResendAPIKeyFile != "" || c.SMTPAddress != "" {
			return errors.New("staging email delivery must remain disabled until recipient isolation is configured")
		}
	}
	if !strings.EqualFold(c.Environment, "production") {
		return nil
	}
	if c.CheckoutEnabled {
		if !c.PublicContentApproved {
			return errors.New("checkout requires approved public content")
		}
		if strings.TrimSpace(c.RazorpayKeyID) == "" || strings.TrimSpace(c.RazorpayKeySecret) == "" || strings.TrimSpace(c.RazorpayWebhookSecret) == "" {
			return errors.New("checkout requires complete Razorpay order and webhook credentials")
		}
	}
	public, err := url.Parse(c.PublicURL)
	if err != nil || public.Scheme != "https" || public.Host == "" || public.User != nil {
		return errors.New("production requires an HTTPS public URL")
	}
	if !c.SecureCookies {
		return errors.New("production requires secure session cookies")
	}
	if c.LocalMode || c.NoPaymentDemo {
		return errors.New("development features are unavailable in production")
	}
	if c.PaymentMode != "razorpay" {
		return errors.New("production requires the Razorpay payment provider")
	}
	if !c.Staging && c.RazorpayKeyID != "" && !strings.HasPrefix(c.RazorpayKeyID, "rzp_live_") {
		return errors.New("production requires live Razorpay keys; sandbox keys require isolated staging")
	}
	database, err := url.Parse(c.DatabaseURL)
	if err != nil || database.User == nil {
		return errors.New("production requires configured database credentials")
	}
	password, _ := database.User.Password()
	if password == "" || password == "wecratfs" {
		return errors.New("production cannot use development database credentials")
	}
	if database.Query().Get("sslmode") != "verify-full" {
		return errors.New("production database connections require sslmode=verify-full")
	}
	if c.S3AccessKey == "" || c.S3SecretKey == "" || c.S3AccessKey == "wecratfs" || c.S3SecretKey == "wecratfs-local-only" ||
		c.S3AccessKey == "GK0123456789abcdef01234567" || c.S3SecretKey == "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		return errors.New("production requires dedicated object storage credentials")
	}
	if !c.S3Secure {
		return errors.New("production object storage requires TLS")
	}
	return nil
}
