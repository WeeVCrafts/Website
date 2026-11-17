package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
)

func (h *LivePortalHandler) loadEmailHistory(r *http.Request, order, path string, filters url.Values) viewmodels.EmailDeliveryHistory {
	history := viewmodels.EmailDeliveryHistory{Order: order}
	before := r.URL.Query().Get("email_before")
	if before != "" {
		cursor, err := uuid.Parse(before)
		if err != nil {
			history.Unavailable = true
			return history
		}
		before = cursor.String()
		history.Latest = path + "?" + filters.Encode()
	}
	rows, err := postgres.NewEmailDeliveryRepository(h.pool).List(r.Context(), strings.TrimSpace(order), before)
	if err != nil {
		history.Unavailable = true
		return history
	}
	if len(rows) > 50 {
		rows = rows[:50]
		filters.Set("email_before", rows[49].ID)
		history.Next = path + "?" + filters.Encode()
	}
	history.Rows = rows
	return history
}
