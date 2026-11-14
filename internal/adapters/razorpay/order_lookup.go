package razorpay

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/wecratfs/commerce/internal/ports"
)

func (g *Gateway) LookupOrdersByReceipt(ctx context.Context, receipt string) ([]ports.PaymentOrder, error) {
	if receipt == "" || g.keyID == "" || g.keySecret == "" {
		return nil, errors.New("razorpay order lookup is not configured")
	}
	query := url.Values{"receipt": {receipt}, "count": {"2"}}
	var response struct {
		Entity string `json:"entity"`
		Items  []struct {
			ID       string `json:"id"`
			Receipt  string `json:"receipt"`
			Amount   int64  `json:"amount"`
			Currency string `json:"currency"`
		} `json:"items"`
	}
	if err := g.doJSON(ctx, http.MethodGet, "/v1/orders?"+query.Encode(), nil, &response); err != nil {
		return nil, err
	}
	if len(response.Items) > 2 {
		return nil, ports.ErrPaymentMismatch
	}
	if response.Entity != "collection" || response.Items == nil {
		return nil, errors.New("razorpay returned an invalid order collection")
	}
	orders := make([]ports.PaymentOrder, 0, len(response.Items))
	for _, item := range response.Items {
		if item.Receipt != receipt || item.ID == "" {
			return nil, ports.ErrPaymentMismatch
		}
		orders = append(orders, ports.PaymentOrder{ProviderOrderID: item.ID, AmountCents: item.Amount, Currency: item.Currency})
	}
	return orders, nil
}
