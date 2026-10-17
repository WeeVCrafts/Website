package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

type sellerProductRecoveryKey struct{}
type sellerProductRecovery struct {
	input   map[string]string
	message string
	field   string
}

// Only the existing application handler can commit a product mutation. Buffer
// its rejected response so native form submissions get their entered data back.
type sellerProductResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *sellerProductResponse) Header() http.Header { return w.header }
func (w *sellerProductResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *sellerProductResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(b)
}

func (h *LivePortalHandler) delegateSellerProduct(w http.ResponseWriter, r *http.Request, legacy string, userID int64, csrf string) {
	mutation := requestWithPath(r, legacy)
	// Category names are authoritative database metadata, including without JS.
	{
		mutation.Body = http.MaxBytesReader(w, mutation.Body, 30<<20)
		var parseErr error
		if strings.HasPrefix(mutation.Header.Get("Content-Type"), "multipart/form-data") {
			parseErr = mutation.ParseMultipartForm(10 << 20)
		} else {
			parseErr = mutation.ParseForm()
		}
		if parseErr != nil {
			if mutation.MultipartForm != nil {
				_ = mutation.MultipartForm.RemoveAll()
			}
			http.Error(w, "The product form could not be read. Choose an image up to 25 MB and try again.", http.StatusBadRequest)
			return
		}
		if mutation.MultipartForm != nil {
			defer mutation.MultipartForm.RemoveAll()
		}
		var name string
		if err := h.queryRow(r, `SELECT name FROM categories WHERE slug=$1`, mutation.FormValue("category_slug")).Scan(&name); err == nil {
			mutation.Form.Set("category_name", name)
		}
	}
	response := &sellerProductResponse{header: make(http.Header)}
	h.delegateWithPortalRedirect(response, mutation, "/seller-admin")
	if response.status == http.StatusUnprocessableEntity || response.status == http.StatusServiceUnavailable {
		input := make(map[string]string)
		for _, name := range []string{"name", "category_slug", "category_name", "sku", "price_rupees", "compare_at_rupees", "cost_price_rupees", "initial_stock", "description", "delivery_label", "materials", "dimensions", "care", "technique", "image_url"} {
			input[name] = mutation.FormValue(name)
		}
		for _, prefix := range []string{"price", "compare_at", "cost_price"} {
			if _, present := mutation.Form[prefix+"_rupees"]; !present {
				if cents, err := strconv.ParseInt(mutation.FormValue(prefix+"_cents"), 10, 64); err == nil && cents >= 0 {
					input[prefix+"_rupees"] = strconv.FormatInt(cents/100, 10) + "." + fmt.Sprintf("%02d", cents%100)
				}
			}
		}
		message := "Product draft could not be saved. Check the product details, category, unique SKU and prices, then try again. Your entered values are retained; choose the image again if needed."
		field := response.header.Get("X-Seller-Product-Field")
		switch field {
		case "price_rupees":
			message = "Enter a price greater than zero, with up to two decimal places."
		case "compare_at_rupees":
			message = "Enter a compare-at price at least as high as the selling price, with up to two decimal places."
		case "cost_price_rupees":
			message = "Enter a non-negative unit cost, with up to two decimal places."
		case "initial_stock":
			message = "Enter a whole stock quantity between zero and 100,000."
		case "image_file":
			message = "Choose a JPEG, PNG or WebP image up to 25 MB."
		default:
			field = ""
		}
		if response.status == http.StatusServiceUnavailable {
			message = "Product saving is temporarily unavailable. Your entered values are retained. Check the saved product list before retrying if the connection was interrupted."
		}
		target := r.URL.Path
		if target == "/seller-admin/products" {
			target = "/seller-admin/products/new"
		}
		recovery := requestWithPath(r, target)
		recovery = recovery.WithContext(context.WithValue(recovery.Context(), sellerProductRecoveryKey{}, sellerProductRecovery{input: input, message: message, field: field}))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		if field != "" {
			w.Header().Set("X-Seller-Product-Field", field)
		}
		w.WriteHeader(response.status)
		h.renderSellerProductForm(w, recovery, userID, h.userDisplayName(r, userID), true, nil, csrf)
		return
	}
	for key, values := range response.header {
		w.Header()[key] = values
	}
	if response.status == 0 {
		response.status = http.StatusOK
	}
	w.WriteHeader(response.status)
	_, _ = w.Write(response.body.Bytes())
}

func applySellerProductInput(r *http.Request, form *viewmodels.SellerProductForm) {
	state, ok := r.Context().Value(sellerProductRecoveryKey{}).(sellerProductRecovery)
	if !ok {
		return
	}
	form.Input, form.ErrorMessage = state.input, state.message
	form.ErrorField = state.field
	form.Name, form.CategorySlug, form.CategoryName = state.input["name"], state.input["category_slug"], state.input["category_name"]
	form.SKU, form.InitialStock, form.DeliveryLabel = state.input["sku"], state.input["initial_stock"], state.input["delivery_label"]
	form.Description, form.Image = state.input["description"], state.input["image_url"]
}

func sellerStockFilter(value string) string {
	if value == "low" || value == "out" {
		return value
	}
	return ""
}

func (h *LivePortalHandler) sellerInventoryHistory(r *http.Request, sellerID, variantID int64) []viewmodels.SellerInventoryTransaction {
	rows, err := h.query(r, `SELECT to_char(t.created_at,'DD Mon YYYY HH24:MI'),t.delta::text,t.before_quantity::text,t.after_quantity::text,t.reason FROM inventory_transactions t JOIN product_variants v ON v.id=t.variant_id JOIN products p ON p.id=v.product_id WHERE t.seller_id=$1 AND p.seller_id=$1 AND t.variant_id=$2 ORDER BY t.created_at DESC,t.id DESC LIMIT 20`, sellerID, variantID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var history []viewmodels.SellerInventoryTransaction
	for rows.Next() {
		var item viewmodels.SellerInventoryTransaction
		if rows.Scan(&item.Date, &item.Delta, &item.Before, &item.After, &item.Reason) == nil {
			if delta, _ := strconv.Atoi(item.Delta); delta > 0 {
				item.Delta = "+" + item.Delta
			}
			history = append(history, item)
		}
	}
	return history
}
