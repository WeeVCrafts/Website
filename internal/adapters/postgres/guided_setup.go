package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

// lockSetupVariant keeps a guided write attached to the seller and active
// product the operator selected. It never changes catalogue approval or money.
func lockSetupVariant(ctx context.Context, tx pgx.Tx, actor, seller, variant int64) error {
	a, err := marketplaceIdentity(ctx, tx, actor)
	if err != nil {
		return err
	}
	if a.role != "super_admin" {
		return ports.ErrForbidden
	}
	var id int64
	if err = tx.QueryRow(ctx, `SELECT v.id FROM product_variants v JOIN products p ON p.id=v.product_id JOIN sellers s ON s.id=p.seller_id WHERE v.id=$1 AND p.seller_id=$2 AND v.status='active' AND p.status<>'archived' AND s.status='active' FOR UPDATE OF p,v`, variant, seller).Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return ports.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *MarketplaceRepository) SaveSetupParcel(ctx context.Context, actor, seller, variant int64, p ports.Parcel) error {
	if p.WeightGrams < 100 || p.WeightGrams > 50000 || p.LengthCM < 1 || p.LengthCM > 200 || p.WidthCM < 1 || p.WidthCM > 200 || p.HeightCM < 1 || p.HeightCM > 200 {
		return ports.ErrShippingInput
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := lockSetupVariant(ctx, tx, actor, seller, variant); err != nil {
			return err
		}
		var before ports.Parcel
		if err := tx.QueryRow(ctx, `SELECT shipping_weight_g,shipping_length_cm,shipping_width_cm,shipping_height_cm FROM product_variants WHERE id=$1`, variant).Scan(&before.WeightGrams, &before.LengthCM, &before.WidthCM, &before.HeightCM); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE product_variants SET shipping_weight_g=$2,shipping_length_cm=$3,shipping_width_cm=$4,shipping_height_cm=$5,updated_at=now() WHERE id=$1`, variant, p.WeightGrams, p.LengthCM, p.WidthCM, p.HeightCM); err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]any{"seller_id": seller, "before": before, "after": p, "source": "guided_setup"})
		_, err := tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'catalog.packed_measurements_saved','product_variant',$2,$3,$4)`, fmt.Sprint(actor), fmt.Sprint(variant), ports.RequestID(ctx), metadata)
		return err
	})
}

func (r *MarketplaceRepository) SaveSetupTaxAndClass(ctx context.Context, actor, seller, variant int64, hsn string, rate int, class, reason string) error {
	hsn = strings.TrimSpace(hsn)
	reason = strings.TrimSpace(reason)
	if !regexp.MustCompile(`^[0-9]{4,8}$`).MatchString(hsn) || rate < 0 || rate > 10000 || (class != "textile_craft" && class != "fragile_art") || len(reason) < 4 || len(reason) > 1000 {
		return ports.ErrInvalidState
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := lockSetupVariant(ctx, tx, actor, seller, variant); err != nil {
			return err
		}
		var registration string
		if err := tx.QueryRow(ctx, `SELECT registration FROM seller_invoice_profiles WHERE seller_id=$1 FOR SHARE`, seller).Scan(&registration); err != nil {
			if err == pgx.ErrNoRows {
				return ports.ErrInvalidState
			}
			return err
		}
		if registration == "unregistered" && rate != 0 {
			return ports.ErrInvalidState
		}
		if _, err := tx.Exec(ctx, `INSERT INTO variant_tax_profiles(variant_id,hsn,rate_basis_points,verified_by) VALUES($1,$2,$3,$4) ON CONFLICT(variant_id) DO UPDATE SET hsn=EXCLUDED.hsn,rate_basis_points=EXCLUDED.rate_basis_points,verified_by=EXCLUDED.verified_by,verified_at=now()`, variant, hsn, rate, actor); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE product_variants SET return_class=$2,updated_at=now() WHERE id=$1`, variant, class); err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]any{"seller_id": seller, "hsn": hsn, "rate_basis_points": rate, "return_class": class, "reason": reason})
		_, err := tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'catalog.tax_and_return_verified','product_variant',$2,$3,$4)`, fmt.Sprint(actor), fmt.Sprint(variant), ports.RequestID(ctx), metadata)
		return err
	})
}
