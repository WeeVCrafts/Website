package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type RoleAdministrationRepository struct {
	pool *pgxpool.Pool
}

func NewRoleAdministrationRepository(pool *pgxpool.Pool) *RoleAdministrationRepository {
	return &RoleAdministrationRepository{pool: pool}
}

func (r *RoleAdministrationRepository) ListRoleAssignments(ctx context.Context, actorID int64, query string) ([]domainidentity.RoleAssignment, error) {
	if err := r.requireSuperAdmin(ctx, actorID); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.email, u.display_name, u.status,
		       COALESCE(array_agg(ur.role_slug ORDER BY ur.role_slug) FILTER (WHERE ur.role_slug IS NOT NULL), ARRAY[]::text[])
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE ($1 = '' OR u.email ILIKE '%' || $1 || '%' OR u.display_name ILIKE '%' || $1 || '%' OR EXISTS (SELECT 1 FROM user_roles search_role WHERE search_role.user_id = u.id AND search_role.role_slug ILIKE '%' || $1 || '%'))
		GROUP BY u.id, u.email, u.display_name, u.status
		ORDER BY u.created_at DESC, u.id DESC
		LIMIT 200`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assignments := make([]domainidentity.RoleAssignment, 0)
	for rows.Next() {
		var assignment domainidentity.RoleAssignment
		if err := rows.Scan(&assignment.ID, &assignment.Email, &assignment.DisplayName, &assignment.Status, &assignment.Roles); err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	return assignments, rows.Err()
}

func (r *RoleAdministrationRepository) UpdateRoleAssignment(ctx context.Context, actorID, userID, sellerID int64, role domainidentity.Role, grant bool, reason string) error {
	if err := r.requireSuperAdmin(ctx, actorID); err != nil {
		return err
	}
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		if err := lockRoleAdministrator(transactionContext, tx, actorID); err != nil {
			return err
		}
		var status string
		if err := tx.QueryRow(transactionContext, `SELECT status FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrNotFound
			}
			return err
		}
		if status != "active" {
			return ports.ErrRoleState
		}
		if role == domainidentity.RoleSellerOwner && grant {
			if _, err := tx.Exec(transactionContext, `INSERT INTO sellers(owner_user_id,display_name,status) SELECT id,display_name,'pending' FROM users WHERE id=$1 ON CONFLICT(owner_user_id) DO NOTHING`, userID); err != nil {
				return err
			}
		}
		if role == domainidentity.RoleSellerStaff {
			if sellerID <= 0 {
				return ports.ErrRoleState
			}
			if grant {
				var active bool
				if err := tx.QueryRow(transactionContext, `SELECT EXISTS (SELECT 1 FROM sellers WHERE id=$1 AND status='active')`, sellerID).Scan(&active); err != nil {
					return err
				}
				if !active {
					return ports.ErrSellerNotFound
				}
				if _, err := tx.Exec(transactionContext, `INSERT INTO user_roles (user_id,role_slug) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, string(role)); err != nil {
					return err
				}
				if _, err := tx.Exec(transactionContext, `INSERT INTO seller_users (seller_id,user_id,status,permissions,added_by) VALUES ($1,$2,'active',$3,$4) ON CONFLICT (seller_id,user_id) DO UPDATE SET status='active',permissions=EXCLUDED.permissions,added_by=EXCLUDED.added_by,updated_at=NOW()`, sellerID, userID, []string{"PRODUCT_READ", "ORDER_READ"}, actorID); err != nil {
					return err
				}
			} else {
				result, err := tx.Exec(transactionContext, `UPDATE seller_users SET status='revoked',updated_at=NOW() WHERE seller_id=$1 AND user_id=$2 AND status='active'`, sellerID, userID)
				if err != nil {
					return err
				}
				if result.RowsAffected() != 1 {
					return ports.ErrRoleState
				}
				var hasActiveSeller bool
				if err := tx.QueryRow(transactionContext, `SELECT EXISTS (SELECT 1 FROM seller_users WHERE user_id=$1 AND status='active')`, userID).Scan(&hasActiveSeller); err != nil {
					return err
				}
				if !hasActiveSeller {
					if _, err := tx.Exec(transactionContext, `DELETE FROM user_roles WHERE user_id=$1 AND role_slug=$2`, userID, string(role)); err != nil {
						return err
					}
				}
			}
		} else {
			if sellerID != 0 {
				return ports.ErrRoleState
			}
			if grant {
				if _, err := tx.Exec(transactionContext, `INSERT INTO user_roles (user_id, role_slug) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, string(role)); err != nil {
					return err
				}
			} else {
				if role == domainidentity.RoleSuperAdmin {
					var activeAdmins int
					if err := tx.QueryRow(transactionContext, `SELECT COUNT(*) FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.status='active' AND ur.role_slug=$1`, string(role)).Scan(&activeAdmins); err != nil {
						return err
					}
					if activeAdmins <= 1 {
						return ports.ErrRoleState
					}
				}
				result, err := tx.Exec(transactionContext, `DELETE FROM user_roles WHERE user_id = $1 AND role_slug = $2`, userID, string(role))
				if err != nil {
					return err
				}
				if result.RowsAffected() != 1 {
					return ports.ErrRoleState
				}
			}
		}
		if _, err := tx.Exec(transactionContext, `UPDATE users SET sessions_revoked_at=NOW() WHERE id=$1`, userID); err != nil {
			return err
		}
		action := "identity.role_revoked"
		if grant {
			action = "identity.role_granted"
		}
		metadata, _ := json.Marshal(map[string]any{"role": role, "grant": grant, "seller_id": sellerID, "reason": reason})
		_, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, request_id, metadata) VALUES ($1, $2, 'user', $3, $4, $5)`, fmt.Sprint(actorID), action, fmt.Sprint(userID), ports.RequestID(transactionContext), metadata)
		return err
	})
}

func (r *RoleAdministrationRepository) CreateRoleUser(ctx context.Context, actorID int64, email, displayName, passwordHash string, role domainidentity.Role, sellerID int64, reason string) (int64, error) {
	var userID int64
	if err := r.requireSuperAdmin(ctx, actorID); err != nil {
		return 0, err
	}
	err := WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		if err := lockRoleAdministrator(transactionContext, tx, actorID); err != nil {
			return err
		}
		if role == domainidentity.RoleSellerStaff {
			var active bool
			if err := tx.QueryRow(transactionContext, `SELECT EXISTS (SELECT 1 FROM sellers WHERE id=$1 AND status='active')`, sellerID).Scan(&active); err != nil {
				return err
			}
			if !active {
				return ports.ErrSellerNotFound
			}
		}
		if err := tx.QueryRow(transactionContext, `INSERT INTO users (email, display_name, status, email_verified_at) VALUES ($1, $2, 'active', NOW()) RETURNING id`, email, displayName).Scan(&userID); err != nil {
			if isUniqueViolation(err) {
				return ports.ErrUserAlreadyExists
			}
			return err
		}
		if _, err := tx.Exec(transactionContext, `INSERT INTO credentials (user_id, password_hash) VALUES ($1, $2)`, userID, passwordHash); err != nil {
			return err
		}
		if _, err := tx.Exec(transactionContext, `INSERT INTO user_roles (user_id, role_slug) VALUES ($1, $2)`, userID, string(role)); err != nil {
			return err
		}
		if role == domainidentity.RoleSellerStaff {
			permissions := []string{"PRODUCT_READ", "ORDER_READ"}
			if _, err := tx.Exec(transactionContext, `INSERT INTO seller_users (seller_id, user_id, status, permissions, added_by) VALUES ($1, $2, 'active', $3, $4)`, sellerID, userID, permissions, actorID); err != nil {
				return err
			}
		}
		if role == domainidentity.RoleSellerOwner {
			if _, err := tx.Exec(transactionContext, `INSERT INTO sellers(owner_user_id,display_name,status) VALUES($1,$2,'pending')`, userID, displayName); err != nil {
				return err
			}
		}
		metadata, _ := json.Marshal(map[string]any{"email": email, "role": role, "seller_id": sellerID, "reason": reason})
		_, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, request_id, metadata) VALUES ($1, 'identity.user_created', 'user', $2, $3, $4)`, fmt.Sprint(actorID), fmt.Sprint(userID), ports.RequestID(transactionContext), metadata)
		return err
	})
	if err != nil {
		return 0, err
	}
	return userID, nil
}

