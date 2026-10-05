package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *CommerceRepository) ListSellerProductVariants(ctx context.Context, ownerUserID, productID int64) ([]commerce.ProductVariant, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM products p JOIN sellers s ON s.id=p.seller_id
		JOIN users u ON u.id=$2 AND u.status='active'
		WHERE p.id=$1 AND p.status<>'archived' AND s.status='active'
		AND ((s.owner_user_id=$2) OR EXISTS (
			SELECT 1 FROM seller_users su WHERE su.seller_id=s.id AND su.user_id=$2
			AND su.status='active' AND 'PRODUCT_WRITE'=ANY(su.permissions))))`, productID, ownerUserID).Scan(&allowed)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ports.ErrForbidden
	}
	return listProductVariants(ctx, r.pool, productID)
}

func (r *CommerceRepository) ListAdminProductVariants(ctx context.Context, actorID, productID int64) ([]commerce.ProductVariant, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug IN ('super_admin', 'marketplace_admin', 'seller_owner', 'seller_staff'))`, actorID).Scan(&allowed)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ports.ErrForbidden
	}
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND status<>'archived')`, productID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ports.ErrNotFound
	}
	return listProductVariants(ctx, r.pool, productID)
}

func listProductVariants(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, productID int64) ([]commerce.ProductVariant, error) {
	rows, err := q.Query(ctx, `SELECT pv.id,pv.product_id,pv.name,pv.sku,pv.price_cents,pv.cost_price_cents,
		COALESCE(pv.compare_at_cents,0),pv.compare_at_cents IS NOT NULL,
		COALESCE(i.available_quantity,0),COALESCE(i.reserved_quantity,0),pv.status
		FROM product_variants pv LEFT JOIN inventory_stock i ON i.variant_id=pv.id
		WHERE pv.product_id=$1 ORDER BY CASE WHEN pv.status='active' THEN 0 ELSE 1 END,pv.id`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	variants := make([]commerce.ProductVariant, 0)
	for rows.Next() {
		var v commerce.ProductVariant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.Name, &v.SKU, &v.PriceCents, &v.CostPriceCents, &v.CompareAtCents, &v.HasCompareAt, &v.Available, &v.Reserved, &v.Status); err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

func (r *CommerceRepository) CreateSellerProductVariant(ctx context.Context, ownerUserID, productID int64, input commerce.ProductVariantInput) (commerce.ProductVariant, error) {
	return r.createProductVariant(ctx, ownerUserID, productID, input, false)
}

func (r *CommerceRepository) CreateAdminProductVariant(ctx context.Context, actorID, productID int64, input commerce.ProductVariantInput) (commerce.ProductVariant, error) {
	return r.createProductVariant(ctx, actorID, productID, input, true)
}

func (r *CommerceRepository) createProductVariant(ctx context.Context, actorID, productID int64, input commerce.ProductVariantInput, admin bool) (commerce.ProductVariant, error) {
	var variant commerce.ProductVariant
	err := WithinTransaction(ctx, r.pool, func(txctx context.Context, tx pgx.Tx) error {
		if admin {
			if err := requireActiveSuperAdmin(txctx, tx, actorID); err != nil {
				return err
			}
			if err := lockAdminVariantProduct(txctx, tx, productID); err != nil {
				return err
			}
		} else if err := lockSellerVariantProduct(txctx, tx, actorID, productID); err != nil {
			return err
		}
		if err := tx.QueryRow(txctx, `INSERT INTO product_variants(product_id,name,sku,price_cents,compare_at_cents,cost_price_cents)
			VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, productID, input.Name, input.SKU, input.PriceCents, input.CompareAtCents, input.CostPriceCents).Scan(&variant.ID); err != nil {
			return mapCommerceError(err)
		}
		if _, err := tx.Exec(txctx, `INSERT INTO inventory_stock(variant_id,available_quantity) VALUES($1,$2)`, variant.ID, input.InitialStock); err != nil {
			return err
		}
		variant.ProductID, variant.Name, variant.SKU = productID, input.Name, input.SKU
		variant.PriceCents, variant.CostPriceCents, variant.Available, variant.Status = input.PriceCents, input.CostPriceCents, input.InitialStock, "active"
		variant.HasCompareAt = input.CompareAtCents != nil
		if input.CompareAtCents != nil {
			variant.CompareAtCents = *input.CompareAtCents
		}
		if err := recordVariantChange(txctx, tx, actorID, variant, "catalog.product_variant_created", map[string]any{"source": variantSource(admin)}); err != nil {
			return err
		}
		return requestProductReindex(txctx, tx, productID, "draft")
	})
	return variant, err
}

