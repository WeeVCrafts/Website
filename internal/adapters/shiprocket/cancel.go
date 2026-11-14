package shiprocket

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/wecratfs/commerce/internal/ports"
)

func (g *Gateway) Cancel(ctx context.Context, s ports.Shipment) error {
	var response json.RawMessage
	if s.AWB != "" {
		return g.call(ctx, http.MethodPost, "/orders/cancel/shipment/awbs", map[string][]string{"awbs": {s.AWB}}, &response)
	}
	return g.call(ctx, http.MethodPost, "/orders/cancel", map[string][]int64{"ids": {s.ProviderOrderID}}, &response)
}