func (r *RoleAdministrationRepository) DeleteRoleUser(ctx context.Context, actorID, userID int64, reason string) error {
	if err := r.requireSuperAdmin(ctx, actorID); err != nil {
		return err
	}
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		if err := lockRoleAdministrator(transactionContext, tx, actorID); err != nil {
			return err
		}
		var email, status string
		if err := tx.QueryRow(transactionContext, `SELECT email, status FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&email, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrNotFound
			}
			return err
		}
		roles := make([]string, 0)
		rows, err := tx.Query(transactionContext, `SELECT role_slug FROM user_roles WHERE user_id=$1 ORDER BY role_slug`, userID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var role string
			if err := rows.Scan(&role); err != nil {
				rows.Close()
				return err
			}
			roles = append(roles, role)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if actorID == userID || status == "deleted" {
			return ports.ErrRoleState
		}
		for _, role := range roles {
			if role == string(domainidentity.RoleSuperAdmin) {
				var activeAdmins int
				if err := tx.QueryRow(transactionContext, `SELECT COUNT(*) FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.status='active' AND ur.role_slug=$1`, string(domainidentity.RoleSuperAdmin)).Scan(&activeAdmins); err != nil {
					return err
				}
				if activeAdmins <= 1 {
					return ports.ErrRoleState
				}
			}
		}
		if _, err := tx.Exec(transactionContext, `UPDATE users SET email=$2, display_name='Deleted account', status='deleted', sessions_revoked_at=NOW(), updated_at=NOW() WHERE id=$1`, userID, fmt.Sprintf("deleted+%d@invalid.weevcrafts.local", userID)); err != nil {
			return err
		}
		if _, err := tx.Exec(transactionContext, `DELETE FROM user_roles WHERE user_id=$1`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(transactionContext, `UPDATE seller_users SET status='revoked',updated_at=NOW() WHERE user_id=$1 AND status='active'`, userID); err != nil {
			return err
		}
		archivedRows, err := tx.Query(transactionContext, `UPDATE products SET status='archived',updated_at=NOW() WHERE seller_id IN (SELECT id FROM sellers WHERE owner_user_id=$1) AND status <> 'archived' RETURNING id`, userID)
		if err != nil {
			return err
		}
		archivedProductIDs := make([]int64, 0)
		for archivedRows.Next() {
			var productID int64
			if err := archivedRows.Scan(&productID); err != nil {
				archivedRows.Close()
				return err
			}
			archivedProductIDs = append(archivedProductIDs, productID)
		}
		if err := archivedRows.Err(); err != nil {
			archivedRows.Close()
			return err
		}
		archivedRows.Close()
		for _, productID := range archivedProductIDs {
			productMetadata, _ := json.Marshal(map[string]string{"source": "account_deletion", "reason": reason})
			if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'catalog.product_archived','product',$2,$3,$4)`, fmt.Sprint(actorID), fmt.Sprint(productID), ports.RequestID(transactionContext), productMetadata); err != nil {
				return err
			}
			productEvent, _ := json.Marshal(map[string]any{"product_id": productID, "status": "archived"})
			if _, err := tx.Exec(transactionContext, `INSERT INTO outbox_events(event_type,aggregate_type,aggregate_id,payload) VALUES('ProductIndexRequested','Product',$1,$2)`, fmt.Sprint(productID), productEvent); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(transactionContext, `UPDATE sellers SET status='closed',updated_at=NOW() WHERE owner_user_id=$1 AND status <> 'closed'`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(transactionContext, `UPDATE sessions_metadata SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(transactionContext, `UPDATE auth_action_tokens SET consumed_at=NOW() WHERE user_id=$1 AND consumed_at IS NULL`, userID); err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]any{"email": email, "roles": roles, "reason": reason})
		_, err = tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, request_id, metadata) VALUES ($1, 'identity.user_deleted', 'user', $2, $3, $4)`, fmt.Sprint(actorID), fmt.Sprint(userID), ports.RequestID(transactionContext), metadata)
		return err
	})
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *RoleAdministrationRepository) requireSuperAdmin(ctx context.Context, actorID int64) error {
	var allowed bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM user_roles ur JOIN users u ON u.id=ur.user_id WHERE u.status='active' AND ur.user_id = $1 AND ur.role_slug = $2)`, actorID, string(domainidentity.RoleSuperAdmin)).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ports.ErrForbidden
	}
	return nil
}

var _ ports.RoleAdministrationRepository = (*RoleAdministrationRepository)(nil)
