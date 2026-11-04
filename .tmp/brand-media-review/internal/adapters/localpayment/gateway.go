// Package localpayment provides a deterministic payment provider for local
// demos. It uses the same provider-neutral contracts as Razorpay and never
// stores card data or calls an external service.
package localpayment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wecratfs/commerce/internal/ports"
)

type Gateway struct {
	secret string
}

func NewGateway(secret string) *Gateway {
	if strings.TrimSpace(secret) == "" {
		secret = "wecratfs-local-payment"
	}
	return &Gateway{secret: secret}
}

func (g *Gateway) CreateOrder(_ context.Context, request ports.PaymentOrderRequest) (ports.PaymentOrder, error) {
	if g == nil || request.Reference == "" || request.AmountCents <= 0 || request.Currency == "" {
		return ports.PaymentOrder{}, errors.New("local payment order is incomplete")
	}
	return ports.PaymentOrder{
		ProviderOrderID: "local_order_" + request.Reference,
		AmountCents:     request.AmountCents,
		Currency:        request.Currency,
	}, nil
}

func (g *Gateway) VerifyPayment(_ context.Context, verification ports.PaymentVerification) error {
	if g == nil || verification.ProviderOrderID == "" || verification.ProviderPaymentID == "" {
		return errors.New("local payment verification is incomplete")
	}
	want := g.sign(verification.ProviderOrderID + "|" + verification.ProviderPaymentID)
	if !hmac.Equal([]byte(want), []byte(verification.Signature)) {
		return errors.New("local payment signature is invalid")
	}
	return nil
}

func (g *Gateway) CreateRefund(_ context.Context, request ports.PaymentRefundRequest) (ports.PaymentRefund, error) {
	if g == nil || request.ProviderPaymentID == "" || request.AmountCents <= 0 || request.Currency == "" {
		return ports.PaymentRefund{}, errors.New("local refund request is incomplete")
	}
	return ports.PaymentRefund{
		ProviderRefundID:  "local_refund_" + request.ProviderPaymentID,
		ProviderPaymentID: request.ProviderPaymentID,
		AmountCents:       request.AmountCents,
		Currency:          request.Currency,
		Status:            "processed",
	}, nil
}

// VerifyWebhook accepts the same small Razorpay-shaped payload used by the
// real adapter so the production webhook processor is exercised locally.
func (g *Gateway) VerifyWebhook(_ context.Context, rawBody []byte, signature string) (ports.PaymentWebhook, error) {
	if g == nil || len(rawBody) == 0 || !hmac.Equal([]byte(g.sign(string(rawBody))), []byte(signature)) {
		return ports.PaymentWebhook{}, errors.New("local webhook signature is invalid")
	}
	var payload struct {
		ID      string `json:"id"`
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					ID       string `json:"id"`
					OrderID  string `json:"order_id"`
					Amount   int64  `json:"amount"`
					Currency string `json:"currency"`
					Status   string `json:"status"`
				} `json:"entity"`
			} `json:"payment"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return ports.PaymentWebhook{}, err
	}
	entity := payload.Payload.Payment.Entity
	if payload.ID == "" || payload.Event == "" || entity.OrderID == "" || entity.ID == "" || entity.Amount <= 0 {
		return ports.PaymentWebhook{}, errors.New("local webhook payload is incomplete")
	}
	return ports.PaymentWebhook{
		EventID:           payload.ID,
		EventType:         payload.Event,
		ProviderOrderID:   entity.OrderID,
		ProviderPaymentID: entity.ID,
		Status:            entity.Status,
		AmountCents:       entity.Amount,
		Currency:          entity.Currency,
	}, nil
}

func (g *Gateway) Sign(value string) string {
	if g == nil {
		return ""
	}
	return g.sign(value)
}

func (g *Gateway) sign(value string) string {
	digest := hmac.New(sha256.New, []byte(g.secret))
	_, _ = digest.Write([]byte(value))
	return hex.EncodeToString(digest.Sum(nil))
}

func WebhookPayload(eventID, eventType, providerOrderID, providerPaymentID, status string, amountCents int64, currency string) ([]byte, error) {
	if eventID == "" || eventType == "" || providerOrderID == "" || providerPaymentID == "" || amountCents <= 0 || currency == "" {
		return nil, errors.New("local webhook payload is incomplete")
	}
	return json.Marshal(map[string]any{
		"id":    eventID,
		"event": eventType,
		"payload": map[string]any{
			"payment": map[string]any{
				"entity": map[string]any{
					"id":       providerPaymentID,
					"order_id": providerOrderID,
					"status":   status,
					"amount":   amountCents,
					"currency": currency,
				},
			},
		},
	})
}

func PaymentID(orderNumber string) string {
	return fmt.Sprintf("local_payment_%s", strings.TrimSpace(orderNumber))
}

var _ ports.PaymentGateway = (*Gateway)(nil)
var _ ports.PaymentRefundGateway = (*Gateway)(nil)
var _ ports.PaymentWebhookVerifier = (*Gateway)(nil)
