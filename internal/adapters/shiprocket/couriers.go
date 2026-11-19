package shiprocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/wecratfs/commerce/internal/ports"
)

func (g *Gateway) CourierOptions(ctx context.Context, shipment ports.Shipment) ([]ports.CourierOption, error) {
	if shipment.ProviderOrderID <= 0 || !indianPostcode(shipment.DeliveryPostalCode) {
		return nil, ports.ErrShippingInput
	}
	locations, err := g.PickupPostcodes(ctx)
	if err != nil {
		return nil, err
	}
	pickup := locations[shipment.PickupLocation]
	if !indianPostcode(pickup) {
		return nil, ports.ErrShippingInput
	}
	query := url.Values{"order_id": {strconv.FormatInt(shipment.ProviderOrderID, 10)}, "pickup_postcode": {pickup}, "delivery_postcode": {shipment.DeliveryPostalCode}}
	return g.availableCouriers(ctx, query)
}

func (g *Gateway) DeliveryAvailable(ctx context.Context, pickup, destination string, parcel ports.Parcel) (bool, error) {
	available, _, err := g.DeliveryEstimate(ctx, pickup, destination, parcel)
	return available, err
}

func (g *Gateway) DeliveryEstimate(ctx context.Context, pickup, destination string, parcel ports.Parcel) (bool, int, error) {
	options, err := g.deliveryOptions(ctx, pickup, destination, parcel)
	days := 0
	for _, option := range options {
		estimate, _ := strconv.Atoi(option.EstimatedDays)
		if estimate > 0 && (days == 0 || option.Recommended) {
			days = estimate
		}
	}
	return len(options) > 0, days, err
}

func (g *Gateway) DeliveryRate(ctx context.Context, pickup, destination string, parcel ports.Parcel) (ports.CourierOption, error) {
	options, err := g.deliveryOptions(ctx, pickup, destination, parcel)
	if err != nil {
		return ports.CourierOption{}, err
	}
	if len(options) == 0 {
		return ports.CourierOption{}, ports.ErrDeliveryUnavailable
	}
	var selected ports.CourierOption
	for _, option := range options {
		if option.RateCents <= 0 {
			continue
		}
		if selected.ID == 0 || (option.Recommended && !selected.Recommended) ||
			(option.Recommended == selected.Recommended &&
				(option.RateCents < selected.RateCents || option.RateCents == selected.RateCents && option.ID < selected.ID)) {
			selected = option
		}
	}
	if selected.ID == 0 {
		return selected, ports.ErrShippingUnavailable
	}
	return selected, nil
}

func (g *Gateway) deliveryOptions(ctx context.Context, pickup, destination string, parcel ports.Parcel) ([]ports.CourierOption, error) {
	if !indianPostcode(pickup) || !indianPostcode(destination) || !parcel.Valid() {
		return nil, ports.ErrShippingInput
	}
	query := url.Values{"pickup_postcode": {pickup}, "delivery_postcode": {destination}, "cod": {"0"}, "weight": {strconv.FormatFloat(float64(parcel.WeightGrams)/1000, 'f', 3, 64)}, "length": {strconv.Itoa(parcel.LengthCM)}, "breadth": {strconv.Itoa(parcel.WidthCM)}, "height": {strconv.Itoa(parcel.HeightCM)}}
	options, err := g.availableCouriers(ctx, query)
	if g.courierResult != nil {
		g.courierResult(len(options) > 0, err)
	}
	return options, err
}

func (g *Gateway) availableCouriers(ctx context.Context, query url.Values) ([]ports.CourierOption, error) {
	var reply struct {
		Currency string `json:"currency"`
		Data     struct {
			RecommendedID int64 `json:"recommended_courier_company_id"`
			Couriers      []struct {
				ID      int64           `json:"courier_company_id"`
				Name    string          `json:"courier_name"`
				Blocked *int            `json:"blocked"`
				Rate    json.RawMessage `json:"rate"`
				Days    json.RawMessage `json:"estimated_delivery_days"`
			} `json:"available_courier_companies"`
		} `json:"data"`
	}
	if err := g.call(ctx, http.MethodGet, "/courier/serviceability/?"+query.Encode(), nil, &reply); err != nil {
		return nil, err
	}
	if reply.Data.Couriers == nil {
		return nil, ports.ErrShippingUnavailable
	}
	options := make([]ports.CourierOption, 0, len(reply.Data.Couriers))
	seen := make(map[int64]bool)
	for _, courier := range reply.Data.Couriers {
		name := strings.TrimSpace(courier.Name)
		if courier.ID < 1 || courier.ID > 100000000 || seen[courier.ID] || name == "" || len(name) > 120 || (courier.Blocked != nil && *courier.Blocked != 0) {
			continue
		}
		seen[courier.ID] = true
		option := ports.CourierOption{ID: courier.ID, Name: name, Recommended: courier.ID == reply.Data.RecommendedID}
		if rate, err := strconv.ParseFloat(estimateText(courier.Rate), 64); err == nil && rate > 0 && rate <= 1000000 && (reply.Currency == "" || reply.Currency == "INR") {
			option.EstimatedRate = fmt.Sprintf("₹%.2f", rate)
			option.RateCents = int64(rate*100 + 0.5)
		}
		if days, err := strconv.ParseInt(estimateText(courier.Days), 10, 64); err == nil && days > 0 && days <= 365 {
			option.EstimatedDays = strconv.FormatInt(days, 10)
		}
		options = append(options, option)
	}
	return options, nil
}

func estimateText(value json.RawMessage) string {
	var label string
	if json.Unmarshal(value, &label) == nil {
		return strings.TrimSpace(label)
	}
	return string(value)
}

func indianPostcode(value string) bool {
	return len(value) == 6 && value[0] != '0' && strings.Trim(value, "0123456789") == ""
}
