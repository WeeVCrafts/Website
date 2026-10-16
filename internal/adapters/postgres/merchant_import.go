package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *CommerceRepository) CheckMerchantImport(ctx context.Context, actorID, sellerID int64, input commerce.ProductDraftInput) error {
	if err := managementAuthorize(ctx, r.pool, actorID, "category-management"); err != nil {
		return err
	}
	var active bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sellers s JOIN users u ON u.id=s.owner_user_id AND u.status='active' JOIN user_roles ur ON ur.user_id=u.id AND ur.role_slug='seller' WHERE s.id=$1 AND s.status='active')`, sellerID).Scan(&active); err != nil {
		return err
	}
	if !active {
		return ports.ErrForbidden
	}
	for _, category := range append([]string{input.CategorySlug}, input.CategorySlugs...) {
		var exists bool
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM categories WHERE slug=$1)`, category).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("category %q must be created by an administrator first", category)
		}
	}
	var conflict bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products p WHERE (p.slug=$1 OR EXISTS(SELECT 1 FROM product_variants v WHERE v.product_id=p.id AND v.sku=$2)) AND NOT EXISTS(SELECT 1 FROM merchant_catalog_imports mi WHERE mi.product_id=p.id AND mi.seller_id=$3 AND mi.sku=$2 AND p.slug=$1))`, input.Slug, input.SKU, sellerID).Scan(&conflict)
	if err != nil {
		return err
	}
	if conflict {
		return fmt.Errorf("SKU %q or slug %q belongs to an existing listing; it will not be overwritten", input.SKU, input.Slug)
	}
	return nil
}

// CreateMerchantImport is idempotent by seller and SKU. A resumed draft keeps
// its current price, stock, memberships and edits; only a missing photo resumes.
func (r *CommerceRepository) CreateMerchantImport(ctx context.Context, actorID, sellerID int64, input commerce.ProductDraftInput) (id int64, photoReady bool, err error) {
	err = WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requireCompanyCatalogActor(ctx, tx, actorID); err != nil {
			return err
		}
		var active bool
		if err := tx.QueryRow(ctx, `SELECT status='active' FROM sellers WHERE id=$1 FOR UPDATE`, sellerID).Scan(&active); err != nil {
			return err
		}
		if !active {
			return ports.ErrForbidden
		}
		var image, status string
		err := tx.QueryRow(ctx, `SELECT p.id,p.image_url,p.status FROM merchant_catalog_imports mi JOIN products p ON p.id=mi.product_id WHERE mi.seller_id=$1 AND mi.sku=$2 FOR UPDATE OF p`, sellerID, input.SKU).Scan(&id, &image, &status)
		if err == nil {
			if status == "archived" {
				return ports.ErrInvalidState
			}
			photoReady = image != ""
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := ensureCatalogName(ctx, tx, "brands", input.BrandSlug, input.BrandName); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO products(slug,brand_slug,category_slug,name,description,price_cents,delivery_label,seller_id,status,image_url) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'draft','') RETURNING id`, input.Slug, input.BrandSlug, input.CategorySlug, input.Name, input.Description, input.PriceCents, input.DeliveryLabel, sellerID).Scan(&id); err != nil {
			return mapCommerceError(err)
		}
		var variantID int64
		if err := tx.QueryRow(ctx, `INSERT INTO product_variants(product_id,sku,price_cents) VALUES($1,$2,$3) RETURNING id`, id, input.SKU, input.PriceCents).Scan(&variantID); err != nil {
			return mapCommerceError(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO inventory_stock(variant_id,available_quantity) VALUES($1,$2)`, variantID, input.InitialStock); err != nil {
			return err
		}
		if err := applyAdminCategories(ctx, tx, id, input); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO merchant_catalog_imports(seller_id,sku,product_id) VALUES($1,$2,$3)`, sellerID, input.SKU, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'catalog.merchant_imported','product',$2,$3,jsonb_build_object('sku',$4::text,'seller_id',$5::bigint))`, fmt.Sprint(actorID), fmt.Sprint(id), ports.RequestID(ctx), input.SKU, sellerID)
		return err
	})
	return
}
