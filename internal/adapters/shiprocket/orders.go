package shiprocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/wecratfs/commerce/internal/ports"
)

type orderItem struct {
	Name  string      `json:"name"`
	SKU   string      `json:"sku"`
	Units int         `json:"units"`
	Price json.Number `json:"selling_price"`
}

type orderInput struct {
	Reference         string      `json:"order_id"`
	Date              string      `json:"order_date"`
	PickupLocation    string      `json:"pickup_location"`
	FirstName         string      `json:"billing_customer_name"`
	LastName          string      `json:"billing_last_name"`
	Line1             string      `json:"billing_address"`
	Line2             string      `json:"billing_address_2"`
	City              string      `json:"billing_city"`
	PostalCode        string      `json:"billing_pincode"`
	State             string      `json:"billing_state"`
	Country           string      `json:"billing_country"`
	Email             string      `json:"billing_email"`
	Phone             string      `json:"billing_phone"`
	ShippingIsBilling bool        `json:"shipping_is_billing"`
	Items             []orderItem `json:"order_items"`
	PaymentMethod     string      `json:"payment_method"`
	Subtotal          json.Number `json:"sub_total"`
	Discount          json.Number `json:"total_discount"`
	Length            int         `json:"length"`
	Width             int         `json:"breadth"`
	Height            int         `json:"height"`
	Weight            float64     `json:"weight"`
}

func rupees(cents int64) json.Number {
	return json.Number(strconv.FormatInt(cents/100, 10) + "." + strconv.FormatInt(100+cents%100, 10)[1:])
}

func (g *Gateway) Create(ctx context.Context, order ports.ShippingOrder) (ports.Shipment, error) {
	shipment := order.Shipment
	name := strings.Fields(order.Address.RecipientName)
	if len(name) == 0 || shipment.Reference == "" || order.Address.CountryCode != "IN" {
		return shipment, ports.ErrShippingInput
	}
	phone := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		if unicode.IsSpace(r) || strings.ContainsRune("+()-", r) {
			return -1
		}
		return r
	}, order.Address.Phone)
	if len(phone) == 12 && strings.HasPrefix(phone, "91") {
		phone = phone[2:]
	}
	if len(phone) != 10 || strings.ContainsFunc(phone, func(r rune) bool { return r < '0' || r > '9' }) {
		return shipment, ports.ErrShippingInput
	}
	input := orderInput{
		Reference: shipment.Reference, Date: order.PlacedAt.In(time.FixedZone("IST", 19800)).Format("2006-01-02 15:04"), PickupLocation: shipment.PickupLocation,
		FirstName: name[0], LastName: strings.Join(name[1:], " "), Line1: order.Address.Line1, Line2: order.Address.Line2, City: order.Address.City, PostalCode: order.Address.PostalCode,
		State: indianState(order.Address.State), Country: "India", Email: order.Email, Phone: phone, ShippingIsBilling: true, PaymentMethod: "Prepaid", Discount: rupees(order.DiscountCents),
		Length: shipment.LengthCM, Width: shipment.WidthCM, Height: shipment.HeightCM, Weight: float64(shipment.WeightGrams) / 1000,
	}
	var subtotal int64
	for _, item := range order.Items {
		input.Items = append(input.Items, orderItem{item.ProductName, item.SKU, item.Quantity, rupees(item.UnitPriceCents)})
		subtotal += item.LineTotalCents
	}
	input.Subtotal = rupees(subtotal)
	var response struct {
		OrderID    int64  `json:"order_id"`
		ShipmentID int64  `json:"shipment_id"`
		Reference  string `json:"channel_order_id"`
		Status     string `json:"status"`
	}
	if err := g.call(ctx, http.MethodPost, "/orders/create/adhoc", input, &response); err != nil {
		return shipment, err
	}
	if response.OrderID <= 0 || response.ShipmentID <= 0 || (response.Reference != "" && response.Reference != shipment.Reference) {
		return shipment, ports.ErrShippingPending
	}
	shipment.ProviderOrderID, shipment.ProviderShipmentID, shipment.ProviderStatus = response.OrderID, response.ShipmentID, response.Status
	return shipment, nil
}

