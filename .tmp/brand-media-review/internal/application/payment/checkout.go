package payment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/wecratfs/commerce/internal/ports"
)

var ErrCheckoutPaymentInvalid = errors.New("checkout payment could not be verified")

// CheckoutVerificationService verifies the Razorpay Checkout signature and
// records the resulting authorization through the same idempotent repository
// path used by provider webhooks. A later captured webhook remains the source
// of truth for marking an order paid.
type CheckoutVerificationService struct {
	gateway  ports.PaymentGateway
	orders   ports.PaymentOrderRepository
	webhooks ports.PaymentWebhookRepository
}

func NewCheckoutVerificationService(gateway ports.PaymentGateway, orders ports.PaymentOrderRepository, webhooks ports.PaymentWebhookRepository) *CheckoutVerificationService {
	return &CheckoutVerificationService{gateway: gateway, orders: orders, webhooks: webhooks}
}

func (s *CheckoutVerificationService) Verify(ctx context.Context, userID int64, orderNumber string, verification ports.PaymentVerification) error {
	orderNumber = strings.TrimSpace(orderNumber)
	if userID <= 0 || orderNumber == "" || s.gateway == nil || s.orders == nil || s.webhooks == nil ||
		strings.TrimSpace(verification.ProviderOrderID) == "" || strings.TrimSpace(verification.ProviderPaymentID) == "" || strings.TrimSpace(verification.Signature) == "" {
		return ErrCheckoutPaymentInvalid
	}
	record, err := s.orders.GetPaymentOrder(ctx, userID, orderNumber)
	if err != nil {
		return ErrCheckoutPaymentInvalid
	}
	if record.Provider != "razorpay" || record.OrderStatus != "pending_payment" || record.ProviderOrderID == "" || record.ProviderOrderID != verification.ProviderOrderID || record.AmountCents <= 0 || record.Currency == "" {
		return ErrCheckoutPaymentInvalid
	}
	if err := s.gateway.VerifyPayment(ctx, verification); err != nil {
		return ErrCheckoutPaymentInvalid
	}
	eventHash := sha256.Sum256([]byte(verification.ProviderPaymentID))
	webhook := ports.PaymentWebhook{
		EventID:   "checkout_verified_" + hex.EncodeToString(eventHash[:]),
		EventType: "payment.authorized", ProviderOrderID: record.ProviderOrderID,
		ProviderPaymentID: verification.ProviderPaymentID, Status: "authorized",
		AmountCents: record.AmountCents, Currency: record.Currency,
	}
	payload, err := json.Marshal(webhook)
	if err != nil {
		return ErrCheckoutPaymentInvalid
	}
	if _, err := s.webhooks.ProcessWebhook(ctx, webhook, payload); err != nil {
		return err
	}
	return nil
}
