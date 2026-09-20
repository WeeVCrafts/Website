package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
)

// LocalDemoHandler exposes the additive demo-only workflows. It is registered
// only when WECRATFS_LOCAL_MODE=true and keeps all reads and writes in
// PostgreSQL so the demo cannot silently diverge from the real application.
type LocalDemoHandler struct {
	pool     *pgxpool.Pool
	sessions *applicationauth.SessionService
}

func NewLocalDemoHandler(pool *pgxpool.Pool, sessions *applicationauth.SessionService) *LocalDemoHandler {
	return &LocalDemoHandler{pool: pool, sessions: sessions}
}

func (h *LocalDemoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/local/")
	switch path {
	case "addresses":
		h.addresses(w, r)
	case "notifications":
		h.notifications(w, r)
	case "promotions":
		h.promotions(w, r)
	case "reviews":
		h.reviews(w, r)
	case "seller-finance":
		h.sellerFinance(w, r)
	case "support-tickets":
		h.supportTickets(w, r)
	case "admin/settings":
		h.adminSettings(w, r)
	default:
		writeLocalError(w, http.StatusNotFound, "NOT_FOUND", "Local demo endpoint not found.")
	}
}

func (h *LocalDemoHandler) session(r *http.Request) (int64, string, bool) {
	if h.sessions == nil {
		return 0, "", false
	}
	cookie, err := r.Cookie("wecratfs_session")
	if err != nil {
		return 0, "", false
	}
	record, err := h.sessions.Get(r.Context(), cookie.Value)
	if err != nil {
		return 0, "", false
	}
	return record.UserID, record.CSRFToken, true
}

func (h *LocalDemoHandler) requireSession(w http.ResponseWriter, r *http.Request) (int64, string, bool) {
	userID, csrf, ok := h.session(r)
	if !ok {
		writeLocalError(w, http.StatusUnauthorized, "AUTH_REQUIRED", "Sign in is required.")
	}
	return userID, csrf, ok
}

func (h *LocalDemoHandler) requireCSRF(w http.ResponseWriter, r *http.Request, csrf string) bool {
	provided := r.Header.Get("X-CSRF-Token")
	if csrf == "" || subtle.ConstantTimeCompare([]byte(csrf), []byte(provided)) != 1 {
		writeLocalError(w, http.StatusForbidden, "CSRF_REQUIRED", "A valid CSRF token is required.")
		return false
	}
	return true
}

func (h *LocalDemoHandler) hasRole(r *http.Request, userID int64, roles ...string) bool {
	var allowed bool
	_ = h.pool.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug = ANY($2::text[]))`, userID, roles).Scan(&allowed)
	return allowed
}

func (h *LocalDemoHandler) addresses(w http.ResponseWriter, r *http.Request) {
	userID, csrf, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := h.pool.Query(r.Context(), `SELECT id,recipient_name,line1,line2,city,state,postal_code,country_code FROM addresses WHERE user_id=$1 ORDER BY id DESC`, userID)
		if err != nil {
			writeLocalError(w, http.StatusInternalServerError, "ADDRESSES_UNAVAILABLE", "Addresses could not be loaded.")
			return
		}
		defer rows.Close()
		result := make([]map[string]any, 0)
		for rows.Next() {
			var id int64
			var recipient, line1, line2, city, state, postal, country string
			if rows.Scan(&id, &recipient, &line1, &line2, &city, &state, &postal, &country) == nil {
				result = append(result, map[string]any{"id": id, "recipient_name": recipient, "line1": line1, "line2": line2, "city": city, "state": state, "postal_code": postal, "country_code": country})
			}
		}
		writeLocalJSON(w, http.StatusOK, map[string]any{"data": result, "source": "postgresql"})
	case http.MethodPost:
		if !h.requireCSRF(w, r, csrf) {
			return
		}
		var input struct {
			RecipientName string `json:"recipient_name"`
			Line1         string `json:"line1"`
			Line2         string `json:"line2"`
			City          string `json:"city"`
			State         string `json:"state"`
			PostalCode    string `json:"postal_code"`
			CountryCode   string `json:"country_code"`
		}
		if !decodeLocalJSON(w, r, &input) {
			return
		}
		if input.RecipientName == "" || input.Line1 == "" || input.City == "" || input.State == "" || input.PostalCode == "" {
			writeLocalError(w, http.StatusBadRequest, "ADDRESS_INCOMPLETE", "Required address fields are missing.")
			return
		}
		if input.CountryCode == "" {
			input.CountryCode = "IN"
		}
		var id int64
		err := h.pool.QueryRow(r.Context(), `INSERT INTO addresses (user_id,recipient_name,line1,line2,city,state,postal_code,country_code) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, userID, input.RecipientName, input.Line1, input.Line2, input.City, input.State, input.PostalCode, input.CountryCode).Scan(&id)
		if err != nil {
			writeLocalError(w, http.StatusInternalServerError, "ADDRESS_CREATE_FAILED", "Address could not be saved.")
			return
		}
		writeLocalJSON(w, http.StatusCreated, map[string]any{"id": id, "source": "postgresql"})
	default:
		writeLocalError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET and POST are supported.")
	}
}

