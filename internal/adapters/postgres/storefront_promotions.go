package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/domain/storefront"
)

func readStorefrontPromotions(ctx context.Context, tx pgx.Tx) ([]storefront.Promotion, error) {
	rows, e := tx.Query(ctx, `SELECT id,slug,name,description,discount_type,discount_value,status,starts_at,ends_at,COALESCE(usage_limit,0),targeting_mode FROM promotions ORDER BY id`)
	if e != nil {
		return nil, e
	}
	var result []storefront.Promotion
	for rows.Next() {
		var p storefront.Promotion
		if e = rows.Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.Kind, &p.Value, &p.Status, &p.StartsAt, &p.EndsAt, &p.UsageLimit, &p.TargetMode); e != nil {
			rows.Close()
			return nil, e
		}
		if p.TargetMode == "selected" {
			p.TargetMode = "products"
		}
		result = append(result, p)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return nil, e
	}
	for i := range result {
		rows, e = tx.Query(ctx, `SELECT id,code,COALESCE(usage_limit,0),active FROM coupons WHERE promotion_id=$1 ORDER BY id`, result[i].ID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var c storefront.Coupon
			if e = rows.Scan(&c.ID, &c.Code, &c.UsageLimit, &c.Active); e != nil {
				rows.Close()
				return nil, e
			}
			result[i].Coupons = append(result[i].Coupons, c)
		}
		rows.Close()
		if e = rows.Err(); e != nil {
			return nil, e
		}
		rows, e = tx.Query(ctx, `SELECT product_id FROM promotion_products WHERE promotion_id=$1 ORDER BY product_id`, result[i].ID)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			var id int64
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return nil, e
			}
			result[i].ProductIDs = append(result[i].ProductIDs, id)
		}
		rows.Close()
		if e = rows.Err(); e != nil {
			return nil, e
		}
	}
	return result, nil
}
func publishStorefrontPromotions(ctx context.Context, tx pgx.Tx, actor int64, items []storefront.Promotion) error {
	keys := []string{}
	for _, p := range items {
		keys = append(keys, p.Key)
	}
	if _, e := tx.Exec(ctx, `UPDATE promotions SET status='archived',updated_at=now() WHERE NOT(slug=ANY($1::text[])) AND status<>'archived'`, keys); e != nil {
		return e
	}
	// Definitions may change. Redemption rows and usage counters never rewind.
	for _, p := range items {
		var id int64
		targetMode := p.TargetMode
		if targetMode == "products" {
			targetMode = "selected"
		}
		if p.ID > 0 {
			var key string
			if e := tx.QueryRow(ctx, `SELECT slug FROM promotions WHERE id=$1 FOR UPDATE`, p.ID).Scan(&key); e != nil {
				return e
			}
			if key != p.Key {
				return fmt.Errorf("promotion identity changed; reload")
			}
		}
		e := tx.QueryRow(ctx, `INSERT INTO promotions(slug,name,description,discount_type,discount_value,status,starts_at,ends_at,usage_limit,targeting_mode,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,0),$10,$11) ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,discount_type=EXCLUDED.discount_type,discount_value=EXCLUDED.discount_value,status=EXCLUDED.status,starts_at=EXCLUDED.starts_at,ends_at=EXCLUDED.ends_at,usage_limit=EXCLUDED.usage_limit,targeting_mode=EXCLUDED.targeting_mode,updated_at=now() RETURNING id`, p.Key, p.Name, p.Description, p.Kind, p.Value, p.Status, p.StartsAt, p.EndsAt, p.UsageLimit, targetMode, actor).Scan(&id)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `DELETE FROM promotion_products WHERE promotion_id=$1`, id); e != nil {
			return e
		}
		for _, productID := range p.ProductIDs {
			if _, e = tx.Exec(ctx, `INSERT INTO promotion_products(promotion_id,product_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, productID); e != nil {
				return fmt.Errorf("promotion references an unavailable product: %w", e)
			}
		}
		for _, c := range p.Coupons {
			if c.ID > 0 {
				result, e := tx.Exec(ctx, `UPDATE coupons SET code=$2,usage_limit=NULLIF($3,0),active=$4 WHERE id=$1 AND promotion_id=$5`, c.ID, c.Code, c.UsageLimit, c.Active, id)
				if e != nil {
					return e
				}
				if result.RowsAffected() != 1 {
					return fmt.Errorf("coupon no longer belongs to this promotion")
				}
			} else {
				// A restored snapshot of a newly-created coupon resolves by code.
				result, e := tx.Exec(ctx, `INSERT INTO coupons(promotion_id,code,usage_limit,active) VALUES($1,$2,NULLIF($3,0),$4) ON CONFLICT(code) DO UPDATE SET usage_limit=EXCLUDED.usage_limit,active=EXCLUDED.active WHERE coupons.promotion_id=EXCLUDED.promotion_id`, id, c.Code, c.UsageLimit, c.Active)
				if e != nil {
					return e
				}
				if result.RowsAffected() != 1 {
					return fmt.Errorf("coupon code already belongs to another promotion")
				}
			}
		}
	}
	return nil
}
