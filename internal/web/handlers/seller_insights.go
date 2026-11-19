package handlers

import (
	"net/http"
	"strconv"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func sellerReturnStatus(value string) string {
	switch value {
	case "pending", "approved", "rejected", "received":
		return value
	}
	return ""
}

func sellerReturnRange(value string) string {
	switch value {
	case "week", "month":
		return value
	}
	return "all"
}

func (h *LivePortalHandler) sellerWorkspaceStats(r *http.Request, sellerID int64, page viewmodels.SellerAdminPage) []viewmodels.SellerAdminStat {
	var values [4]int64
	var labels, icons, tones [4]string
	var err error
	switch page.Active {
	case "products":
		err = h.queryRow(r, `SELECT COUNT(*),COUNT(*) FILTER(WHERE p.status='approved'),COUNT(DISTINCT p.category_slug),COUNT(*) FILTER(WHERE COALESCE((SELECT SUM(i.available_quantity) FROM product_variants v JOIN inventory_stock i ON i.variant_id=v.id WHERE v.product_id=p.id AND v.status='active'),0)=0) FROM products p WHERE p.seller_id=$1 AND p.status<>'archived'`, sellerID).Scan(&values[0], &values[1], &values[2], &values[3])
		labels = [4]string{"Total products", "Active listings", "Categories", "Out of stock"}
		icons = [4]string{"inventory_2", "sell", "category", "shopping_cart"}
		tones = [4]string{"green", "orange", "gold", "green"}
	case "inventory":
		err = h.queryRow(r, `SELECT COUNT(*),COALESCE(SUM(i.available_quantity),0),COALESCE(SUM(i.reserved_quantity),0),COUNT(*) FILTER(WHERE COALESCE(i.available_quantity,0)<$2) FROM product_variants v JOIN products p ON p.id=v.product_id LEFT JOIN inventory_stock i ON i.variant_id=v.id WHERE p.seller_id=$1 AND v.status='active'`, sellerID, page.LowStockThreshold).Scan(&values[0], &values[1], &values[2], &values[3])
		labels = [4]string{"Total variants", "Available stock", "Reserved stock", "Low stock (< " + strconv.Itoa(page.LowStockThreshold) + ")"}
		icons = [4]string{"inventory_2", "category", "bookmark", "bar_chart"}
		tones = [4]string{"green", "gold", "orange", "blue"}
	case "orders":
		err = h.queryRow(r, `WITH scoped AS (SELECT o.created_at,CASE WHEN o.workflow_version=2 THEN sf.lifecycle_status ELSE CASE COALESCE(sf.status,'pending') WHEN 'pending' THEN 'new' WHEN 'processing' THEN 'accepted' WHEN 'out_for_delivery' THEN 'shipped' ELSE sf.status END END AS stage FROM orders o LEFT JOIN seller_fulfillments sf ON sf.order_id=o.id AND sf.seller_id=$1 WHERE EXISTS(SELECT 1 FROM order_items oi WHERE oi.order_id=o.id AND oi.seller_id=$1) AND (o.funding_status='funded' OR EXISTS(SELECT 1 FROM payment_attempts pa WHERE pa.order_id=o.id AND pa.status IN ('captured','refunded','funded'))) AND ((o.workflow_version=2 AND sf.order_id IS NOT NULL) OR o.status IN ('paid','processing','shipped','delivered','cancelled'))) SELECT COUNT(*) FILTER(WHERE created_at>=date_trunc('week',NOW())),COUNT(*) FILTER(WHERE stage IN ('new','accepted','packing','ready_to_ship')),COUNT(*) FILTER(WHERE stage='shipped'),COUNT(*) FILTER(WHERE stage='delivered') FROM scoped`, sellerID).Scan(&values[0], &values[1], &values[2], &values[3])
		labels = [4]string{"Orders · This week", "Awaiting dispatch", "Shipped", "Delivered"}
		icons = [4]string{"shopping_bag", "sell", "local_shipping", "check_circle"}
		tones = [4]string{"green", "orange", "gold", "green"}
	case "returns":
		err = h.queryRow(r, `SELECT COUNT(*),COUNT(*) FILTER(WHERE rr.bucket='needs_review'),COUNT(*) FILTER(WHERE rr.bucket='in_progress'),COUNT(*) FILTER(WHERE rr.bucket IN ('completed','rejected')) FROM commerce_request_summary rr WHERE rr.seller_id=$1 OR (rr.seller_id IS NULL AND EXISTS(SELECT 1 FROM order_items oi WHERE oi.order_id=rr.order_id AND oi.seller_id=$1))`, sellerID).Scan(&values[0], &values[1], &values[2], &values[3])
		labels = [4]string{"Total requests", "Needs review", "In progress", "Completed or rejected"}
		icons = [4]string{"undo", "schedule", "check", "verified_user"}
		tones = [4]string{"orange", "gold", "green", "blue"}
	default:
		return page.Stats
	}
	if err != nil {
		return nil
	}
	stats := make([]viewmodels.SellerAdminStat, 4)
	for i := range stats {
		stats[i] = viewmodels.SellerAdminStat{Value: strconv.FormatInt(values[i], 10), Label: labels[i], Icon: icons[i], Tone: tones[i]}
	}
	return stats
}