func (r *CommerceRepository) UpdateSellerProductVariant(ctx context.Context, ownerUserID, productID, variantID int64, input commerce.ProductVariantInput) (commerce.ProductVariant, error) {
	return r.updateProductVariant(ctx, ownerUserID, productID, variantID, input, false)
}

func (r *CommerceRepository) UpdateAdminProductVariant(ctx context.Context, actorID, productID, variantID int64, input commerce.ProductVariantInput) (commerce.ProductVariant, error) {
	return r.updateProductVariant(ctx, actorID, productID, variantID, input, true)
}

func (r *CommerceRepository) updateProductVariant(ctx context.Context, actorID, productID, variantID int64, input commerce.ProductVariantInput, admin bool) (commerce.ProductVariant, error) {
	var variant commerce.ProductVariant
	err := WithinTransaction(ctx, r.pool, func(txctx context.Context, tx pgx.Tx) error {
		if admin {
			if err := requireActiveSuperAdmin(txctx, tx, actorID); err != nil {
				return err
			}
			if err := lockAdminVariantProduct(txctx, tx, productID); err != nil {
				return err
			}
		} else if err := lockSellerVariantProduct(txctx, tx, actorID, productID); err != nil {
			return err
		}
		result, err := tx.Exec(txctx, `UPDATE product_variants SET name=$3,sku=$4,price_cents=$5,compare_at_cents=$6,cost_price_cents=$7,updated_at=NOW()
			WHERE id=$1 AND product_id=$2 AND status='active'`, variantID, productID, input.Name, input.SKU, input.PriceCents, input.CompareAtCents, input.CostPriceCents)
		if err != nil {
			return mapCommerceError(err)
		}
		if result.RowsAffected() != 1 {
			return ports.ErrInvalidState
		}
		variant = commerce.ProductVariant{ID: variantID, ProductID: productID, Name: input.Name, SKU: input.SKU, PriceCents: input.PriceCents, CostPriceCents: input.CostPriceCents, Status: "active"}
		variant.HasCompareAt = input.CompareAtCents != nil
		if input.CompareAtCents != nil {
			variant.CompareAtCents = *input.CompareAtCents
		}
		if err := tx.QueryRow(txctx, `SELECT available_quantity,reserved_quantity FROM inventory_stock WHERE variant_id=$1 FOR UPDATE`, variantID).Scan(&variant.Available, &variant.Reserved); err != nil {
			return err
		}
		if err := recordVariantChange(txctx, tx, actorID, variant, "catalog.product_variant_updated", map[string]any{"source": variantSource(admin)}); err != nil {
			return err
		}
		return requestProductReindex(txctx, tx, productID, "active")
	})
	return variant, err
}

func (r *CommerceRepository) ArchiveSellerProductVariant(ctx context.Context, ownerUserID, productID, variantID int64, reason string) error {
	return r.archiveProductVariant(ctx, ownerUserID, productID, variantID, reason, false)
}

func (r *CommerceRepository) ArchiveAdminProductVariant(ctx context.Context, actorID, productID, variantID int64, reason string) error {
	return r.archiveProductVariant(ctx, actorID, productID, variantID, reason, true)
}

