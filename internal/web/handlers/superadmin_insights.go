package handlers

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

type superAdminContextKey struct{}

func superAdminRequest(r *http.Request) bool {
	value, _ := r.Context().Value(superAdminContextKey{}).(bool)
	return value
}
func isUserManagementWorkspace(p viewmodels.AdminPage) bool {
	switch p.Workspace {
	case "users", "roles", "security", "security-events", "audit-logs", "settings", "system-health", "search-indexing-health", "worker-queue-health":
		return true
	}
	return p.Active == "security"
}
func withSuperAdminContext(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), superAdminContextKey{}, true))
}

func insightDays(r *http.Request) int {
	switch r.URL.Query().Get("range") {
	case "7":
		return 7
	case "90":
		return 90
	default:
		return 30
	}
}

// superAdminInsights reads aggregates separately from the bounded list projections.
// Existing services remain the only owners of operational mutations.
func (h *LivePortalHandler) superAdminInsights(r *http.Request, p *viewmodels.AdminPage) {
	p.Insights = viewmodels.AdminInsights{Days: insightDays(r)}
	p.Range = fmt.Sprintf("Last %d days", p.Insights.Days)
	q := portalQuery(p.Query)
	stat := func(icon, label string, value int64) viewmodels.AdminStat {
		return viewmodels.AdminStat{Icon: icon, Label: label, Value: strconv.FormatInt(value, 10)}
	}
	counts := func(sql string, args ...any) ([]int64, bool) {
		var a, b, c, d int64
		if err := h.queryRow(r, sql, args...).Scan(&a, &b, &c, &d); err != nil {
			return nil, false
		}
		return []int64{a, b, c, d}, true
	}
	var values []int64
	var ok bool
	switch p.Active {
	case "sellers":
		values, ok = counts(`SELECT COUNT(*),COUNT(*) FILTER(WHERE status='pending'),COUNT(*) FILTER(WHERE status='active'),COUNT(*) FILTER(WHERE status='suspended') FROM sellers WHERE status<>'closed' AND ($1='' OR concat_ws(' ',display_name,status) ILIKE '%'||$1||'%')`, q)
		if ok {
			p.Stats = []viewmodels.AdminStat{stat("groups", "Total sellers", values[0]), stat("pending_actions", "New requests", values[1]), stat("person", "Active sellers", values[2]), stat("block", "Suspended", values[3])}
		}
	case "products":
		values, ok = counts(`SELECT COUNT(*) FILTER(WHERE p.status='pending_review'),COUNT(*) FILTER(WHERE p.status='approved'),COUNT(*) FILTER(WHERE p.status='rejected'),COUNT(*) FILTER(WHERE p.status='draft') FROM products p LEFT JOIN sellers s ON s.id=p.seller_id JOIN categories c ON c.slug=p.category_slug WHERE ($1='' OR concat_ws(' ',p.name,p.slug,s.display_name,c.name,p.status) ILIKE '%'||$1||'%' OR EXISTS(SELECT 1 FROM product_variants pv WHERE pv.product_id=p.id AND pv.status='active' AND pv.sku ILIKE '%'||$1||'%'))`, q)
		if ok {
			p.Stats = []viewmodels.AdminStat{stat("schedule", "Pending review", values[0]), stat("inventory_2", "Published", values[1]), stat("close", "Rejected", values[2]), stat("description", "Drafts", values[3])}
		}
	case "inventory":
		p.StatusFilter = stockInsightFilter(r)
		values, ok = counts(`SELECT COUNT(*),COUNT(*) FILTER(WHERE COALESCE(s.available_quantity,0)<=5),COALESCE(SUM(s.reserved_quantity),0),(SELECT COUNT(*) FROM inventory_transactions WHERE created_at>=NOW()-INTERVAL '24 hours') FROM product_variants pv JOIN products p ON p.id=pv.product_id LEFT JOIN inventory_stock s ON s.variant_id=pv.id WHERE pv.status='active' AND ($1='' OR concat_ws(' ',p.name,pv.sku,CASE WHEN COALESCE(s.available_quantity,0)<=5 THEN 'low_stock' ELSE 'in_stock' END) ILIKE '%'||$1||'%') AND ($2='' OR ($2='in_stock' AND COALESCE(s.available_quantity,0)>5) OR ($2='low_stock' AND COALESCE(s.available_quantity,0) BETWEEN 1 AND 5) OR ($2='out_of_stock' AND COALESCE(s.available_quantity,0)=0))`, q, stockInsightFilter(r))
		if ok {
			p.Stats = []viewmodels.AdminStat{stat("inventory_2", "Active variants", values[0]), stat("layers", "Low stock", values[1]), stat("schedule", "Reserved units", values[2]), stat("trending_up", "Stock changes (24h)", values[3])}
		}
		p.Insights.Distribution = h.insightDistribution(r, `SELECT CASE WHEN COALESCE(s.available_quantity,0)=0 THEN 'Out of stock' WHEN s.available_quantity<=5 THEN 'Low stock' ELSE 'In stock' END,COUNT(*) FROM product_variants pv JOIN products p ON p.id=pv.product_id LEFT JOIN inventory_stock s ON s.variant_id=pv.id WHERE pv.status='active' AND ($1='' OR concat_ws(' ',p.name,pv.sku,CASE WHEN COALESCE(s.available_quantity,0)<=5 THEN 'low_stock' ELSE 'in_stock' END) ILIKE '%'||$1||'%') AND ($2='' OR ($2='in_stock' AND COALESCE(s.available_quantity,0)>5) OR ($2='low_stock' AND COALESCE(s.available_quantity,0) BETWEEN 1 AND 5) OR ($2='out_of_stock' AND COALESCE(s.available_quantity,0)=0)) GROUP BY 1 ORDER BY 1`, q, stockInsightFilter(r))
		rows, err := h.query(r, `SELECT p.name,pv.sku,it.delta,it.reason,it.created_at FROM inventory_transactions it JOIN product_variants pv ON pv.id=it.variant_id JOIN products p ON p.id=pv.product_id ORDER BY it.created_at DESC,it.id DESC LIMIT 5`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var item viewmodels.AdminStockChange
				var at time.Time
				if rows.Scan(&item.Product, &item.SKU, &item.Delta, &item.Reason, &at) == nil {
					item.Date = at.Format("2 Jan, 15:04")
					p.Insights.RecentStock = append(p.Insights.RecentStock, item)
				}
			}
		}
	case "returns":
		p.StatusFilter = returnInsightFilter(r)
		values, ok = counts(`SELECT COUNT(*) FILTER(WHERE rr.status IN('pending','approved')),COUNT(*) FILTER(WHERE rr.status='pending'),COUNT(*) FILTER(WHERE rr.status='approved'),COUNT(*) FILTER(WHERE rr.status IN('received','rejected') AND rr.updated_at>=date_trunc('month',NOW())) FROM return_requests rr JOIN orders o ON o.id=rr.order_id JOIN users u ON u.id=rr.user_id WHERE ($1='' OR concat_ws(' ','RTN-'||rr.id::text,o.order_number,u.display_name,rr.reason,rr.status) ILIKE '%'||$1||'%') AND ($2='' OR rr.status=$2) AND rr.created_at>=date_trunc('day',NOW() AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata'-($3::int-1)*INTERVAL '1 day'`, q, returnInsightFilter(r), p.Insights.Days)
		if ok {
			p.Stats = []viewmodels.AdminStat{stat("undo", "Open requests", values[0]), stat("schedule", "Awaiting review", values[1]), stat("verified_user", "Approved", values[2]), stat("task_alt", "Closed this month", values[3])}
		}
	case "support":
		values, ok = counts(`SELECT COUNT(*) FILTER(WHERE t.status='open'),COUNT(*) FILTER(WHERE t.status IN('in_progress','waiting_customer')),COUNT(*) FILTER(WHERE t.status IN('resolved','closed')),COUNT(*) FILTER(WHERE lower(t.subject) LIKE '%deliver%') FROM support_tickets t JOIN users u ON u.id=t.user_id LEFT JOIN orders o ON o.id=t.order_id WHERE ($1='' OR concat_ws(' ',t.ticket_number,t.subject,u.display_name,COALESCE(o.order_number,''),t.status,t.priority) ILIKE '%'||$1||'%') AND ($2='' OR t.status=$2) AND ($3::int=0 OR t.created_at>=date_trunc('day',NOW() AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata'-($3::int-1)*INTERVAL '1 day')`, q, supportInsightFilter(r), supportInsightDays(r))
		if ok {
			p.Stats = []viewmodels.AdminStat{stat("support_agent", "Open cases", values[0]), stat("schedule", "In progress", values[1]), stat("task_alt", "Resolved", values[2]), stat("local_shipping", "Delivery cases", values[3])}
		}
	case "finance":
		values, ok = counts(`SELECT COALESCE(SUM(amount_cents) FILTER(WHERE pa.status='captured'),0),COUNT(*) FILTER(WHERE pa.status='failed'),COUNT(*) FILTER(WHERE pa.status='refunded'),COALESCE(SUM(amount_cents) FILTER(WHERE pa.status='refunded'),0) FROM payment_attempts pa JOIN orders o ON o.id=pa.order_id WHERE pa.created_at>=date_trunc('day',NOW() AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata'-($1::int-1)*INTERVAL '1 day' AND ($2='' OR concat_ws(' ',o.order_number,'Payment',pa.provider,pa.status) ILIKE '%'||$2||'%')`, p.Insights.Days, q)
		if ok {
			if failed, valid := counts(`SELECT COUNT(*) FROM `+failedPaymentActivitySQL+` pa JOIN orders o ON o.id=pa.order_id WHERE pa.created_at>=date_trunc('day',NOW() AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata'-($1::int-1)*INTERVAL '1 day' AND ($2='' OR concat_ws(' ',o.order_number,'Payment',pa.provider,pa.status) ILIKE '%'||$2||'%')`, p.Insights.Days, q); valid {
				values[1] = failed[0]
			}
			p.Stats = []viewmodels.AdminStat{{Icon: "payments", Label: "Captured order totals", Value: money(values[0])}, stat("error_outline", "Failed payment attempts", values[1]), stat("undo", "Refunded payments", values[2]), {Icon: "currency_rupee", Label: "Refunded amount", Value: money(values[3])}}
			p.Insights.Total = money(values[0])
		}
		p.Insights.Trend = h.insightTrend(r, `SELECT (pa.created_at AT TIME ZONE 'Asia/Kolkata')::date,COALESCE(SUM(pa.amount_cents),0) FROM payment_attempts pa JOIN orders o ON o.id=pa.order_id WHERE pa.status='captured' AND ($2='' OR concat_ws(' ',o.order_number,'Payment',pa.provider,pa.status) ILIKE '%'||$2||'%') GROUP BY 1`, p.Insights.Days, q)
		p.Insights.Distribution = h.insightDistribution(r, `SELECT ss.status,COUNT(*) FROM seller_settlements ss JOIN sellers s ON s.id=ss.seller_id WHERE ss.created_at>=date_trunc('day',NOW() AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata'-($1::int-1)*INTERVAL '1 day' AND ($2='' OR concat_ws(' ',s.display_name,ss.reference,ss.status) ILIKE '%'||$2||'%') GROUP BY 1 ORDER BY 1`, p.Insights.Days, q)
		p.Insights.RefundTrend = h.insightTrend(r, `SELECT (pa.created_at AT TIME ZONE 'Asia/Kolkata')::date,COUNT(*) FROM payment_attempts pa JOIN orders o ON o.id=pa.order_id WHERE pa.status='refunded' AND ($2='' OR concat_ws(' ',o.order_number,'Payment',pa.provider,pa.status) ILIKE '%'||$2||'%') GROUP BY 1`, p.Insights.Days, q)
	case "security":
		if p.Workspace == "users" {
			filter := ` FROM users u WHERE u.status<>'deleted' AND ($1='' OR concat_ws(' ',u.email,u.display_name) ILIKE '%'||$1||'%' OR EXISTS(SELECT 1 FROM user_roles sr WHERE sr.user_id=u.id AND sr.role_slug ILIKE '%'||$1||'%')) AND ($2='' OR u.status=$2) AND ($3='' OR EXISTS(SELECT 1 FROM user_roles rf WHERE rf.user_id=u.id AND rf.role_slug=$3))`
			values, ok = counts(`SELECT COUNT(*),COUNT(*) FILTER(WHERE EXISTS(SELECT 1 FROM user_roles ur WHERE ur.user_id=u.id AND ur.role_slug='seller')),COUNT(*) FILTER(WHERE EXISTS(SELECT 1 FROM user_roles ur WHERE ur.user_id=u.id AND ur.role_slug='customer')),COUNT(*) FILTER(WHERE u.status='suspended')`+filter, q, p.StatusFilter, p.RoleFilter)
			if ok {
				p.Stats = []viewmodels.AdminStat{stat("groups", "Total users", values[0]), stat("person", "Sellers", values[1]), stat("shopping_cart", "Customers", values[2]), stat("block", "Suspended", values[3])}
			}
			// $1 is the range; committed account filters have their own placeholders.
			trendFilter := strings.NewReplacer("$3", "$4", "$2", "$3", "$1", "$2").Replace(filter)
			p.Insights.Trend = h.insightTrend(r, `SELECT (u.created_at AT TIME ZONE 'Asia/Kolkata')::date,COUNT(*)`+trendFilter+` GROUP BY 1`, p.Insights.Days, q, p.StatusFilter, p.RoleFilter)
			p.Insights.Distribution = h.insightDistribution(r, `SELECT COALESCE((SELECT role_slug FROM user_roles WHERE user_id=u.id LIMIT 1),'Unassigned'),COUNT(*)`+filter+` GROUP BY 1 ORDER BY 1`, q, p.StatusFilter, p.RoleFilter)
		}
	default:
		ok = true
	}
	if p.Active == "security" && p.Workspace != "users" {
		ok = true
	}
	if !ok {
		p.Stats = nil
	}
	p.Insights.Available = ok
	if projectionWarning(r, "") != "" {
		p.Insights.Available = false
		p.Stats = nil
	}
}