type shipmentDetails struct {
	ID      int64           `json:"id"`
	AWB     string          `json:"awb"`
	Courier string          `json:"courier"`
	Pickup  string          `json:"pickup_token_number"`
	Status  json.RawMessage `json:"status"`
}

func (details shipmentDetails) deliveryStatus(orderStatus string) string {
	var code int
	if json.Unmarshal(details.Status, &code) == nil {
		switch code {
		case 1:
			return "AWB ASSIGNED"
		case 2:
			return "LABEL GENERATED"
		case 3:
			return "PICKUP SCHEDULED"
		case 4:
			return "PICKUP QUEUED"
		case 5:
			return "MANIFEST GENERATED"
		case 6:
			return "SHIPPED"
		case 7:
			return "DELIVERED"
		case 8, 45:
			return "CANCELLED"
		case 9:
			return "RTO INITIATED"
		case 10:
			return "RTO DELIVERED"
		case 11:
			return "PENDING"
		case 12:
			return "LOST"
		case 13, 20:
			return "PICKUP EXCEPTION"
		case 14, 40, 41, 46:
			return "RTO INITIATED"
		case 16:
			return "CANCELLATION REQUESTED"
		case 17:
			return "OUT FOR DELIVERY"
		case 18:
			return "IN TRANSIT"
		case 19:
			return "OUT FOR PICKUP"
		case 15:
			return "PICKUP RESCHEDULED"
		case 52:
			return "SHIPMENT BOOKED"
		case 21:
			return "UNDELIVERED"
		case 22:
			return "DELAYED"
		case 24, 25:
			return "DAMAGED"
		case 38:
			return "REACHED AT DESTINATION HUB"
		case 42, 51:
			return "PICKED UP"
		case 0:
			return strings.ToUpper(strings.TrimSpace(orderStatus))
		default:
			return "UNKNOWN"
		}
	}
	var label string
	_ = json.Unmarshal(details.Status, &label)
	if code, err := strconv.Atoi(label); err == nil {
		return (shipmentDetails{Status: json.RawMessage(strconv.Itoa(code))}).deliveryStatus(orderStatus)
	}
	switch label = strings.ToUpper(strings.TrimSpace(label)); label {
	case "NEW", "PROCESSING", "PENDING", "READY TO SHIP", "AWB ASSIGNED", "LABEL GENERATED", "PICKUP SCHEDULED", "PICKUP QUEUED", "MANIFEST GENERATED", "PICKUP RESCHEDULED", "SHIPMENT BOOKED", "OUT FOR PICKUP", "CANCELLATION REQUESTED", "LOST", "PICKUP EXCEPTION", "UNDELIVERED", "DELAYED", "DAMAGED", "SHIPPED", "DELIVERED", "RETURN DELIVERED", "CANCELLED", "CANCELED", "RTO INITIATED", "RTO DELIVERED", "OUT FOR DELIVERY", "IN TRANSIT", "IN-TRANSIT", "REACHED AT DESTINATION HUB", "PICKED UP":
		return label
	case "CANCELLED_BEFORE_DISPATCHED", "CANCELLED BEFORE DISPATCHED":
		return "CANCELLED"
	case "":
		return strings.ToUpper(strings.TrimSpace(orderStatus))
	default:
		return "UNKNOWN"
	}
}

type orderDetails struct {
	ID        int64           `json:"id"`
	Reference string          `json:"channel_order_id"`
	Status    string          `json:"status"`
	Shipments json.RawMessage `json:"shipments"`
}

func (order orderDetails) shipment() (shipmentDetails, error) {
	var details shipmentDetails
	if json.Unmarshal(order.Shipments, &details) == nil && details.ID > 0 {
		return details, nil
	}
	var list []shipmentDetails
	if json.Unmarshal(order.Shipments, &list) == nil && len(list) == 1 && list[0].ID > 0 {
		return list[0], nil
	}
	return details, ports.ErrShippingPending
}

