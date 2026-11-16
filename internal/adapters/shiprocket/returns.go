package shiprocket

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/wecratfs/commerce/internal/ports"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (g *Gateway) CreateReturn(ctx context.Context, order ports.ReturnShippingOrder) (ports.Shipment, error) {
	s := order.Shipment
	if !s.Parcel.Valid() || s.Reference == "" || order.Address.CountryCode != "IN" || len(order.Items) == 0 {
		return s, ports.ErrShippingInput
	}
	// A return is collected from the original buyer and delivered to the seller's
	// verified warehouse. Never reuse a different seller's default address.
	var pickups struct {
		Data struct {
			Addresses []struct {
				Location string      `json:"pickup_location"`
				Name     string      `json:"name"`
				Address  string      `json:"address"`
				Address2 string      `json:"address_2"`
				City     string      `json:"city"`
				State    string      `json:"state"`
				Postcode json.Number `json:"pin_code"`
				Phone    string      `json:"phone"`
				Email    string      `json:"email"`
				Verified int         `json:"phone_verified"`
			} `json:"shipping_address"`
		} `json:"data"`
	}
	if err := g.call(ctx, http.MethodGet, "/settings/company/pickup", nil, &pickups); err != nil {
		return s, err
	}
	var destination map[string]any
	for _, p := range pickups.Data.Addresses {
		if p.Location == s.PickupLocation && p.Verified == 1 && p.Name != "" && p.Phone != "" {
			destination = map[string]any{"shipping_customer_name": p.Name, "shipping_address": p.Address, "shipping_address_2": p.Address2, "shipping_city": p.City, "shipping_state": p.State, "shipping_country": "India", "shipping_pincode": p.Postcode.String(), "shipping_phone": p.Phone, "shipping_email": p.Email}
		}
	}
	if destination == nil {
		return s, ports.ErrShippingInput
	}
	var subtotal int64
	var items []orderItem
	for _, item := range order.Items {
		items = append(items, orderItem{item.ProductName, item.SKU, item.Quantity, rupees(item.UnitPriceCents)})
		subtotal += item.LineTotalCents
	}
	query := url.Values{"pickup_postcode": {order.Address.PostalCode}, "delivery_postcode": {fmt.Sprint(destination["shipping_pincode"])}, "is_return": {"1"}, "declared_value": {string(rupees(subtotal - order.DiscountCents))}, "cod": {"0"}, "weight": {strconv.FormatFloat(float64(s.WeightGrams)/1000, 'f', 3, 64)}, "length": {strconv.Itoa(s.LengthCM)}, "breadth": {strconv.Itoa(s.WidthCM)}, "height": {strconv.Itoa(s.HeightCM)}}
	options, err := g.availableCouriers(ctx, query)
	if err != nil {
		return s, err
	}
	if len(options) == 0 {
		return s, ports.ErrShippingUnavailable
	}
	input := destination
	for k, v := range map[string]any{"order_id": s.Reference, "order_date": order.PlacedAt.Format("2006-01-02 15:04"), "pickup_customer_name": order.Address.RecipientName, "pickup_address": order.Address.Line1, "pickup_address_2": order.Address.Line2, "pickup_city": order.Address.City, "pickup_state": indianState(order.Address.State), "pickup_country": "India", "pickup_pincode": order.Address.PostalCode, "pickup_email": order.Email, "pickup_phone": order.Address.Phone, "payment_method": "Prepaid", "order_items": items, "sub_total": rupees(subtotal), "total_discount": rupees(order.DiscountCents), "length": s.LengthCM, "breadth": s.WidthCM, "height": s.HeightCM, "weight": float64(s.WeightGrams) / 1000} {
		input[k] = v
	}
	type returnResponse struct {
		OrderID    int64  `json:"order_id"`
		ShipmentID int64  `json:"shipment_id"`
		AWB        string `json:"awb_code"`
		Courier    string `json:"courier_name"`
	}
	var reply struct {
		returnResponse
		Data *returnResponse `json:"data"`
	}
	if err = g.call(ctx, http.MethodPost, "/shipments/create/return-shipment", input, &reply); err != nil {
		return s, err
	}
	if reply.OrderID == 0 && reply.Data != nil {
		reply.returnResponse = *reply.Data
	}
	if reply.OrderID <= 0 || reply.ShipmentID <= 0 {
		return s, ports.ErrShippingPending
	}
	s.ProviderOrderID, s.ProviderShipmentID, s.AWB, s.Courier = reply.OrderID, reply.ShipmentID, reply.AWB, reply.Courier
	// An authenticated read confirms the returned identifiers and actual state.
	fresh, err := g.Status(ctx, s)
	if err != nil {
		// The POST may already have booked the pickup. Even a rejected follow-up
		// read is ambiguous; retain intent and recover by reference next time.
		return s, ports.ErrShippingPending
	}
	return fresh, nil
}

func (g *Gateway) FindReturn(ctx context.Context, reference string) (ports.Shipment, error) {
	if reference == "" {
		return ports.Shipment{}, ports.ErrShippingInput
	}
	var found ports.Shipment
	for page := 1; page <= 5; page++ {
		var reply struct {
			Data []orderDetails `json:"data"`
		}
		path := "/orders/processing/return?" + url.Values{"search": {reference}, "per_page": {"100"}, "page": {strconv.Itoa(page)}}.Encode()
		if err := g.call(ctx, http.MethodGet, path, nil, &reply); err != nil {
			return found, err
		}
		for _, o := range reply.Data {
			if strings.TrimSpace(o.Reference) == reference {
				if found.ProviderOrderID != 0 {
					return ports.Shipment{}, ports.ErrShippingPending
				}
				details, e := o.shipment()
				if e != nil {
					return found, e
				}
				found = ports.Shipment{Reference: reference, ProviderOrderID: o.ID, ProviderShipmentID: details.ID, AWB: details.AWB, Courier: details.Courier, ProviderStatus: details.deliveryStatus(o.Status)}
			}
		}
		if len(reply.Data) < 100 {
			break
		}
	}
	if found.ProviderOrderID <= 0 || found.ProviderShipmentID <= 0 {
		return found, ports.ErrShippingPending
	}
	return g.Status(ctx, found)
}