func (r *CommerceRepository) archiveProductVariant(ctx context.Context, actorID, productID, variantID int64, reason string, admin bool) error {
	return WithinTransaction(ctx, r.pool, func(txctx context.Context, tx pgx.Tx) error {
		if admin {
			if err := requireActiveSuperAdmin(txctx, tx, actorID); err != nil {
				return err
			}
			if err := lockAdminVariantProduct(txctx, tx, productID); err != nil {
				return err
			}
		} else if err := lockSellerVariantProduct(txctx, tx, actorID, productID); err != nil {
			return err
		}
		var status string
		var reserved int
		err := tx.QueryRow(txctx, `SELECT pv.status,i.reserved_quantity FROM product_variants pv JOIN inventory_stock i ON i.variant_id=pv.id
			WHERE pv.id=$1 AND pv.product_id=$2 FOR UPDATE OF pv,i`, variantID, productID).Scan(&status, &reserved)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "active" {
			return ports.ErrInvalidState
		}
		if reserved > 0 {
			return ports.ErrVariantReservedStock
		}
		var activeCount int
		if err := tx.QueryRow(txctx, `SELECT count(*) FROM product_variants WHERE product_id=$1 AND status='active'`, productID).Scan(&activeCount); err != nil {
			return err
		}
		if activeCount <= 1 {
			return ports.ErrFinalActiveVariant
		}
		if _, err := tx.Exec(txctx, `UPDATE product_variants SET status='archived',updated_at=NOW() WHERE id=$1`, variantID); err != nil {
			return err
		}
		metadata := map[string]any{"source": variantSource(admin), "reason": reason}
		if err := recordVariantChange(txctx, tx, actorID, commerce.ProductVariant{ID: variantID, ProductID: productID, Status: "archived"}, "catalog.product_variant_archived", metadata); err != nil {
			return err
		}
		return requestProductReindex(txctx, tx, productID, "active")
	})
}

func lockSellerVariantProduct(ctx context.Context, tx pgx.Tx, userID, productID int64) error {
	var status string
	err := tx.QueryRow(ctx, `SELECT p.status FROM products p JOIN sellers s ON s.id=p.seller_id
		JOIN users u ON u.id=$2 AND u.status='active'
		WHERE p.id=$1 AND s.status='active' AND ((s.owner_user_id=$2) OR EXISTS(
			SELECT 1 FROM seller_users su WHERE su.seller_id=s.id AND su.user_id=$2 AND su.status='active' AND 'PRODUCT_WRITE'=ANY(su.permissions)))
		FOR UPDATE OF p`, productID, userID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ErrForbidden
	}
	if err != nil {
		return err
	}
	if status == "archived" {
		return ports.ErrInvalidState
	}
	return nil
}

func lockAdminVariantProduct(ctx context.Context, tx pgx.Tx, productID int64) error {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM products WHERE id=$1 FOR UPDATE`, productID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status == "archived" {
		return ports.ErrInvalidState
	}
	return nil
}

func variantSource(admin bool) string {
	if admin {
		return "super_admin"
	}
	return "seller"
}

func recordVariantChange(ctx context.Context, tx pgx.Tx, actorID int64, variant commerce.ProductVariant, action string, metadata map[string]any) error {
	encoded, _ := json.Marshal(metadata)
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata)
		VALUES($1,$2,'product_variant',$3,$4,$5)`, fmt.Sprint(actorID), action, fmt.Sprint(variant.ID), ports.RequestID(ctx), encoded); err != nil {
		return err
	}
	return nil
}

func requestProductReindex(ctx context.Context, tx pgx.Tx, productID int64, status string) error {
	payload, _ := json.Marshal(map[string]any{"product_id": productID, "status": status})
	_, err := tx.Exec(ctx, `INSERT INTO outbox_events(event_type,aggregate_type,aggregate_id,payload)
		VALUES('ProductIndexRequested','Product',$1,$2)`, fmt.Sprint(productID), payload)
	return err
}
