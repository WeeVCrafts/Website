package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	applicationcommerce "github.com/wecratfs/commerce/internal/application/commerce"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) sellerInventory(w http.ResponseWriter, r *http.Request) {
	if h.inventoryService == nil || r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	items, err := h.inventoryService.List(r.Context(), session.UserID)
	if errors.Is(err, ports.ErrForbidden) {
		h.renderError(w, http.StatusForbidden, "Inventory access denied", "Your seller account does not have inventory-read permission.")
		return
	}
	if err != nil {
		h.renderError(w, http.StatusServiceUnavailable, "Inventory unavailable", "We could not load the live stock records.")
		return
	}
	h.render(w, http.StatusOK, "seller-inventory", pageData{
		Brand: identity.Name, Tagline: identity.Tagline, Description: identity.Description,
		Title: "Inventory · " + identity.Name, Authenticated: true, AccountUserID: session.UserID,
		CSRFToken: session.CSRFToken, Inventory: items,
	})
}

func (h *Handler) sellerInventoryAdjust(w http.ResponseWriter, r *http.Request) {
	if h.inventoryService == nil || r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := h.sessionFromRequest(r)
	if !ok {
		h.redirect(w, r, "/login")
		return
	}
	if !h.validSessionCSRF(r, session) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	variantID, variantErr := strconv.ParseInt(r.FormValue("variant_id"), 10, 64)
	delta, deltaErr := inventoryDelta(r)
	if variantErr != nil || deltaErr != nil {
		h.renderCommerceError(w, r, applicationcommerce.ErrInvalidInventoryAdjustment, "The stock adjustment is invalid.")
		return
	}
	adjust := h.inventoryService.Adjust
	if r.URL.Path == "/super-admin/inventory/adjust" {
		adjust = h.inventoryService.AdjustAdmin
	}
	err := adjust(r.Context(), session.UserID, domaincommerce.InventoryAdjustment{
		VariantID: variantID,
		Delta:     delta,
		Reason:    r.FormValue("reason"),
	})
	if err != nil {
		h.renderCommerceError(w, r, err, "We could not save that stock adjustment.")
		return
	}
	if r.URL.Path == "/super-admin/inventory/adjust" {
		h.redirect(w, r, "/super-admin/inventory?notice=Stock+adjustment+saved")
		return
	}
	query := url.Values{"notice": {"Stock adjustment saved."}}
	query.Set("variant", strconv.FormatInt(variantID, 10))
	query.Set("q", r.FormValue("return_query"))
	query.Set("stock", validInventoryFilter(r.FormValue("return_stock")))
	if page, err := strconv.Atoi(r.FormValue("return_page")); err == nil && page > 0 && page <= 1000000 {
		query.Set("page", strconv.Itoa(page))
	}
	h.redirect(w, r, "/seller/inventory?"+query.Encode()+"#stock-"+strconv.FormatInt(variantID, 10))
}

// Explicit direction/quantity is additive; legacy signed deltas remain valid.
func inventoryDelta(r *http.Request) (int, error) {
	if r.FormValue("adjustment_type") == "" && r.FormValue("quantity") == "" {
		return strconv.Atoi(r.FormValue("delta"))
	}
	quantity, err := strconv.Atoi(r.FormValue("quantity"))
	if err != nil || quantity < 1 || quantity > 100000 {
		return 0, applicationcommerce.ErrInvalidInventoryAdjustment
	}
	switch r.FormValue("adjustment_type") {
	case "add":
		return quantity, nil
	case "remove":
		return -quantity, nil
	default:
		return 0, applicationcommerce.ErrInvalidInventoryAdjustment
	}
}

func validInventoryFilter(value string) string {
	if value == "low" || value == "out" {
		return value
	}
	return ""
}
