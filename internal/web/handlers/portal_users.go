package handlers

import (
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"net/http"
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
	rows, err := h.projectionRows(r, "admin-users", `SELECT u.id,u.email,u.display_name,u.status,
 COALESCE(array_agg(ur.role_slug ORDER BY ur.role_slug) FILTER (WHERE ur.role_slug IS NOT NULL),ARRAY[]::text[])
 FROM users u LEFT JOIN user_roles ur ON ur.user_id=u.id
 WHERE ($1='' OR u.email ILIKE '%'||$1||'%' OR u.display_name ILIKE '%'||$1||'%'
 OR EXISTS(SELECT 1 FROM user_roles sr WHERE sr.user_id=u.id AND sr.role_slug ILIKE '%'||$1||'%'))
 GROUP BY u.id ORDER BY u.created_at DESC,u.id DESC`, query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	result := make([]domainidentity.RoleAssignment, 0)
	for rows.Next() {
		var user domainidentity.RoleAssignment
		if err := rows.Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.Roles); err != nil {
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
