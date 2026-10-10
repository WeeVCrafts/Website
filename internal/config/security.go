package config

import (
	"errors"
	"net/url"
	"strings"
)

// ValidateSecurity rejects development defaults before connecting or serving.
func (c Config) ValidateSecurity() error {
	if !strings.EqualFold(c.Environment, "production") {
		return nil
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
	if c.S3AccessKey == "" || c.S3SecretKey == "" || c.S3AccessKey == "wecratfs" || c.S3SecretKey == "wecratfs-local-only" {
		return errors.New("production requires dedicated object storage credentials")
	}
	if !c.S3Secure {
		return errors.New("production object storage requires TLS")
	}
	return nil
}
