package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/wecratfs/commerce/internal/adapters/shiprocket"
	application "github.com/wecratfs/commerce/internal/application/commerce"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/transport/webhook"
)

type parcelProvider struct {
	mu                                                          sync.Mutex
	providerOrderID                                             int64
	created, lookedUp, assigned, pickedUp, labelled, manifested int
	reference, otherSKU                                         string
	delivered                                                   bool
	cancellationAccepted, cancelled, outForPickup               bool
	cancelRequests                                              int
}

func (p *parcelProvider) serve(t *testing.T, w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.providerOrderID == 0 {
		p.providerOrderID = 7001
	}
	w.Header().Set("Content-Type", "application/json")
	endpoint := r.Method + " " + strings.TrimPrefix(r.URL.Path, "/v1/external")
	if endpoint == "POST /auth/login" {
		fmt.Fprint(w, `{"token":"fixture-token"}`)
		return
	}
	if r.Header.Get("Authorization") != "Bearer fixture-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch endpoint {
	case "POST /orders/create/adhoc":
		p.created++
		var input struct {
			Reference string `json:"order_id"`
			Pickup    string `json:"pickup_location"`
			State     string `json:"billing_state"`
			Method    string `json:"payment_method"`
			Items     []struct {
				SKU string `json:"sku"`
			} `json:"order_items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || len(input.Items) != 1 || input.Items[0].SKU == p.otherSKU || input.State != "Maharashtra" || input.Pickup != "Fixture Depot" || input.Method != "Prepaid" || input.Reference == "" {
			t.Error("provider received incorrect seller-scoped parcel")
		}
		p.reference = input.Reference
		// The provider stored the shipment, but the successful response was incomplete.
		fmt.Fprint(w, `{"order_id":`)
	case "GET /orders":
		p.lookedUp++
		if r.URL.Query().Get("search") != p.reference {
			t.Error("lookup did not use the original receipt")
		}
		fmt.Fprintf(w, `{"data":[{"id":%d,"channel_order_id":%q,"status":"NEW","shipments":[{"id":%d}]}]}`, p.providerOrderID, p.reference, p.providerOrderID+1)
	case "GET /settings/company/pickup":
		fmt.Fprint(w, `{"data":{"shipping_address":[{"pickup_location":"Fixture Depot","pin_code":"110030","phone_verified":1}]}}`)
	case "GET /courier/serviceability/":
		query := r.URL.Query()
		if query.Get("order_id") != fmt.Sprint(p.providerOrderID) || query.Get("pickup_postcode") != "110030" || query.Get("delivery_postcode") != "411001" {
			t.Error("courier lookup did not use the saved route")
		}
		fmt.Fprint(w, `{"currency":"INR","data":{"available_courier_companies":[{"courier_company_id":217,"courier_name":"Test Courier","rate":50}]}}`)
	case "POST /courier/assign/awb":
		p.assigned++
		var input struct {
			Courier  int64 `json:"courier_id"`
			Shipment int64 `json:"shipment_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Shipment != p.providerOrderID+1 {
			t.Error("AWB request did not use the saved shipment")
		}
		if input.Courier == 0 {
			fmt.Fprint(w, `{"awb_assign_status":0,"response":{"data":"AWB assignment failed"}}`)
			return
		}
		fmt.Fprint(w, `{"awb_assign_status":1,"response":{"data":{"awb_code":"TEST-WAYBILL","courier_name":"Test Courier"}}}`)
	case "POST /courier/generate/pickup":
		p.pickedUp++
		fmt.Fprint(w, `{"pickup_status":1,"response":{"pickup_token_number":"TEST-PICKUP"}}`)
	case "POST /courier/generate/label":
		p.labelled++
		fmt.Fprint(w, `{"label_created":1,"label_url":"https://shiprocket.in/test-label.pdf"}`)
	case "POST /manifests/generate":
		p.manifested++
		fmt.Fprint(w, `{"status":1,"manifest_url":"https://shiprocket.in/test-manifest.pdf"}`)
	case "POST /orders/cancel/shipment/awbs":
		p.cancelRequests++
		var input struct {
			AWBs []string `json:"awbs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil || len(input.AWBs) != 1 || input.AWBs[0] != "TEST-WAYBILL" {
			t.Error("cancel used the wrong waybill")
		}
		p.cancellationAccepted = true
		fmt.Fprint(w, `{"message":"Cancellation requested"}`)
	case "GET /orders/show/" + fmt.Sprint(p.providerOrderID):
		status := 3
		if p.outForPickup {
			status = 19
		}
		if p.cancellationAccepted && p.cancelled {
			status = 8
		}
		if p.delivered {
			status = 7
		}
		fmt.Fprintf(w, `{"data":{"id":%d,"channel_order_id":%q,"status":"PROCESSING","shipments":{"id":%d,"status":%d,"awb":"TEST-WAYBILL","courier":"Test Courier","pickup_token_number":"TEST-PICKUP"}}}`, p.providerOrderID, p.reference, p.providerOrderID+1, status)
	default:
		t.Errorf("unexpected provider endpoint: %s", endpoint)
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestShiprocketClaimsAndReconciliationAreSellerScoped(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := t.Context()
	first, second := newCommerceFixture(t, pool, 1, 1), newCommerceFixture(t, pool, 0, 1)
	if _, err := pool.Exec(ctx, `INSERT INTO cart_items (cart_id,variant_id,quantity) VALUES($1,$2,1)`, first.cartIDs[0], second.variantID); err != nil {
		t.Fatal(err)
	}
	repo, payments := NewCommerceRepository(pool), NewPaymentRepository(pool)
	order, err := repo.CreateOrder(ctx, first.userIDs[0], first.cartIDs[0], "shiprocket-test", domain.AddressInput{RecipientName: "Test Buyer", Phone: "9000000000", Line1: "1 Sandbox Test Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM delivery_exceptions WHERE order_id=$1`, order.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM shiprocket_shipments WHERE order_id=$1`, order.ID)
	})
	providerID := fmt.Sprintf("order_test_shiprocket_%d", order.ID)
	if _, err := payments.AttachProviderOrder(ctx, first.userIDs[0], order.OrderNumber, ports.PaymentOrder{ProviderOrderID: providerID, AmountCents: order.TotalCents, Currency: order.Currency}); err != nil {
		t.Fatal(err)
	}
	var seller, otherSeller int64
	var otherSKU string
	if err := pool.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id=$1`, first.ownerID).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT s.id,v.sku FROM product_variants v JOIN products p ON p.id=v.product_id JOIN sellers s ON s.id=p.seller_id WHERE v.id=$1`, second.variantID).Scan(&otherSeller, &otherSKU); err != nil {
		t.Fatal(err)
	}
	provider := &parcelProvider{otherSKU: otherSKU}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { provider.serve(t, w, r) }))
	t.Cleanup(server.Close)
	gateway := shiprocket.New(server.URL, "fixture@example.com", "fixture-only")
	service := application.NewShippingService(repo, gateway, "live", map[int64]string{seller: "Fixture Depot", otherSeller: "Other Depot"})
	parcel := ports.Parcel{WeightGrams: 500, LengthCM: 15, WidthCM: 12, HeightCM: 8}
	if err := service.Act(ctx, first.ownerID, seller, order.OrderNumber, "create", parcel, 0); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("unpaid create: %v", err)
	}
	if _, err = payments.ProcessWebhook(ctx, ports.PaymentWebhook{EventID: providerID + "_capture", EventType: "payment.captured", ProviderOrderID: providerID, ProviderPaymentID: providerID + "_pay", AmountCents: order.TotalCents, Currency: order.Currency}, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	// This suite exercises preserved pre-v2 parcels; new package workflows have
	// separate multi-package, capture-gate and recovery integration coverage.
	if _, err = pool.Exec(ctx, `UPDATE orders SET workflow_version=1 WHERE id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"processing", "packing"} {
		if err := repo.UpdateSellerFulfillment(ctx, first.ownerID, seller, order.OrderNumber, stage, "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Act(ctx, second.ownerID, seller, order.OrderNumber, "create", parcel, 0); !errors.Is(err, ports.ErrOrderNotFound) {
		t.Fatalf("cross-seller create: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := service.Act(ctx, first.ownerID, seller, order.OrderNumber, "create", parcel, 0); !errors.Is(err, ports.ErrShippingPending) {
			t.Fatalf("create/retry %d: %v", i, err)
		}
	}
	provider.mu.Lock()
	createCount := provider.created
	provider.mu.Unlock()
	if createCount != 1 {
		t.Fatalf("submitted %d create POSTs", createCount)
	}
	if err := repo.UpdateSellerFulfillment(ctx, first.ownerID, seller, order.OrderNumber, "shipped", "Fake Carrier", "FAKE-AWB", ""); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("manual ship bypass: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE shiprocket_shipments SET next_check_at=now()-interval '1 second' WHERE order_id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	saved, err := service.Get(ctx, first.ownerID, seller, order.OrderNumber)
	provider.mu.Lock()
	lookupCount := provider.lookedUp
	provider.mu.Unlock()
	if err != nil || saved.ProviderShipmentID != 7002 || saved.PendingAction != "" || lookupCount != 1 {
		t.Fatalf("reconciliation: %+v, %v", saved, err)
	}
	if _, err := service.Get(ctx, second.ownerID, seller, order.OrderNumber); !errors.Is(err, ports.ErrOrderNotFound) {
		t.Fatalf("cross-seller read: %v", err)
	}
	if err := service.Act(ctx, first.ownerID, seller, order.OrderNumber, "assign", ports.Parcel{}, 0); !errors.Is(err, ports.ErrShippingRejected) {
		t.Fatalf("declined AWB: %v", err)
	}
	for _, action := range []string{"pickup", "label", "manifest"} {
		if err := service.Act(ctx, first.ownerID, seller, order.OrderNumber, action, ports.Parcel{}, 0); !errors.Is(err, ports.ErrInvalidState) {
			t.Fatalf("%s without an assigned waybill: %v", action, err)
		}
	}
	for _, action := range []string{"assign", "pickup", "label", "manifest"} {
		for i := 0; i < 2; i++ {
			if err := service.Act(ctx, first.ownerID, seller, order.OrderNumber, action, ports.Parcel{}, 217); err != nil {
				t.Fatalf("%s attempt %d: %v", action, i+1, err)
			}
		}
		if action == "assign" {
			if err := service.Act(ctx, first.ownerID, seller, order.OrderNumber, "manifest", ports.Parcel{}, 0); !errors.Is(err, ports.ErrInvalidState) {
				t.Fatalf("manifest before pickup: %v", err)
			}
		}
	}
	saved, err = service.Get(ctx, first.ownerID, seller, order.OrderNumber)
	provider.mu.Lock()
	oneOfEach := provider.assigned == 2 && provider.pickedUp == 1 && provider.labelled == 1 && provider.manifested == 1
	provider.mu.Unlock()
	if err != nil || saved.AWB != "TEST-WAYBILL" || saved.PickupReference != "TEST-PICKUP" || saved.LabelURL != "https://shiprocket.in/test-label.pdf" || saved.ManifestURL != "https://shiprocket.in/test-manifest.pdf" || saved.PendingAction != "" || !oneOfEach {
		t.Fatalf("parcel actions were not saved exactly once: %+v, %v", saved, err)
	}
	var beforeScan string
	if err := pool.QueryRow(ctx, `SELECT status FROM seller_fulfillments WHERE order_id=$1 AND seller_id=$2`, order.ID, seller).Scan(&beforeScan); err != nil || beforeScan != "packing" {
		t.Fatalf("waybill, pickup or label moved fulfillment before a scan: %s, %v", beforeScan, err)
	}
	for _, hint := range []struct {
		providerID                  int64
		reference, environment, awb string
	}{{7001, "wrong-reference", "live", saved.AWB}, {7002, saved.Reference, "live", saved.AWB}, {7001, saved.Reference, "sandbox", saved.AWB}, {7001, saved.Reference, "live", "OTHER-WAYBILL"}} {
		if err := repo.ScheduleShipmentCheck(ctx, hint.environment, hint.providerID, hint.reference, hint.awb); err != nil {
			t.Fatal(err)
		}
	}
	if value, err := service.Get(ctx, first.ownerID, seller, order.OrderNumber); err != nil || value.CanReconcile {
		t.Fatalf("unmatched webhook queued shipment: %+v, %v", value, err)
	}
	const secret = "delivery-callback-secret-for-integration-test"
	callback := webhook.DeliveryCallback(secret, service.ScheduleCheck)
	sendDelivery := func(key string, wantedStatus int) {
		t.Helper()
		body := fmt.Sprintf(`{"sr_order_id":7001,"order_id":%q,"awb":"TEST-WAYBILL","shipment_status":"DELIVERED"}`, saved.Reference)
		request := httptest.NewRequest(http.MethodPost, webhook.DeliveryCallbackPath, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-API-Key", key)
		response := httptest.NewRecorder()
		callback.ServeHTTP(response, request)
		if response.Code != wantedStatus {
			t.Fatalf("callback status=%d, want %d", response.Code, wantedStatus)
		}
	}
	sendDelivery("wrong-secret", http.StatusUnauthorized)
	if value, err := service.Get(ctx, first.ownerID, seller, order.OrderNumber); err != nil || value.CanReconcile {
		t.Fatalf("unauthenticated callback queued shipment: %+v, %v", value, err)
	}
	sendDelivery(secret, http.StatusOK)
	if err := service.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM seller_fulfillments WHERE order_id=$1 AND seller_id=$2`, order.ID, seller).Scan(&beforeScan); err != nil || beforeScan != "packing" {
		t.Fatalf("callback status overrode provider status: %s, %v", beforeScan, err)
	}
	provider.mu.Lock()
	provider.delivered = true
	provider.mu.Unlock()
	sendDelivery(secret, http.StatusOK)
	sendDelivery(secret, http.StatusOK)
	if err := service.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	var stage, overall string
	if err := pool.QueryRow(ctx, `SELECT sf.status,o.status FROM orders o JOIN seller_fulfillments sf ON sf.order_id=o.id WHERE o.id=$1 AND sf.seller_id=$2`, order.ID, seller).Scan(&stage, &overall); err != nil || stage != "delivered" || overall != "processing" {
		t.Fatalf("seller=%s overall=%s, %v", stage, overall, err)
	}
	projected, err := repo.GetOrder(ctx, first.userIDs[0], order.OrderNumber)
	if err != nil || len(projected.Fulfillments) != 2 {
		t.Fatalf("customer parcel projection: %+v, %v", projected.Fulfillments, err)
	}
	tracked := false
	for _, parcel := range projected.Fulfillments {
		if parcel.TrackingNumber == "TEST-WAYBILL" && parcel.Status == "delivered" {
			tracked = true
		}
	}
	if !tracked {
		t.Fatal("customer did not receive the confirmed seller waybill")
	}
}

func TestShiprocketCancellationRequiresOperatorAndVerifiedStatus(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := t.Context()
	fixture := newCommerceFixture(t, pool, 1, 1)
	user := fixture.userIDs[0]
	repo, payments := NewCommerceRepository(pool), NewPaymentRepository(pool)
	var seller int64
	if err := pool.QueryRow(ctx, `SELECT id FROM sellers WHERE owner_user_id=$1`, fixture.ownerID).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	order, err := repo.CreateOrder(ctx, user, fixture.cartIDs[0], "cancel-fixture", domain.AddressInput{RecipientName: "Test Buyer", Phone: "9000000000", Line1: "1 Test Lane", City: "Pune", State: "MH", PostalCode: "411001", CountryCode: "IN"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM delivery_exceptions WHERE order_id=$1`, order.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM shiprocket_shipments WHERE order_id=$1`, order.ID)
	})
	providerID := fmt.Sprintf("order_sr_cancel_%d", order.ID)
	if _, err := payments.AttachProviderOrder(ctx, user, order.OrderNumber, ports.PaymentOrder{ProviderOrderID: providerID, AmountCents: order.TotalCents, Currency: order.Currency}); err != nil {
		t.Fatal(err)
	}
	if _, err := payments.ProcessWebhook(ctx, ports.PaymentWebhook{EventID: providerID + "_event", EventType: "payment.captured", ProviderOrderID: providerID, ProviderPaymentID: providerID + "_pay", AmountCents: order.TotalCents, Currency: order.Currency}, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	// This suite exercises preserved pre-v2 parcels; new package workflows have
	// separate multi-package, capture-gate and recovery integration coverage.
	if _, err = pool.Exec(ctx, `UPDATE orders SET workflow_version=1 WHERE id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"processing", "packing"} {
		if err := repo.UpdateSellerFulfillment(ctx, fixture.ownerID, seller, order.OrderNumber, stage, "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	const ref = "WC-CANCEL-FIXTURE"
	if _, err := pool.Exec(ctx, `INSERT INTO shiprocket_shipments(order_id,seller_id,requested_by,environment,reference,pickup_location,weight_grams,length_cm,width_cm,height_cm,
 provider_order_id,provider_shipment_id,provider_status,awb,pickup_reference) VALUES($1,$2,$3,'live',$4,'Fixture Depot',500,15,12,9,7001,7002,'PICKUP SCHEDULED','TEST-WAYBILL','TEST-PICKUP')`, order.ID, seller, fixture.ownerID, ref); err != nil {
		t.Fatal(err)
	}
	provider := &parcelProvider{reference: ref, outForPickup: true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { provider.serve(t, w, r) }))
	defer server.Close()
	shipping := application.NewShippingService(repo, shiprocket.New(server.URL, "test@example.com", "fixture-only"), "live", nil)
	if err := shipping.Cancel(ctx, user, order.ID, seller); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("shopper cancel: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'admin')`, user); err != nil {
		t.Fatal(err)
	}
	if err := shipping.Cancel(ctx, user, order.ID, seller); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("out-for-pickup cancel: %v", err)
	}
	provider.mu.Lock()
	provider.outForPickup = false
	provider.mu.Unlock()
	if err := shipping.Cancel(ctx, user, order.ID, seller); !errors.Is(err, ports.ErrShippingPending) {
		t.Fatalf("unconfirmed cancel: %v", err)
	}
	if err := shipping.Cancel(ctx, user, order.ID, seller); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("second cancel: %v", err)
	}
	cases, err := repo.ListDeliveryExceptions(ctx, user, "live")
	if err != nil || len(cases) != 1 || cases[0].Kind != "pending_operation" || cases[0].CanResolve {
		t.Fatalf("pending cases=%+v err=%v", cases, err)
	}
	pending, oldest, exceptions, err := repo.ShippingBacklog(ctx, "live")
	if err != nil || pending != 1 || oldest.IsZero() || exceptions != 1 {
		t.Fatalf("backlog=%d oldest=%s exceptions=%d err=%v", pending, oldest, exceptions, err)
	}
	if err := repo.UpdateSellerFulfillment(ctx, fixture.ownerID, seller, order.OrderNumber, "shipped", "Manual Courier", "ACTUAL-AWB", ""); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("manual dispatch while pending: %v", err)
	}
	provider.mu.Lock()
	provider.cancelled = true
	requests := provider.cancelRequests
	provider.mu.Unlock()
	if requests != 1 {
		t.Fatalf("cancel requests=%d", requests)
	}
	if _, err := pool.Exec(ctx, `UPDATE shiprocket_shipments SET next_check_at=now() WHERE order_id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}
	if err := shipping.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateSellerFulfillment(ctx, fixture.ownerID, seller, order.OrderNumber, "shipped", "Manual Courier", "ACTUAL-AWB", ""); !errors.Is(err, ports.ErrInvalidState) {
		t.Fatalf("manual dispatch before approval: %v", err)
	}
	if err := repo.UpdateDeliveryException(ctx, user, order.ID, seller, "live", "cancelled_parcel", "resolve", "Verified cancellation and manual courier booked"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateSellerFulfillment(ctx, fixture.ownerID, seller, order.OrderNumber, "shipped", "Manual Courier", "ACTUAL-AWB", ""); err != nil {
		t.Fatalf("manual dispatch after approval: %v", err)
	}
}
