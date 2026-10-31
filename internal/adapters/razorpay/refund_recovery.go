package razorpay

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/wecratfs/commerce/internal/ports"
)

type recoveryRefund struct {
	ID        string `json:"id"`
	PaymentID string `json:"payment_id"`
	Receipt   string `json:"receipt"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Status    string `json:"status"`
}

func (r recoveryRefund) refund() ports.PaymentRefund {
	return ports.PaymentRefund{ProviderRefundID: r.ID, ProviderPaymentID: r.PaymentID, AmountCents: r.Amount, Currency: r.Currency, Status: r.Status}
}

// LookupRefund never creates a refund. An absent reference is resolved by the
// stable receipt sent on the original POST, not by retrying that POST.
func (g *Gateway) LookupRefund(ctx context.Context, record ports.PaymentRefundRecord) (ports.PaymentRefund, error) {
	if record.ProviderPaymentID == "" {
		return ports.PaymentRefund{}, ports.ErrRefundNotFound
	}
	if record.ProviderRefundID != "" {
		var result recoveryRefund
		if err := g.doJSON(ctx, http.MethodGet, "/v1/refunds/"+url.PathEscape(record.ProviderRefundID), nil, &result); err != nil {
			return ports.PaymentRefund{}, err
		}
		if result.ID != record.ProviderRefundID {
			return ports.PaymentRefund{}, ports.ErrRefundState
		}
		return result.refund(), nil
	}
	receipt := fmt.Sprintf("refund-%d", record.ID)
	var match *recoveryRefund
	for page := 0; page < 10; page++ {
		var result struct {
			Items []recoveryRefund `json:"items"`
		}
		path := fmt.Sprintf("/v1/payments/%s/refunds?count=100&skip=%d", url.PathEscape(record.ProviderPaymentID), page*100)
		if err := g.doJSON(ctx, http.MethodGet, path, nil, &result); err != nil {
			return ports.PaymentRefund{}, err
		}
		for _, item := range result.Items {
			if item.Receipt == receipt {
				if match != nil {
					return ports.PaymentRefund{}, errors.New("multiple provider refunds share one receipt")
				}
				copy := item
				match = &copy
			}
		}
		if len(result.Items) < 100 {
			if match == nil {
				return ports.PaymentRefund{}, ports.ErrRefundNotFound
			}
			return match.refund(), nil
		}
	}
	return ports.PaymentRefund{}, errors.New("provider refund lookup exceeded bounded pagination")
}
