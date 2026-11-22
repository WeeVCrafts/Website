package shiprocket

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/wecratfs/commerce/internal/ports"
)

// WalletFunded uses the account read endpoint. A negative or zero balance
// prevents new checkout. A positive balance is not a reservation of funds.
func (g *Gateway) WalletFunded(ctx context.Context) (bool, error) {
	var response struct {
		Data struct {
			Balance json.Number `json:"balance_amount"`
		} `json:"data"`
	}
	if err := g.call(ctx, http.MethodGet, "/account/details/wallet-balance", nil, &response); err != nil {
		return false, err
	}
	value, err := strconv.ParseFloat(response.Data.Balance.String(), 64)
	if err != nil {
		return false, ports.ErrShippingUnavailable
	}
	return value >= 0.01, nil
}