func (h *LocalDemoHandler) notifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLocalError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	userID, _, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	rows, err := h.pool.Query(r.Context(), `SELECT id,kind,title,body,read_at,created_at FROM notifications WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		writeLocalError(w, http.StatusInternalServerError, "NOTIFICATIONS_UNAVAILABLE", "Notifications could not be loaded.")
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var kind, title, body string
		var readAt *time.Time
		var created time.Time
		if rows.Scan(&id, &kind, &title, &body, &readAt, &created) == nil {
			result = append(result, map[string]any{"id": id, "kind": kind, "title": title, "body": body, "read_at": readAt, "created_at": created})
		}
	}
	writeLocalJSON(w, http.StatusOK, map[string]any{"data": result, "source": "postgresql"})
}

func (h *LocalDemoHandler) promotions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLocalError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	rows, err := h.pool.Query(r.Context(), `SELECT slug,name,description,discount_type,discount_value,status,starts_at,ends_at FROM promotions WHERE status='active' AND (starts_at IS NULL OR starts_at<=NOW()) AND (ends_at IS NULL OR ends_at>=NOW()) ORDER BY id DESC`)
	if err != nil {
		writeLocalError(w, http.StatusInternalServerError, "PROMOTIONS_UNAVAILABLE", "Promotions could not be loaded.")
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var slug, name, description, discountType, status string
		var value int64
		var starts, ends *time.Time
		if rows.Scan(&slug, &name, &description, &discountType, &value, &status, &starts, &ends) == nil {
			result = append(result, map[string]any{"slug": slug, "name": name, "description": description, "discount_type": discountType, "discount_value": value, "status": status, "starts_at": starts, "ends_at": ends})
		}
	}
	writeLocalJSON(w, http.StatusOK, map[string]any{"data": result, "source": "postgresql"})
}

func (h *LocalDemoHandler) reviews(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeLocalError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	userID, csrf, ok := h.requireSession(w, r)
	if !ok || !h.requireCSRF(w, r, csrf) {
		return
	}
	var input struct {
		ProductID int64  `json:"product_id"`
		OrderID   int64  `json:"order_id"`
		Rating    int    `json:"rating"`
		Title     string `json:"title"`
		Body      string `json:"body"`
	}
	if !decodeLocalJSON(w, r, &input) {
		return
	}
	if input.ProductID <= 0 || input.OrderID <= 0 || input.Rating < 1 || input.Rating > 5 {
		writeLocalError(w, http.StatusBadRequest, "REVIEW_INVALID", "Product, order, and a rating from 1 to 5 are required.")
		return
	}
	var id int64
	err := h.pool.QueryRow(r.Context(), `INSERT INTO product_reviews (product_id,user_id,order_id,rating,title,body,status,verified_purchase) SELECT $1,$2,$3,$4,$5,$6,'pending',TRUE WHERE EXISTS (SELECT 1 FROM orders WHERE id=$3 AND user_id=$2 AND status='delivered') RETURNING id`, input.ProductID, userID, input.OrderID, input.Rating, input.Title, input.Body).Scan(&id)
	if err != nil {
		writeLocalError(w, http.StatusBadRequest, "REVIEW_NOT_ALLOWED", "Only delivered orders owned by the signed-in customer can be reviewed.")
		return
	}
	writeLocalJSON(w, http.StatusCreated, map[string]any{"id": id, "status": "pending", "source": "postgresql"})
}

func (h *LocalDemoHandler) sellerFinance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLocalError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	userID, _, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	if !h.hasRole(r, userID, "seller_owner", "finance_operator") {
		writeLocalError(w, http.StatusForbidden, "FORBIDDEN", "Finance data is restricted to seller owners and finance operators.")
		return
	}
	rows, err := h.pool.Query(r.Context(), `SELECT s.display_name,l.gross_cents,l.commission_cents,l.net_cents,l.status,l.created_at FROM seller_ledger l JOIN sellers s ON s.id=l.seller_id WHERE s.owner_user_id=$1 OR EXISTS (SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug='finance_operator') ORDER BY l.created_at DESC LIMIT 100`, userID)
	if err != nil {
		writeLocalError(w, http.StatusInternalServerError, "FINANCE_UNAVAILABLE", "Seller finance data could not be loaded.")
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var seller, status string
		var gross, commission, net int64
		var created time.Time
		if rows.Scan(&seller, &gross, &commission, &net, &status, &created) == nil {
			result = append(result, map[string]any{"seller": seller, "gross_cents": gross, "commission_cents": commission, "net_cents": net, "status": status, "created_at": created})
		}
	}
	writeLocalJSON(w, http.StatusOK, map[string]any{"data": result, "source": "postgresql"})
}

func (h *LocalDemoHandler) supportTickets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLocalError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	userID, _, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	if !h.hasRole(r, userID, "support_agent", "marketplace_admin", "super_admin", "customer") {
		writeLocalError(w, http.StatusForbidden, "FORBIDDEN", "Support access is restricted.")
		return
	}
	rows, err := h.pool.Query(r.Context(), `SELECT t.ticket_number,t.subject,u.display_name,t.status,t.priority,t.created_at FROM support_tickets t JOIN users u ON u.id=t.user_id WHERE t.user_id=$1 OR EXISTS (SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug IN ('support_agent','marketplace_admin','super_admin')) ORDER BY t.created_at DESC LIMIT 100`, userID)
	if err != nil {
		writeLocalError(w, http.StatusInternalServerError, "SUPPORT_UNAVAILABLE", "Support tickets could not be loaded.")
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var number, subject, customer, status, priority string
		var created time.Time
		if rows.Scan(&number, &subject, &customer, &status, &priority, &created) == nil {
			if !h.hasRole(r, userID, "marketplace_admin", "super_admin") {
				customer = maskLocalName(customer)
			}
			result = append(result, map[string]any{"ticket_number": number, "subject": subject, "customer": customer, "status": status, "priority": priority, "created_at": created})
		}
	}
	writeLocalJSON(w, http.StatusOK, map[string]any{"data": result, "source": "postgresql"})
}

func (h *LocalDemoHandler) adminSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeLocalError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	userID, _, ok := h.requireSession(w, r)
	if !ok || !h.hasRole(r, userID, "super_admin") {
		writeLocalError(w, http.StatusForbidden, "FORBIDDEN", "Only Super Admins can inspect system settings.")
		return
	}
	rows, err := h.pool.Query(r.Context(), `SELECT setting_key,setting_value,updated_at FROM system_settings ORDER BY setting_key`)
	if err != nil {
		writeLocalError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "System settings could not be loaded.")
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var key string
		var value []byte
		var updated time.Time
		if rows.Scan(&key, &value, &updated) == nil {
			var decoded any
			if json.Unmarshal(value, &decoded) != nil {
				decoded = string(value)
			}
			result = append(result, map[string]any{"key": key, "value": decoded, "updated_at": updated})
		}
	}
	writeLocalJSON(w, http.StatusOK, map[string]any{"data": result, "source": "postgresql"})
}

func decodeLocalJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeLocalError(w, http.StatusBadRequest, "INVALID_JSON", "The JSON request is invalid.")
		return false
	}
	return true
}

func writeLocalJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeLocalError(w http.ResponseWriter, status int, code, message string) {
	writeLocalJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func maskLocalName(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return "Hidden customer"
	}
	return parts[0][:1] + "***"
}
