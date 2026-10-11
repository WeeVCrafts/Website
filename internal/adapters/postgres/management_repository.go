package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/ports"
)

type ManagementRepository struct{ pool *pgxpool.Pool }

func NewManagementRepository(pool *pgxpool.Pool) *ManagementRepository {
	return &ManagementRepository{pool: pool}
}

func managementAuthorize(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, actorID int64, section string) error {
	var allowed bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND (ur.role_slug='super_admin' OR (ur.role_slug='admin' AND $2 NOT IN ('settings','customers'))))`, actorID, section).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ports.ErrForbidden
	}
	return nil
}

func (r *ManagementRepository) ListManagement(ctx context.Context, actorID int64, section, query string, limit, offset int) (ports.ManagementResult, error) {
	result := ports.ManagementResult{Records: []ports.ManagementRecord{}}
	authorizedSection := section
	if section == "customers" {
		authorizedSection = "customer-directory"
	}
	if err := managementAuthorize(ctx, r.pool, actorID, authorizedSection); err != nil {
		return result, err
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	var projection string
	switch section {
	case "customers":
		projection = `SELECT u.id::text AS id,u.display_name AS name,u.email,u.status,''::text AS category,0 AS basis_points,false AS active,0::bigint AS products,(SELECT COUNT(*) FROM orders o WHERE o.user_id=u.id) AS orders FROM users u WHERE EXISTS(SELECT 1 FROM user_roles ur WHERE ur.user_id=u.id AND ur.role_slug='customer')`
	case "brands":
		projection = `SELECT b.slug AS id,b.name,''::text AS email,''::text AS status,''::text AS category,0 AS basis_points,false AS active,(SELECT COUNT(*) FROM products p WHERE p.brand_slug=b.slug) AS products,0::bigint AS orders FROM brands b`
	case "category-management":
		projection = `SELECT c.slug AS id,c.name,''::text AS email,''::text AS status,''::text AS category,0 AS basis_points,false AS active,(SELECT COUNT(*) FROM products p WHERE p.category_slug=c.slug) AS products,0::bigint AS orders FROM categories c`
	case "commission-rules":
		projection = `SELECT cr.id::text AS id,cr.name,''::text AS email,''::text AS status,COALESCE(cr.category_slug,'') AS category,cr.percentage_basis_points AS basis_points,cr.active,0::bigint AS products,0::bigint AS orders FROM commission_rules cr`
	case "settings":
		return result, nil
	default:
		return result, ports.ErrManagementValidation
	}
	filter := ` FROM (` + projection + `) records WHERE $1='' OR STRPOS(LOWER(name||' '||id||' '||email||' '||category),LOWER($1))>0`
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*)`+filter, query).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := r.pool.Query(ctx, `SELECT id,name,email,status,category,basis_points,active,products,orders`+filter+` ORDER BY LOWER(name),id LIMIT $2 OFFSET $3`, query, limit, offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var record ports.ManagementRecord
		if err := rows.Scan(&record.ID, &record.Name, &record.Email, &record.Status, &record.Category, &record.BasisPoints, &record.Active, &record.Products, &record.Orders); err != nil {
			return result, err
		}
		result.Records = append(result.Records, record)
	}
	return result, rows.Err()
}

