package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReturnLabelLiveUsesOwnedApprovedDatabaseReturn(t *testing.T) {
	pool, sessions, ids, records := liveSupportFixture(t)
	var orderID, returnID int64
	number := fmt.Sprintf("RETURN-SLIP-QA-%d", time.Now().UnixNano())
	ctx := context.Background()
	if err := pool.QueryRow(ctx, `INSERT INTO orders(order_number,user_id,status,currency,subtotal_cents,total_cents,address_snapshot,idempotency_scope,idempotency_key) VALUES($1,$2,'delivered','INR',12300,12300,'{"recipient_name":"Real recipient","line1":"Saved shipping road","city":"Bengaluru","state":"Karnataka","postal_code":"560001","country_code":"IN"}','return-slip-test',$1) RETURNING id`, number, ids[2]).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO return_requests(order_id,user_id,reason,status) VALUES($1,$2,'Actual saved return reason','pending') RETURNING id`, orderID, ids[2]).Scan(&returnID); err != nil {
		t.Fatal(err)
	}
	handler := &LiveStorefrontHandler{pool: pool, sessions: sessions}
	invoke := func(sessionIndex int) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		if !handler.serveReturnLabel(response, supportTestRequest(http.MethodGet, fmt.Sprintf("/returns/%d/label", returnID), nil, records[sessionIndex])) {
			t.Fatal("return label not handled")
		}
		return response
	}
	response := invoke(2)
	if response.Code != http.StatusConflict {
		t.Fatalf("pending slip allowed: %d", response.Code)
	}
	if _, err := pool.Exec(ctx, `UPDATE return_requests SET status='approved' WHERE id=$1`, returnID); err != nil {
		t.Fatal(err)
	}
	response = invoke(2)
	if response.Code != http.StatusOK {
		t.Fatalf("approved slip: %d %s", response.Code, response.Body.String())
	}
	for _, value := range []string{number, "Actual saved return reason", "Saved shipping road", "not a prepaid carrier label", "A return destination has not been recorded"} {
		if !strings.Contains(response.Body.String(), value) {
			t.Fatalf("missing actual return detail %q", value)
		}
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("return slip lacks private cache policy")
	}
	response = invoke(3)
	if response.Code != http.StatusNotFound {
		t.Fatalf("foreign return accessible: %d", response.Code)
	}
	response = invoke(0)
	if response.Code != http.StatusForbidden {
		t.Fatalf("seller can view customer slip: %d", response.Code)
	}
	if _, err := pool.Exec(ctx, `UPDATE return_requests SET status='received' WHERE id=$1`, returnID); err != nil {
		t.Fatal(err)
	}
	if response := invoke(2); response.Code != http.StatusOK {
		t.Fatalf("received slip: %d", response.Code)
	}
}
