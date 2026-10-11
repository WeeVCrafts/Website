package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/pages"
)

func safeNotificationLink(link string) string {
	if !strings.HasPrefix(link, "/") || strings.HasPrefix(link, "//") || strings.ContainsAny(link, "\\\r\n") {
		return "/notifications"
	}
	return link
}

func (h *LivePortalHandler) serveNotifications(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != "/notifications" && r.URL.Path != "/notifications/read" && r.URL.Path != "/ui/notifications" {
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	session, ok := h.session(r)
	if !ok {
		redirectToLogin(w, r)
		return true
	}
	roles := h.userRoles(r, session.UserID)
	var active bool
	if h.pool == nil || h.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active')`, session.UserID).Scan(&active) != nil || !active {
		denyPortalAccess(w, r)
		return true
	}
	workspace := applicationauth.LoginDestination(roles)
	if workspace == "/access-unavailable" || h.pool == nil {
		denyPortalAccess(w, r)
		return true
	}
	if r.URL.Path == "/notifications/read" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method not allowed", 405)
			return true
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if r.ParseForm() != nil || session.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(r.FormValue("csrf_token")), []byte(session.CSRFToken)) != 1 {
			http.Error(w, "Invalid request", 403)
			return true
		}
		if r.FormValue("all") == "true" {
			if _, err := h.pool.Exec(r.Context(), `UPDATE notifications SET read_at=NOW() WHERE user_id=$1 AND read_at IS NULL`, session.UserID); err != nil {
				http.Error(w, "Updates could not be saved", 503)
				return true
			}
			http.Redirect(w, r, "/notifications", 303)
			return true
		}
		id, err := strconv.ParseInt(r.FormValue("notification_id"), 10, 64)
		if err != nil || id < 1 {
			http.Error(w, "Invalid notification", 400)
			return true
		}
		var link string
		if err := h.pool.QueryRow(r.Context(), `UPDATE notifications SET read_at=COALESCE(read_at,NOW()) WHERE id=$1 AND user_id=$2 RETURNING link_url`, id, session.UserID).Scan(&link); err != nil {
			http.NotFound(w, r)
			return true
		}
		http.Redirect(w, r, safeNotificationLink(link), 303)
		return true
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", 405)
		return true
	}
	if r.URL.Path == "/ui/notifications" {
		var unread int64
		if err := h.pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND read_at IS NULL`, session.UserID).Scan(&unread); err != nil {
			http.Error(w, "Notifications unavailable", 503)
			return true
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{"unread": unread})
		return true
	}
	p := viewmodels.CustomerPage{IsCustomer: workspace == "/account", Route: "/notifications", Title: "Notifications", Authenticated: true, UserName: h.userDisplayName(r, session.UserID), UserEmail: h.userEmail(r, session.UserID), CSRFToken: session.CSRFToken, Now: time.Now()}
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	rows, err := h.pool.Query(r.Context(), `SELECT id,title,body,link_url,read_at IS NULL,created_at FROM notifications WHERE user_id=$1 AND ($2::bigint=0 OR id<$2) ORDER BY id DESC LIMIT 51`, session.UserID, before)
	if err != nil {
		http.Error(w, "Notifications unavailable. Please retry.", 503)
		return true
	}
	defer rows.Close()
	for rows.Next() {
		var n viewmodels.CustomerNotification
		var created time.Time
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Link, &n.Unread, &created); err != nil {
			http.Error(w, "Notifications unavailable", 503)
			return true
		}
		n.Date = created.Format("2 Jan 2006, 15:04")
		p.Notifications = append(p.Notifications, n)
	}
	if rows.Err() != nil {
		http.Error(w, "Notifications unavailable", 503)
		return true
	}
	if len(p.Notifications) > 50 {
		p.Notifications = p.Notifications[:50]
		p.NotificationsNextURL = "/notifications?before=" + strconv.FormatInt(p.Notifications[49].ID, 10)
	}
	_ = pages.Notifications(p, workspace).Render(r.Context(), w)
	return true
}