func (g *Gateway) Status(ctx context.Context, shipment ports.Shipment) (ports.Shipment, error) {
	var response struct {
		Data orderDetails `json:"data"`
	}
	if err := g.call(ctx, http.MethodGet, "/orders/show/"+strconv.FormatInt(shipment.ProviderOrderID, 10), nil, &response); err != nil {
		return shipment, err
	}
	order := response.Data
	details, err := order.shipment()
	if err != nil || order.ID != shipment.ProviderOrderID || order.Reference != shipment.Reference || details.ID != shipment.ProviderShipmentID {
		return shipment, ports.ErrShippingPending
	}
	shipment.ProviderStatus, shipment.AWB, shipment.Courier, shipment.PickupReference = details.deliveryStatus(order.Status), details.AWB, details.Courier, details.Pickup
	return shipment, nil
}

func (g *Gateway) Find(ctx context.Context, reference string) (ports.Shipment, error) {
	var found ports.Shipment
	// The sandbox search index can return zero after creation. The recent-order list is the fallback.
	for page := 0; page <= 3; page++ {
		query := url.Values{"per_page": {"50"}, "sort": {"DESC"}, "sort_by": {"id"}}
		if page == 0 {
			query.Set("search", reference)
		} else {
			query.Set("page", strconv.Itoa(page))
		}
		var response struct {
			Data []orderDetails `json:"data"`
			Meta struct {
				Pagination struct {
					TotalPages int `json:"total_pages"`
				} `json:"pagination"`
			} `json:"meta"`
		}
		if err := g.call(ctx, http.MethodGet, "/orders?"+query.Encode(), nil, &response); err != nil {
			return found, err
		}
		for _, order := range response.Data {
			if order.Reference != reference {
				continue
			}
			details, err := order.shipment()
			if err != nil || order.ID <= 0 || found.ProviderOrderID != 0 {
				return ports.Shipment{}, ports.ErrShippingPending
			}
			found = ports.Shipment{Reference: reference, ProviderOrderID: order.ID, ProviderShipmentID: details.ID, ProviderStatus: details.deliveryStatus(order.Status), AWB: details.AWB, Courier: details.Courier, PickupReference: details.Pickup}
		}
		if page == 0 && found.ProviderOrderID > 0 || page > 0 && page >= response.Meta.Pagination.TotalPages {
			break
		}
	}
	if found.ProviderOrderID == 0 {
		return found, ports.ErrShippingPending
	}
	return found, nil
}

func indianState(value string) string {
	states := map[string]string{"AN": "Andaman and Nicobar Islands", "AP": "Andhra Pradesh", "AR": "Arunachal Pradesh", "AS": "Assam", "BR": "Bihar", "CH": "Chandigarh", "CG": "Chhattisgarh", "CT": "Chhattisgarh", "DL": "Delhi", "DN": "Dadra and Nagar Haveli and Daman and Diu", "GA": "Goa", "GJ": "Gujarat", "HR": "Haryana", "HP": "Himachal Pradesh", "JK": "Jammu and Kashmir", "JH": "Jharkhand", "KA": "Karnataka", "KL": "Kerala", "LA": "Ladakh", "LD": "Lakshadweep", "MP": "Madhya Pradesh", "MH": "Maharashtra", "MN": "Manipur", "ML": "Meghalaya", "MZ": "Mizoram", "NL": "Nagaland", "OD": "Odisha", "OR": "Odisha", "PB": "Punjab", "PY": "Puducherry", "RJ": "Rajasthan", "SK": "Sikkim", "TN": "Tamil Nadu", "TS": "Telangana", "TG": "Telangana", "TR": "Tripura", "UP": "Uttar Pradesh", "UK": "Uttarakhand", "UT": "Uttarakhand", "WB": "West Bengal"}
	if state := states[strings.ToUpper(strings.TrimSpace(value))]; state != "" {
		return state
	}
	return strings.TrimSpace(value)
}
