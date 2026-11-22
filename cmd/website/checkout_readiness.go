package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/config"
)

const (
	checkoutCheckInterval = time.Minute
	checkoutCheckJob      = "checkout-readiness"
)

func checkoutCheckKey(t time.Time) string {
	return fmt.Sprintf("scheduled:%s:%d", checkoutCheckJob, t.UnixNano()/int64(checkoutCheckInterval))
}

type checkoutPaymentsProbe interface {
	CheckOrdersAccess(context.Context) error
}

type checkoutShippingProbe interface {
	PickupPostcodes(context.Context) (map[string]string, error)
	WalletFunded(context.Context) (bool, error)
}

func checkoutSettingsKey(cfg config.Config) string {
	settings, _ := json.Marshal([]any{cfg.CheckoutEnabled, cfg.PublicContentApproved, cfg.PublicURL,
		cfg.RazorpayKeyID, cfg.RazorpayKeySecret, cfg.RazorpayWebhookSecret, cfg.RazorpayBaseURL, cfg.Shipping})
	return fmt.Sprintf("%x", sha256.Sum256(settings))
}

func refreshCheckoutHealth(ctx context.Context, cfg config.Config, settingsKey string, repo *postgres.MarketplaceRepository, payment, shipping any) error {
	startedAt := time.Now()
	blockers, pickups := collectCheckoutHealth(ctx, cfg, payment, shipping)
	writeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := repo.SaveCheckoutHealth(writeCtx, settingsKey, startedAt, blockers, pickups); err != nil {
		return err
	}
	return repo.PublishCheckoutGuidance(writeCtx)
}

func collectCheckoutHealth(ctx context.Context, cfg config.Config, payment, shipping any) ([]string, map[string]string) {
	var blockers []string
	if !cfg.CheckoutEnabled {
		blockers = append(blockers, "checkout_switch")
	}
	if !cfg.PublicContentApproved {
		blockers = append(blockers, "public_content")
	}
	paymentProbe, paymentOK := payment.(checkoutPaymentsProbe)
	if !paymentOK || !strings.HasPrefix(cfg.RazorpayKeyID, "rzp_live_") || cfg.RazorpayKeySecret == "" {
		blockers = append(blockers, "razorpay_config")
	} else {
		probeCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		err := paymentProbe.CheckOrdersAccess(probeCtx)
		cancel()
		if err != nil {
			blockers = append(blockers, "razorpay_access")
		}
	}
	if cfg.RazorpayWebhookSecret == "" {
		blockers = append(blockers, "razorpay_webhook")
	}
	if !cfg.Shipping.BookingsEnabled {
		blockers = append(blockers, "shiprocket_bookings")
	}
	shippingProbe, shippingOK := shipping.(checkoutShippingProbe)
	if !shippingOK || cfg.Shipping.Mode != "live" || cfg.ValidateShipping() != nil {
		return append(blockers, "shiprocket_config"), nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	pickups, err := shippingProbe.PickupPostcodes(probeCtx)
	cancel()
	if err != nil {
		blockers = append(blockers, "shiprocket_pickups")
	}
	probeCtx, cancel = context.WithTimeout(ctx, 6*time.Second)
	funded, err := shippingProbe.WalletFunded(probeCtx)
	cancel()
	if err != nil {
		blockers = append(blockers, "shiprocket_wallet_access")
	} else if !funded {
		blockers = append(blockers, "shiprocket_wallet")
	}
	return blockers, pickups
}
