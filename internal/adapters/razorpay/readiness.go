package razorpay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// CheckOrdersAccess verifies the configured credentials without creating an order.
func (g *Gateway) CheckOrdersAccess(ctx context.Context) error {
	var response struct {
		Entity string            `json:"entity"`
		Items  []json.RawMessage `json:"items"`
	}
	if err := g.doJSON(ctx, http.MethodGet, "/v1/orders?count=1", nil, &response); err != nil {
		return err
	}
	if response.Entity != "collection" || response.Items == nil || len(response.Items) > 1 {
		return errors.New("razorpay returned an invalid order collection")
	}
	return nil
}
