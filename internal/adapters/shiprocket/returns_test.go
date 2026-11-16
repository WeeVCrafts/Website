package shiprocket

import (
	"context"
	"encoding/json"
	"fmt"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReversePickupUsesBuyerOriginAndVerifiedSellerDestination(t *testing.T) {
	for _, reply := range []string{`{"order_id":1,"shipment_id":2,"awb_code":"FIXTURE-RETURN-AWB","status":1}`, `{"data":{"order_id":1,"shipment_id":2,"awb_code":"FIXTURE-RETURN-AWB"}}`} {
		t.Run(reply, func(t *testing.T) {
			posts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method + " " + r.URL.Path {
				case "POST /v1/external/auth/login":
					fmt.Fprint(w, `{"token":"fixture-token"}`)
				case "GET /v1/external/settings/company/pickup":
					fmt.Fprint(w, `{"data":{"shipping_address":[{"pickup_location":"Seller Warehouse","name":"Fixture seller","address":"Fixture warehouse lane","city":"Hyderabad","state":"Telangana","pin_code":"500079","phone":"9000000001","email":"fixture@example.invalid","phone_verified":1}]}}`)
				case "GET /v1/external/courier/serviceability/":
					if r.URL.Query().Get("is_return") != "1" || r.URL.Query().Get("pickup_postcode") != "411001" || r.URL.Query().Get("delivery_postcode") != "500079" {
						t.Error("wrong reverse serviceability route")
					}
					fmt.Fprint(w, `{"data":{"available_courier_companies":[{"courier_company_id":1,"courier_name":"Fixture courier","rate":90,"blocked":0}]}}`)
				case "POST /v1/external/shipments/create/return-shipment":
					posts++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["pickup_address"] != "Fixture buyer lane" || body["shipping_address"] != "Fixture warehouse lane" || body["payment_method"] != "Prepaid" || body["order_id"] != "WC-RET-7" {
						t.Errorf("return mixed up origin or seller")
					}
					fmt.Fprint(w, reply)
				case "GET /v1/external/orders/show/1":
					fmt.Fprint(w, `{"data":{"id":1,"channel_order_id":"WC-RET-7","status":"PICKUP SCHEDULED","shipments":{"id":2,"awb":"FIXTURE-RETURN-AWB","courier":"Fixture courier","status":3}}}`)
				default:
					t.Errorf("unexpected call %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected", 404)
				}
			}))
			defer srv.Close()
			g := New(srv.URL, "fixture@example.invalid", "fixture-password")
			order := ports.ReturnShippingOrder{ClaimID: 7, ShippingOrder: ports.ShippingOrder{Shipment: ports.Shipment{Reference: "WC-RET-7", PickupLocation: "Seller Warehouse", Parcel: ports.Parcel{WeightGrams: 500, LengthCM: 15, WidthCM: 12, HeightCM: 8}}, Address: domain.AddressInput{RecipientName: "Fixture buyer", Phone: "9000000000", Line1: "Fixture buyer lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"}, Email: "buyer@example.invalid", PlacedAt: time.Now(), Items: []domain.SellerOrderItem{{ProductName: "Fixture item", SKU: "FIXTURE", Quantity: 1, UnitPriceCents: 30000, LineTotalCents: 30000}}}}
			s, err := g.CreateReturn(context.Background(), order)
			if err != nil {
				t.Fatal(err)
			}
			if posts != 1 || s.AWB != "FIXTURE-RETURN-AWB" || s.ProviderStatus != "PICKUP SCHEDULED" {
				t.Fatalf("wrong reverse result %+v, posts %d", s, posts)
			}
		})
	}
}

func TestPickupReadinessExcludesUnverifiedPhones(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/external/auth/login" {
			fmt.Fprint(w, `{"token":"fixture-token"}`)
			return
		}
		fmt.Fprint(w, `{"data":{"shipping_address":[{"pickup_location":"Verified","pin_code":"500079","phone_verified":1},{"pickup_location":"Unverified","pin_code":"500084","phone_verified":0}]}}`)
	}))
	defer srv.Close()
	locations, err := New(srv.URL, "fixture@example.invalid", "fixture-password").PickupPostcodes(context.Background())
	if err != nil || len(locations) != 1 || locations["Verified"] != "500079" {
		t.Fatalf("unverified pickup exposed %v %v", locations, err)
	}
}
