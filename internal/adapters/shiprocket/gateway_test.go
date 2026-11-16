package shiprocket

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func TestShiprocketParcelLifecycle(t *testing.T) {
	logins := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/external/auth/login" {
			logins++
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input["email"] != "sandbox@example.com" || input["password"] != "fixture-secret" {
				t.Errorf("unexpected authentication payload")
			}
			fmt.Fprintf(w, `{"token":"test-token-%d"}`, logins)
			return
		}
		if r.Header.Get("Authorization") == "Bearer test-token-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token-2" {
			t.Error("missing refreshed authorization")
		}
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/external/settings/company/pickup":
			fmt.Fprint(w, `{"data":{"shipping_address":[{"pickup_location":"Test Pickup","pin_code":"110030","phone_verified":1,"address":"Private fixture address"}]}}`)
		case "GET /v1/external/courier/serviceability/":
			query := r.URL.Query()
			if query.Get("order_id") != "42" || query.Get("pickup_postcode") != "110030" || query.Get("delivery_postcode") != "122002" {
				t.Error("wrong parcel routing parameters")
			}
			fmt.Fprint(w, `{"currency":"INR","data":{"available_courier_companies":[{"courier_company_id":51,"courier_name":"Xpressbees Surface","blocked":0,"rate":54,"estimated_delivery_days":"4"},{"courier_company_id":99,"courier_name":"Blocked test courier","blocked":1},{"courier_company_id":217,"courier_name":"India Post","blocked":0,"rate":"","estimated_delivery_days":"N/A"}]}}`)
		case "POST /v1/external/orders/create/adhoc":
			var input orderInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Reference != "WC-1-S2" || input.Weight != 0.5 || input.State != "Karnataka" || input.PaymentMethod != "Prepaid" || input.Date != "2026-11-14 05:30" || input.Phone != "9000000000" || input.Subtotal.String() != "12.50" || input.Discount.String() != "1.25" || len(input.Items) != 1 {
				t.Errorf("invalid prepaid parcel: %+v, %v", input, err)
			}
			fmt.Fprint(w, `{"order_id":42,"shipment_id":17,"status":"NEW","awb_code":""}`)
		case "GET /v1/external/orders":
			if query := r.URL.Query().Get("search"); query != "" {
				fmt.Fprint(w, `{"data":[]}`)
				return
			}
			fmt.Fprint(w, `{"data":[{"id":42,"channel_order_id":"WC-1-S2","status":"NEW","shipments":[{"id":17,"status":11}]}],"meta":{"pagination":{"total_pages":1}}}`)
		case "GET /v1/external/orders/show/42":
			fmt.Fprint(w, `{"data":{"id":42,"channel_order_id":"WC-1-S2","status":"PROCESSING","shipments":{"id":17,"status":18,"awb":"123456","courier":"Test Courier","pickup_token_number":"Test Pickup"}}}`)
		case "POST /v1/external/courier/assign/awb":
			var input struct {
				Courier int64 `json:"courier_id"`
			}
			_ = json.NewDecoder(r.Body).Decode(&input)
			if input.Courier == 51 {
				fmt.Fprint(w, `{"awb_assign_status":0,"response":{"data":{"courier_id":51,"awb_assign_error":"Awb Assignment Failed"}}}`)
				return
			}
			fmt.Fprint(w, `{"awb_assign_status":1,"response":{"data":{"awb_code":"123456","courier_name":"Test Courier"}}}`)
		case "POST /v1/external/courier/generate/pickup":
			fmt.Fprint(w, `{"pickup_status":1,"response":{"pickup_token_number":"Test Pickup"}}`)
		case "POST /v1/external/courier/generate/label":
			fmt.Fprint(w, `{"label_created":1,"label_url":"https://kr-shipmultichannel.s3.ap-southeast-1.amazonaws.com/test.pdf"}`)
		case "POST /v1/external/manifests/generate":
			var input shipmentIDs
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || len(input.IDs) != 1 || input.IDs[0] != 17 {
				t.Error("manifest request must include only the saved shipment")
			}
			fmt.Fprint(w, `{"status":1,"manifest_url":"https://s3-ap-southeast-1.amazonaws.com/kr-shipmultichannel/25149/manifest/MANIFEST-3051.pdf"}`)
		default:
			t.Errorf("unexpected Shiprocket path %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := New(server.URL, "sandbox@example.com", "fixture-secret")
	ctx := t.Context()
	locations, err := client.PickupLocations(ctx)
	if _, ok := locations["Test Pickup"]; err != nil || !ok || len(locations) != 1 {
		t.Fatalf("pickup locations: %v, %v", locations, err)
	}
	order := ports.ShippingOrder{
		Shipment: ports.Shipment{Reference: "WC-1-S2", PickupLocation: "Test Pickup", Parcel: ports.Parcel{WeightGrams: 500, LengthCM: 15, WidthCM: 12, HeightCM: 8}},
		PlacedAt: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), Address: domain.AddressInput{RecipientName: "Test Buyer", State: "KA", CountryCode: "IN", Phone: "+91 90000 00000"}, Email: "buyer@example.com",
		Items: []domain.SellerOrderItem{{ProductName: "Test craft", SKU: "TEST", Quantity: 1, UnitPriceCents: 1250, LineTotalCents: 1250}}, DiscountCents: 125,
	}
	shipment, err := client.Create(ctx, order)
	if err != nil || shipment.ProviderOrderID != 42 || shipment.ProviderShipmentID != 17 {
		t.Fatalf("create: %v %+v", err, shipment)
	}
	shipment.DeliveryPostalCode = "122002"
	options, err := client.CourierOptions(ctx, shipment)
	if err != nil || len(options) != 2 || options[0] != (ports.CourierOption{ID: 51, Name: "Xpressbees Surface", EstimatedRate: "₹54.00", EstimatedDays: "4", RateCents: 5400}) || options[1] != (ports.CourierOption{ID: 217, Name: "India Post"}) {
		t.Fatalf("unexpected courier options %v, %v", options, err)
	}
	if found, err := client.Find(ctx, shipment.Reference); err != nil || found.ProviderShipmentID != shipment.ProviderShipmentID {
		t.Fatalf("recover order: %v %+v", err, found)
	}
	if _, err := client.Assign(ctx, shipment, 51); !errors.Is(err, ports.ErrShippingRejected) {
		t.Fatalf("failed AWB accepted: %v", err)
	}
	shipment, err = client.Assign(ctx, shipment, 0)
	if err != nil || shipment.AWB != "123456" {
		t.Fatalf("AWB: %v %+v", err, shipment)
	}
	shipment, err = client.Pickup(ctx, shipment)
	if err != nil || shipment.PickupReference == "" {
		t.Fatalf("pickup: %v %+v", err, shipment)
	}
	shipment, err = client.Label(ctx, shipment)
	if err != nil || shipment.LabelURL == "" {
		t.Fatalf("label: %v %+v", err, shipment)
	}
	shipment, err = client.Manifest(ctx, shipment)
	if err != nil || shipment.ManifestURL == "" {
		t.Fatalf("manifest: %v %+v", err, shipment)
	}
	shipment, err = client.Status(ctx, shipment)
	if err != nil || shipment.ProviderStatus != "IN TRANSIT" || logins != 2 {
		t.Fatalf("status: %v %+v, logins=%d", err, shipment, logins)
	}
}

func TestShiprocketDocumentURLRejectsUnrelatedBuckets(t *testing.T) {
	for _, raw := range []string{"https://s3-ap-southeast-1.amazonaws.com/other-bucket/manifest.pdf", "https://kr-shipmultichannel.s3.ap-southeast-1.amazonaws.com.example.org/manifest.pdf", "http://shiprocket.in/label.pdf"} {
		if _, err := documentURL(raw, true); !errors.Is(err, ports.ErrShippingPending) {
			t.Fatalf("unsafe document URL accepted: %s", raw)
		}
	}
}

func TestShiprocketDocumentURLAllowsSandboxCDNOnlyInSandbox(t *testing.T) {
	const manifest = "https://sr-core-cdn-stage.kartrocket.com/test/manifest.pdf"
	if got, err := documentURL(manifest, true); err != nil || got != manifest {
		t.Fatalf("sandbox manifest URL rejected: %v", err)
	}
	for _, raw := range []string{manifest, "https://sr-core-cdn-stage.kartrocket.com.example.org/test/manifest.pdf"} {
		if _, err := documentURL(raw, false); !errors.Is(err, ports.ErrShippingPending) {
			t.Fatalf("sandbox or unrelated CDN accepted outside sandbox: %s", raw)
		}
	}
}