func (r *ManagementRepository) MutateManagement(ctx context.Context, actorID int64, m ports.ManagementMutation) error {
	err := WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		// Serialize management changes with authorization changes for each actor.
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, actorID).Scan(&actorID); err != nil {
			return ports.ErrForbidden
		}
		if err := managementAuthorize(ctx, tx, actorID, m.Section); err != nil {
			return err
		}
		var result pgconn.CommandTag
		var err error
		switch m.Section {
		case "brands", "category-management":
			table, column := "brands", "brand_slug"
			if m.Section == "category-management" {
				table, column = "categories", "category_slug"
			}
			switch m.Action {
			case "create":
				result, err = tx.Exec(ctx, `INSERT INTO `+table+`(slug,name) VALUES($1,$2)`, m.ID, m.Name)
			case "update":
				result, err = tx.Exec(ctx, `UPDATE `+table+` SET name=$2 WHERE slug=$1`, m.ID, m.Name)
			case "delete":
				var slug string
				if err = tx.QueryRow(ctx, `SELECT slug FROM `+table+` WHERE slug=$1 FOR UPDATE`, m.ID).Scan(&slug); errors.Is(err, pgx.ErrNoRows) {
					return ports.ErrNotFound
				} else if err != nil {
					return err
				}
				var used bool
				usage := `SELECT EXISTS(SELECT 1 FROM products WHERE ` + column + `=$1)`
				if table == "categories" {
					usage += ` OR EXISTS(SELECT 1 FROM commission_rules WHERE category_slug=$1)`
				}
				if err = tx.QueryRow(ctx, usage, m.ID).Scan(&used); err != nil {
					return err
				}
				if used {
					return ports.ErrManagementConflict
				}
				result, err = tx.Exec(ctx, `DELETE FROM `+table+` WHERE slug=$1`, m.ID)
			default:
				return ports.ErrManagementValidation
			}
		case "commission-rules":
			if m.Action == "create" {
				err = tx.QueryRow(ctx, `INSERT INTO commission_rules(name,category_slug,percentage_basis_points,active,created_by) VALUES($1,NULLIF($2,''),$3,$4,$5) RETURNING id::text`, m.Name, m.Category, m.BasisPoints, m.Active, actorID).Scan(&m.ID)
			} else {
				id, parseErr := strconv.ParseInt(m.ID, 10, 64)
				if parseErr != nil || id <= 0 {
					return ports.ErrManagementValidation
				}
				if m.Action == "update" {
					result, err = tx.Exec(ctx, `UPDATE commission_rules SET name=$2,category_slug=NULLIF($3,''),percentage_basis_points=$4,active=$5,updated_at=NOW() WHERE id=$1`, id, m.Name, m.Category, m.BasisPoints, m.Active)
				} else if m.Action == "delete" {
					result, err = tx.Exec(ctx, `DELETE FROM commission_rules WHERE id=$1`, id)
				} else {
					return ports.ErrManagementValidation
				}
			}
		case "settings":
			if m.Action != "save" {
				return ports.ErrManagementValidation
			}
			values := map[string]string{"support_email": m.Settings.SupportEmail, "shipping_information": m.Settings.ShippingInformation}
			if m.SaveSocialLinks {
				var allowed bool
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN user_roles r ON r.user_id=u.id WHERE u.id=$1 AND u.status='active' AND r.role_slug='super_admin')`, actorID).Scan(&allowed); err != nil {
					return err
				}
				if !allowed {
					return ports.ErrForbidden
				}
				values["instagram_url"] = m.Settings.InstagramURL
				values["facebook_url"] = m.Settings.FacebookURL
				values["youtube_url"] = m.Settings.YouTubeURL
				values["x_url"] = m.Settings.XURL
				values["whatsapp_url"] = m.Settings.WhatsAppURL
			}
			for key, value := range values {
				encoded, _ := json.Marshal(value)
				if _, err = tx.Exec(ctx, `INSERT INTO system_settings(setting_key,setting_value,updated_by) VALUES($1,$2,$3) ON CONFLICT(setting_key) DO UPDATE SET setting_value=EXCLUDED.setting_value,updated_by=EXCLUDED.updated_by,updated_at=NOW()`, key, encoded, actorID); err != nil {
					return err
				}
			}
		default:
			return ports.ErrManagementValidation
		}
		if err != nil {
			return err
		}
		if m.Section != "settings" && !(m.Section == "commission-rules" && m.Action == "create") && result.RowsAffected() != 1 {
			return ports.ErrNotFound
		}
		metadata, _ := json.Marshal(map[string]any{"name": m.Name, "reason": m.Reason, "category": m.Category, "basis_points": m.BasisPoints, "active": m.Active})
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,$2,$3,$4,$5,$6)`, fmt.Sprint(actorID), "management."+m.Action, m.Section, m.ID, ports.RequestID(ctx), metadata)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" || pgErr.Code == "23503" {
			return ports.ErrManagementConflict
		}
		if pgErr.Code == "23514" {
			return ports.ErrManagementValidation
		}
	}
	return err
}

func (r *ManagementRepository) StorefrontSettings(ctx context.Context) (ports.StorefrontSettings, error) {
	return ReadStorefrontSettings(ctx, r.pool)
}

// ReadStorefrontSettings returns the administrator's current customer-facing details.
// Call at request time so edits take effect without restarting the application.
func ReadStorefrontSettings(ctx context.Context, pool *pgxpool.Pool) (ports.StorefrontSettings, error) {
	settings := ports.StorefrontSettings{SupportEmail: "support@weevcrafts.com", ShippingInformation: "Delivery options and shipping charges are shown at checkout."}
	rows, err := pool.Query(ctx, `SELECT setting_key,setting_value FROM system_settings WHERE setting_key IN ('support_email','shipping_information','instagram_url','facebook_url','youtube_url','x_url','whatsapp_url')`)
	if err != nil {
		return settings, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var encoded []byte
		if err := rows.Scan(&key, &encoded); err != nil {
			return settings, err
		}
		var value string
		if err := json.Unmarshal(encoded, &value); err != nil {
			return settings, err
		}
		switch key {
		case "support_email":
			settings.SupportEmail = value
		case "shipping_information":
			settings.ShippingInformation = value
		case "instagram_url":
			settings.InstagramURL = value
		case "facebook_url":
			settings.FacebookURL = value
		case "youtube_url":
			settings.YouTubeURL = value
		case "x_url":
			settings.XURL = value
		case "whatsapp_url":
			settings.WhatsAppURL = value
		}
	}
	return settings, rows.Err()
}

var _ ports.ManagementRepository = (*ManagementRepository)(nil)
