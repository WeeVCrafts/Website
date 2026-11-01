package handlers

import (
	"encoding/csv"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"net/http"
	"strconv"
	"strings"
)

// adminUsers is the paged projection behind the Super Admin account directory.
// The canonical route authorizes Super Admin before loading this projection;
// writes retain RoleAdminService's transactional permission and session checks.
func (h *LivePortalHandler) adminUsers(r *http.Request, query string) []domainidentity.RoleAssignment {
	query = strings.TrimSpace(query)
	if len(query) > 120 {
		query = query[:120]
	}
	statusFilter := adminUserStatusFilter(r.URL.Query().Get("status"))
	roleFilter := adminUserRoleFilter(r.URL.Query().Get("role"))
	rows, err := h.projectionRows(r, "admin-users", `SELECT u.id,u.email,u.display_name,u.status,
 COALESCE(array_agg(ur.role_slug ORDER BY ur.role_slug) FILTER (WHERE ur.role_slug IS NOT NULL),ARRAY[]::text[]),
 to_char(u.created_at AT TIME ZONE 'Asia/Kolkata','DD Mon YYYY'),
 COALESCE(to_char(MAX(sm.last_seen_at) AT TIME ZONE 'Asia/Kolkata','DD Mon YYYY, HH12:MI AM'),'Never')
 FROM users u
 LEFT JOIN user_roles ur ON ur.user_id=u.id
 LEFT JOIN sessions_metadata sm ON sm.user_id=u.id
 WHERE u.status <> 'deleted'
 AND ($1='' OR u.email ILIKE '%'||$1||'%' OR u.display_name ILIKE '%'||$1||'%'
 OR EXISTS(SELECT 1 FROM user_roles sr WHERE sr.user_id=u.id AND sr.role_slug ILIKE '%'||$1||'%'))
 AND ($2='' OR u.status=$2)
 AND ($3='' OR EXISTS(SELECT 1 FROM user_roles rf WHERE rf.user_id=u.id AND rf.role_slug=$3))
 GROUP BY u.id ORDER BY u.created_at DESC,u.id DESC`, query, statusFilter, roleFilter)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]domainidentity.RoleAssignment, 0)
	for rows.Next() {
		var user domainidentity.RoleAssignment
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.Roles, &user.CreatedAt, &user.LastActive); err != nil {
			recordProjectionError(r, err)
			return nil
		}
		result = append(result, user)
	}
	if err := rows.Err(); err != nil {
		recordProjectionError(r, err)
		return nil
	}
	return result
}

func (h *LivePortalHandler) adminUserExport(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		http.Error(w, "PostgreSQL unavailable", http.StatusServiceUnavailable)
		return
	}
	query := portalQuery(r.URL.Query().Get("q"))
	statusFilter := adminUserStatusFilter(r.URL.Query().Get("status"))
	roleFilter := adminUserRoleFilter(r.URL.Query().Get("role"))
	rows, err := h.pool.Query(r.Context(), `SELECT u.id,u.email,u.display_name,u.status,
 COALESCE(array_agg(ur.role_slug ORDER BY ur.role_slug) FILTER (WHERE ur.role_slug IS NOT NULL),ARRAY[]::text[]),
 to_char(u.created_at AT TIME ZONE 'Asia/Kolkata','DD Mon YYYY'),
 COALESCE(to_char(MAX(sm.last_seen_at) AT TIME ZONE 'Asia/Kolkata','DD Mon YYYY, HH12:MI AM'),'Never')
 FROM users u
 LEFT JOIN user_roles ur ON ur.user_id=u.id
 LEFT JOIN sessions_metadata sm ON sm.user_id=u.id
 WHERE u.status <> 'deleted'
 AND ($1='' OR u.email ILIKE '%'||$1||'%' OR u.display_name ILIKE '%'||$1||'%' OR EXISTS(SELECT 1 FROM user_roles sr WHERE sr.user_id=u.id AND sr.role_slug ILIKE '%'||$1||'%'))
 AND ($2='' OR u.status=$2)
 AND ($3='' OR EXISTS(SELECT 1 FROM user_roles rf WHERE rf.user_id=u.id AND rf.role_slug=$3))
 GROUP BY u.id ORDER BY u.created_at DESC,u.id DESC`, query, statusFilter, roleFilter)
	if err != nil {
		http.Error(w, "User export unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=weevcrafts-users.csv")
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"Account ID", "Display name", "Email", "Role", "Status", "Last active", "Created"})
	for rows.Next() {
		var user domainidentity.RoleAssignment
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.Roles, &user.CreatedAt, &user.LastActive); err != nil {
			http.Error(w, "User export unavailable", http.StatusServiceUnavailable)
			return
		}
		_ = writer.Write([]string{strconv.FormatInt(user.ID, 10), user.DisplayName, user.Email, strings.Join(user.Roles, ", "), user.Status, user.LastActive, user.CreatedAt})
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "User export unavailable", http.StatusServiceUnavailable)
		return
	}
	writer.Flush()
}

func adminUserStatusFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active", "suspended", "pending":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func adminUserRoleFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "customer", "seller", "admin", "super_admin":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}
