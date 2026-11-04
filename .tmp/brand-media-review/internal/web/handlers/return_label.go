package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/pages"
)

func returnLabelID(path string) (int64, bool) {
	if !strings.HasPrefix(path, "/returns/") || !strings.HasSuffix(path, "/label") {
		return 0, false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(path, "/returns/"), "/label")
	value = strings.TrimPrefix(value, "RET")
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
}

// serveReturnLabel renders a printable authorization slip from an owned approved return.
// No carrier label, tracking number or seller address is invented.
func (h *LiveStorefrontHandler) serveReturnLabel(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/returns/") || !strings.HasSuffix(r.URL.Path, "/label") {
		return false
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return true
	}
	id, valid := returnLabelID(r.URL.Path)
	if !valid {
		http.NotFound(w, r)
		return true
	}
	userID, csrf, ok := h.session(r)
	if !ok {
		http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
		return true
	}
	if !h.customerRole(userID, r) {
		http.Error(w, "customer access required", http.StatusForbidden)
		return true
	}
	if h.pool == nil {
		http.Error(w, "return slip unavailable", http.StatusServiceUnavailable)
		return true
	}
	page := viewmodels.ReturnLabelPage{ID: strconv.FormatInt(id, 10)}
	var orderID int64
	var created time.Time
	var snapshot []byte
	err := h.pool.QueryRow(r.Context(), `SELECT o.id,o.order_number,rr.status,rr.reason,rr.created_at,o.address_snapshot,u.display_name FROM return_requests rr JOIN orders o ON o.id=rr.order_id JOIN users u ON u.id=rr.user_id WHERE rr.id=$1 AND rr.user_id=$2 AND o.user_id=$2`, id, userID).Scan(&orderID, &page.OrderNumber, &page.Status, &page.Reason, &created, &snapshot, &page.CustomerName)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return true
	}
	if err != nil {
		http.Error(w, "return slip unavailable", http.StatusServiceUnavailable)
		return true
	}
	if page.Status != "approved" && page.Status != "received" {
		http.Error(w, "return slip becomes available after approval", http.StatusConflict)
		return true
	}
	page.Created = created.Format("2 Jan 2006, 15:04")
	var address domaincommerce.AddressInput
	if err := json.Unmarshal(snapshot, &address); err != nil {
		http.Error(w, "return address record could not be read", http.StatusServiceUnavailable)
		return true
	}
	page.SenderLines = []string{address.RecipientName}
	for _, line := range []string{address.Line1, address.Line2, address.City, address.State, address.PostalCode, address.CountryCode} {
		if strings.TrimSpace(line) != "" {
			page.SenderLines = append(page.SenderLines, line)
		}
	}
	rows, err := h.pool.Query(r.Context(), `SELECT oi.product_name,oi.sku,oi.quantity,COALESCE(s.display_name,'') FROM order_items oi JOIN product_variants v ON v.id=oi.variant_id JOIN products p ON p.id=v.product_id LEFT JOIN sellers s ON s.id=p.seller_id WHERE oi.order_id=$1 ORDER BY s.id,oi.id`, orderID)
	if err != nil {
		http.Error(w, "return items could not be loaded", http.StatusServiceUnavailable)
		return true
	}
	for rows.Next() {
		var item viewmodels.ReturnLabelItem
		var quantity int
		if err := rows.Scan(&item.Name, &item.SKU, &quantity, &item.Seller); err != nil {
			rows.Close()
			http.Error(w, "return items could not be loaded", http.StatusServiceUnavailable)
			return true
		}
		item.Quantity = strconv.Itoa(quantity)
		page.Items = append(page.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		http.Error(w, "return items could not be loaded", http.StatusServiceUnavailable)
		return true
	}
	// A shipping address in an owner's account is not necessarily a return destination.
	// Only persisted marketplace instructions are shown until a return destination is configured.
	settings, err := h.pool.Query(r.Context(), `SELECT setting_key,setting_value FROM system_settings WHERE setting_key IN ('support_email','shipping_information')`)
	if err != nil {
		http.Error(w, "return instructions could not be loaded", http.StatusServiceUnavailable)
		return true
	}
	for settings.Next() {
		var key string
		var encoded []byte
		var value string
		if err := settings.Scan(&key, &encoded); err != nil {
			settings.Close()
			http.Error(w, "return instructions could not be loaded", http.StatusServiceUnavailable)
			return true
		}
		if err := json.Unmarshal(encoded, &value); err != nil {
			settings.Close()
			http.Error(w, "return instructions could not be read", http.StatusServiceUnavailable)
			return true
		}
		if key == "support_email" {
			page.SupportEmail = value
		} else {
			page.Instructions = value
		}
	}
	err = settings.Err()
	settings.Close()
	if err != nil {
		http.Error(w, "return instructions could not be loaded", http.StatusServiceUnavailable)
		return true
	}
	page.Customer = viewmodels.CustomerPage{Route: r.URL.Path, Title: "Return authorization slip", UserName: page.CustomerName, AccountUserID: userID, Authenticated: true, CSRFToken: csrf, PublicURL: h.publicURL, RoleLabel: "Customer"}
	w.Header().Set("Cache-Control", "private, no-store")
	h.render(w, r, pages.ReturnLabelLive(page))
	return true
}
