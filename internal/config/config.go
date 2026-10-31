package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	OperationalMetricsToken string
	Staging                 bool
	StagingUsername         string
	StagingPassword         string
	Address                 string
	Environment             string
	PublicURL               string
	DatabaseURL             string
	DatabaseCACert          string
	RedisAddress            string
	RedisURL                string
	MeiliAddress            string
	MeiliAPIKey             string
	S3Address               string
	SecureCookies           bool
	RazorpayKeyID           string
	RazorpayKeySecret       string
	RazorpayWebhookSecret   string
	RazorpayBaseURL         string
	PaymentMode             string
	NoPaymentDemo           bool
	LocalPaymentSecret      string
	LocalMode               bool
	SMTPAddress             string
	SMTPUsername            string
	SMTPPassword            string
	SMTPFrom                string
	ResendAPIKey            string
	ResendAPIKeyFile        string
	ResendFrom              string
	ResendReplyTo           string
	S3AccessKey             string
	S3SecretKey             string
	S3Bucket                string
	S3Secure                bool
}

func FromEnv() Config {
	environment := env("WECRATFS_ENV", "development")
	port := env("PORT", "8080")
	publicURL := env("WECRATFS_PUBLIC_URL", "")
	if publicURL == "" {
		if domain := env("RAILWAY_PUBLIC_DOMAIN", ""); domain != "" {
			publicURL = "https://" + domain
		} else {
			publicURL = "http://localhost:8080"
		}
	}
	return Config{
		OperationalMetricsToken: env("WECRATFS_METRICS_TOKEN", ""),
		Staging:                 boolEnv("WECRATFS_STAGING", false),
		StagingUsername:         env("WECRATFS_STAGING_USERNAME", ""),
		StagingPassword:         env("WECRATFS_STAGING_PASSWORD", ""),
		Address:                 env("WECRATFS_ADDR", ":"+port),
		Environment:             environment,
		PublicURL:               publicURL,
		DatabaseURL:             env("DATABASE_URL", "postgres://wecratfs:wecratfs@localhost:5432/wecratfs?sslmode=disable"),
		DatabaseCACert:          env("DATABASE_CA_CERT", ""),
		RedisAddress:            env("REDIS_ADDR", "localhost:6379"),
		RedisURL:                env("REDIS_URL", ""),
		MeiliAddress:            env("MEILI_ADDR", "localhost:7700"),
		MeiliAPIKey:             env("MEILI_API_KEY", ""),
		S3Address:               env("S3_ADDR", "localhost:3900"),
		SecureCookies:           boolEnv("WECRATFS_SECURE_COOKIES", environment == "production"),
		RazorpayKeyID:           env("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret:       env("RAZORPAY_KEY_SECRET", ""),
		RazorpayWebhookSecret:   env("RAZORPAY_WEBHOOK_SECRET", ""),
		RazorpayBaseURL:         env("RAZORPAY_BASE_URL", "https://api.razorpay.com"),
		PaymentMode:             env("WECRATFS_PAYMENT_MODE", "razorpay"),
		NoPaymentDemo:           boolEnv("WECRATFS_NO_PAYMENT_DEMO", false) && !strings.EqualFold(environment, "production"),
		LocalPaymentSecret:      env("WECRATFS_LOCAL_PAYMENT_SECRET", "wecratfs-local-payment"),
		LocalMode:               boolEnv("WECRATFS_LOCAL_MODE", false) && !strings.EqualFold(environment, "production"),
		SMTPAddress:             env("WECRATFS_SMTP_ADDR", ""),
		SMTPUsername:            env("WECRATFS_SMTP_USERNAME", ""),
		SMTPPassword:            env("WECRATFS_SMTP_PASSWORD", ""),
		SMTPFrom:                env("WECRATFS_SMTP_FROM", ""),
		ResendAPIKey:            env("WECRATFS_RESEND_API_KEY", ""),
		ResendAPIKeyFile:        env("WECRATFS_RESEND_API_KEY_FILE", ""),
		ResendFrom:              env("WECRATFS_RESEND_FROM", ""),
		ResendReplyTo:           env("WECRATFS_RESEND_REPLY_TO", ""),
		S3AccessKey:             env("S3_ACCESS_KEY", "GK0123456789abcdef01234567"),
		S3SecretKey:             env("S3_SECRET_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"),
		S3Bucket:                env("S3_BUCKET", "wecratfs-media"),
		S3Secure:                boolEnv("S3_SECURE", false),
	}
}

func boolEnv(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