func (h *LivePortalHandler) insightTrend(r *http.Request, source string, days int, args ...any) []viewmodels.AdminChartPoint {
	params := append([]any{days}, args...)
	rows, err := h.query(r, `WITH points AS (`+source+`), days AS (SELECT generate_series((NOW() AT TIME ZONE 'Asia/Kolkata')::date-($1::int-1),(NOW() AT TIME ZONE 'Asia/Kolkata')::date,INTERVAL '1 day')::date AS day) SELECT to_char(days.day,'DD Mon'),COALESCE(points.value,0) FROM days LEFT JOIN points AS points(day,value) ON points.day=days.day ORDER BY days.day`, params...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]viewmodels.AdminChartPoint, 0, days)
	for rows.Next() {
		var point viewmodels.AdminChartPoint
		if rows.Scan(&point.Label, &point.Value) != nil {
			return nil
		}
		result = append(result, point)
	}
	if rows.Err() != nil {
		return nil
	}
	return result
}

func (h *LivePortalHandler) insightDistribution(r *http.Request, sql string, args ...any) []viewmodels.AdminChartSlice {
	rows, err := h.query(r, sql, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	palette := []string{"#0d7650", "#85b87d", "#e9b752", "#8a73ba"}
	result := []viewmodels.AdminChartSlice{}
	for rows.Next() {
		var item viewmodels.AdminChartSlice
		if rows.Scan(&item.Label, &item.Value) != nil {
			return nil
		}
		item.Color = palette[len(result)%len(palette)]
		result = append(result, item)
	}
	if rows.Err() != nil {
		return nil
	}
	return result
}

func stockInsightFilter(r *http.Request) string {
	switch r.URL.Query().Get("stock") {
	case "in_stock", "low_stock", "out_of_stock":
		return r.URL.Query().Get("stock")
	}
	return ""
}

func supportInsightFilter(r *http.Request) string {
	switch r.URL.Query().Get("status") {
	case "open", "in_progress", "waiting_customer", "resolved", "closed":
		return r.URL.Query().Get("status")
	}
	return ""
}
func supportInsightDays(r *http.Request) int {
	switch r.URL.Query().Get("range") {
	case "7":
		return 7
	case "30":
		return 30
	case "90":
		return 90
	}
	return 0
}
func returnInsightFilter(r *http.Request) string {
	switch r.URL.Query().Get("status") {
	case "pending", "approved", "rejected", "received":
		return r.URL.Query().Get("status")
	}
	return ""
}

// csvSafe prevents spreadsheet formula interpretation of user-entered values.
func csvSafe(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	trimmed := strings.TrimLeft(value, " \r\n")
	if trimmed != "" && strings.ContainsRune("=+-@\t", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

func (h *LivePortalHandler) superAdminExport(w http.ResponseWriter, r *http.Request, kind string) {
	var sql string
	var args []any
	var headers []string
	if kind == "returns" {
		headers = []string{"Return", "Order", "Customer", "Reason", "Status"}
		sql = `SELECT 'RTN-'||rr.id::text,o.order_number,u.display_name,rr.reason,rr.status FROM return_requests rr JOIN orders o ON o.id=rr.order_id JOIN users u ON u.id=rr.user_id WHERE ($1='' OR concat_ws(' ','RTN-'||rr.id::text,o.order_number,u.display_name,rr.reason,rr.status) ILIKE '%'||$1||'%') AND ($2='' OR rr.status=$2) AND rr.created_at>=date_trunc('day',NOW() AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata'-($3::int-1)*INTERVAL '1 day' ORDER BY rr.id DESC`
		args = []any{portalQuery(r.URL.Query().Get("q")), returnInsightFilter(r), insightDays(r)}
	} else {
		headers = []string{"Time", "Actor", "Action", "Record type", "Record ID"}
		sql = `SELECT to_char(created_at AT TIME ZONE 'Asia/Kolkata','YYYY-MM-DD HH24:MI:SS'),COALESCE(actor_id::text,''),action,resource_type,resource_id FROM audit_logs WHERE ($1='' OR concat_ws(' ',actor_id::text,action,resource_type,resource_id) ILIKE '%'||$1||'%') AND created_at>=date_trunc('day',NOW() AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata'-($2::int-1)*INTERVAL '1 day' ORDER BY id DESC`
		args = []any{portalQuery(r.URL.Query().Get("q")), insightDays(r)}
	}
	rows, err := h.query(r, sql, args...)
	if err != nil {
		http.Error(w, "Export unavailable. Try again.", 503)
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="weevcrafts-`+kind+`.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	writer := csv.NewWriter(w)
	_ = writer.Write(headers)
	for rows.Next() {
		record := make([]string, len(headers))
		dest := make([]any, len(headers))
		for i := range record {
			dest[i] = &record[i]
		}
		if rows.Scan(dest...) != nil {
			return
		}
		for i := range record {
			record[i] = csvSafe(record[i])
		}
		if writer.Write(record) != nil {
			return
		}
	}
	writer.Flush()
}
