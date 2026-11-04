package razorpay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"

	"github.com/wecratfs/commerce/internal/ports"
)

func (g *Gateway) LookupPayments(ctx context.Context, record ports.PaymentOrderRecord) ([]ports.PaymentWebhook, error) {
	if record.ProviderOrderID == "" {
		return nil, ports.ErrPaymentNotFound
	}
	var response struct {
		Items []struct {
			ID, Status, Currency string
			OrderID              string `json:"order_id"`
			Amount               int64
		}
	}
	if err := g.doJSON(ctx, http.MethodGet, "/v1/orders/"+url.PathEscape(record.ProviderOrderID)+"/payments", nil, &response); err != nil {
		return nil, err
	}
	if len(response.Items) > 500 {
		return nil, fmt.Errorf("provider payment lookup exceeded bounded results")
	}
	events := make([]ports.PaymentWebhook, 0, len(response.Items))
	seen := map[string]bool{}
	for _, item := range response.Items {
		if item.ID == "" || seen[item.ID] || item.OrderID != record.ProviderOrderID || item.Amount != record.AmountCents || item.Currency != record.Currency {
			return nil, ports.ErrPaymentMismatch
		}
		seen[item.ID] = true
		status := item.Status
		if status == "created" {
			status = "pending"
		}
		switch status {
		case "captured", "authorized", "failed", "pending":
		case "refunded":
			continue
		default:
			return nil, ports.ErrPaymentMismatch
		}
		hash := sha256.Sum256([]byte("provider-status-v1|" + record.ProviderOrderID + "|" + item.ID + "|" + status))
		events = append(events, ports.PaymentWebhook{EventID: "reconciled_" + hex.EncodeToString(hash[:]), EventType: "payment." + status, ProviderOrderID: item.OrderID, ProviderPaymentID: item.ID, AmountCents: item.Amount, Currency: item.Currency, Status: status})
	}
	return events, nil
}
