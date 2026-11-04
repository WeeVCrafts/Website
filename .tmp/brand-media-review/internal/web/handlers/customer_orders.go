package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

const customerOrderPageSize = 20

func customerOrderStatusFilter(value string) string {
	switch value {
	case "active", "delivered", "cancelled":
		return value
	}
	return ""
}

func customerOrderPageNumber(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// Only committed order-list state may be carried back from a detail page.
func customerOrderListURL(value string) string {
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || u.Path != "/orders" {
		return "/orders"
	}
	return customerOrderQueueURL(u.Query())
}

func customerOrderQueueURL(values url.Values) string {
	q := url.Values{}
	if status := customerOrderStatusFilter(values.Get("status")); status != "" {
		q.Set("status", status)
	}
	if page := customerOrderPageNumber(values.Get("page")); page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	if len(q) == 0 {
		return "/orders"
	}
	return "/orders?" + q.Encode()
}

func customerOrderDetailsURL(number string, values url.Values) string {
	return "/orders/" + url.PathEscape(number) + "?from=" + url.QueryEscape(customerOrderQueueURL(values))
}

// The customer projection keeps API ListOrders compatibility and applies filters
// before LIMIT/OFFSET. Every count and lookup is scoped to the session owner.
func (h *LiveStorefrontHandler) customerOrderPage(ctx context.Context, userID int64, page *viewmodels.CustomerPage) ([]domaincommerce.Order, error) {
	counts := map[string]int{"": 0, "active": 0, "delivered": 0, "cancelled": 0}
	rows, err := h.pool.Query(ctx, `SELECT status,COUNT(*) FROM orders WHERE user_id=$1 GROUP BY status`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			return nil, err
		}
		counts[""] += count
		switch status {
		case "pending_payment", "paid", "confirmed", "processing", "packed", "shipped":
			counts["active"] += count
		case "delivered":
			counts["delivered"] += count
		case "cancelled":
			counts["cancelled"] += count
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	status := customerOrderStatusFilter(page.Filters.Get("status"))
	if status == "" {
		page.Filters.Del("status")
	} else {
		page.Filters.Set("status", status)
	}
	page.OrderTotals = counts
	page.TotalResults = counts[status]
	page.Pages = (page.TotalResults + customerOrderPageSize - 1) / customerOrderPageSize
	page.Page = customerOrderPageNumber(page.Filters.Get("page"))
	if page.Pages == 0 {
		page.Page = 1
	} else if page.Page > page.Pages {
		page.Page = page.Pages
	}
	page.Filters.Set("page", strconv.Itoa(page.Page))
	page.OrdersPageLoaded = true
	rows, err = h.pool.Query(ctx, `SELECT order_number FROM orders WHERE user_id=$1 AND
 ($2='' OR ($2='active' AND status IN ('pending_payment','paid','confirmed','processing','packed','shipped')) OR ($2='delivered' AND status='delivered') OR ($2='cancelled' AND status='cancelled'))
 ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, userID, status, customerOrderPageSize, (page.Page-1)*customerOrderPageSize)
	if err != nil {
		return nil, err
	}
	numbers := []string{}
	for rows.Next() {
		var number string
		if err := rows.Scan(&number); err != nil {
			rows.Close()
			return nil, err
		}
		numbers = append(numbers, number)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := make([]domaincommerce.Order, 0, len(numbers))
	for _, number := range numbers {
		result = append(result, domaincommerce.Order{OrderNumber: number})
	}
	return result, nil
}

// Legacy status URLs never prove that a payment succeeded. An owned order's
// persisted confirmation is the only destination that can describe its payment.
func (h *LiveStorefrontHandler) legacyCustomerPayment(w http.ResponseWriter, r *http.Request, userID int64) {
	w.Header().Set("Cache-Control", "private, no-store")
	number := strings.TrimSpace(r.URL.Query().Get("order"))
	if number == "" {
		number = strings.TrimSpace(r.URL.Query().Get("order_number"))
	}
	if number == "" {
		http.Redirect(w, r, "/payments", http.StatusSeeOther)
		return
	}
	detail, err := h.orders.Get(r.Context(), userID, number)
	if errors.Is(err, ports.ErrOrderNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Payment details are temporarily unavailable. Try again from your orders.", http.StatusServiceUnavailable)
		return
	}
	http.Redirect(w, r, "/orders/"+url.PathEscape(detail.OrderNumber)+"/confirmation", http.StatusSeeOther)
}
