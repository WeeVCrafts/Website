package shiprocket

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/wecratfs/commerce/internal/ports"
)

// PickupLocations checks the authenticated account without returning addresses or contacts.
func (g *Gateway) PickupLocations(ctx context.Context) (map[string]struct{}, error) {
	addresses, err := g.PickupPostcodes(ctx)
	if err != nil {
		return nil, err
	}
	locations := make(map[string]struct{}, len(addresses))
	for location := range addresses {
		locations[location] = struct{}{}
	}
	return locations, nil
}

func (g *Gateway) PickupPostcodes(ctx context.Context) (map[string]string, error) {
	var response struct {
		Data struct {
			Addresses []struct {
				Location string      `json:"pickup_location"`
				Postcode json.Number `json:"pin_code"`
			} `json:"shipping_address"`
		} `json:"data"`
	}
	if err := g.call(ctx, http.MethodGet, "/settings/company/pickup", nil, &response); err != nil {
		return nil, err
	}
	if response.Data.Addresses == nil {
		return nil, ports.ErrShippingUnavailable
	}
	locations := make(map[string]string, len(response.Data.Addresses))
	for _, address := range response.Data.Addresses {
		if address.Location != "" {
			locations[address.Location] = address.Postcode.String()
		}
	}
	return locations, nil
}
