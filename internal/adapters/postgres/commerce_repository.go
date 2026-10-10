package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

type CommerceRepository struct {
	pool *pgxpool.Pool
}

func NewCommerceRepository(pool *pgxpool.Pool) *CommerceRepository {
	return &CommerceRepository{pool: pool}
}

func ensureCatalogName(ctx context.Context, tx pgx.Tx, table, slug, name string) error {
	var insert, selectName string
	switch table {
	case "brands":
		insert, selectName = `INSERT INTO brands(slug,name) VALUES($1,$2) ON CONFLICT(slug) DO NOTHING`, `SELECT name FROM brands WHERE slug=$1`
	case "categories":
		insert, selectName = `INSERT INTO categories(slug,name) VALUES($1,$2) ON CONFLICT(slug) DO NOTHING`, `SELECT name FROM categories WHERE slug=$1`
	default:
		return errors.New("unsupported catalog name table")
	}
	if _, err := tx.Exec(ctx, insert, slug, name); err != nil {
		return err
	}
	var current string
	if err := tx.QueryRow(ctx, selectName, slug).Scan(&current); err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(name)) {
		return ports.ErrCatalogConflict
	}
	return nil
}

func (r *CommerceRepository) EnsureSeller(ctx context.Context, ownerUserID int64, displayName string) (domaincommerce.Seller, error) {
	var seller domaincommerce.Seller
	err := WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var allowed bool
		if err := tx.QueryRow(transactionContext, `SELECT status='active' AND EXISTS(SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug='seller_owner') FROM users WHERE id=$1 FOR UPDATE`, ownerUserID).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return ports.ErrForbidden
		}
		row := tx.QueryRow(transactionContext, `
			INSERT INTO sellers (owner_user_id, display_name, status)
			VALUES ($1, $2, 'pending')
			ON CONFLICT (owner_user_id) DO UPDATE SET display_name = EXCLUDED.display_name, updated_at = NOW()
			RETURNING id, owner_user_id, display_name, status`, ownerUserID, displayName)
		if err := row.Scan(&seller.ID, &seller.OwnerUserID, &seller.DisplayName, &seller.Status); err != nil {
			return err
		}
		return nil
	})
	return seller, err
}

func (r *CommerceRepository) GetSeller(ctx context.Context, userID int64) (domaincommerce.Seller, error) {
	var seller domaincommerce.Seller
	err := r.pool.QueryRow(ctx, `
		SELECT s.id, s.owner_user_id, s.display_name, s.status
		FROM sellers s
		WHERE s.owner_user_id = $1
		   OR EXISTS (SELECT 1 FROM seller_users su WHERE su.seller_id = s.id AND su.user_id = $1 AND su.status = 'active')
		ORDER BY CASE WHEN s.owner_user_id = $1 THEN 0 ELSE 1 END, s.id
		LIMIT 1`, userID).Scan(&seller.ID, &seller.OwnerUserID, &seller.DisplayName, &seller.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaincommerce.Seller{}, ports.ErrSellerNotFound
	}
	return seller, err
}

func (r *CommerceRepository) UpdateSellerProfile(ctx context.Context, ownerUserID int64, displayName, reason string) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var sellerID int64
		err := tx.QueryRow(transactionContext, `SELECT s.id FROM sellers s JOIN users u ON u.id=s.owner_user_id AND u.status='active' WHERE s.owner_user_id=$1 AND s.status<>'closed' FOR UPDATE OF s`, ownerUserID).Scan(&sellerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrSellerNotFound
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(transactionContext, `UPDATE sellers SET display_name=$2,updated_at=NOW() WHERE id=$1`, sellerID, displayName); err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]string{"display_name": displayName, "reason": reason})
		_, err = tx.Exec(transactionContext, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'seller.profile_updated','seller',$2,$3,$4)`, fmt.Sprint(ownerUserID), fmt.Sprint(sellerID), ports.RequestID(transactionContext), metadata)
		return err
	})
}

func (r *CommerceRepository) CreateDraft(ctx context.Context, ownerUserID int64, input domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	var product domaincommerce.ManagedProduct
	err := WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var sellerID int64
		if err := tx.QueryRow(transactionContext, `SELECT id FROM sellers WHERE status = 'active' AND (owner_user_id = $1 OR EXISTS (SELECT 1 FROM seller_users WHERE seller_id = sellers.id AND user_id = $1 AND status = 'active'))`, ownerUserID).Scan(&sellerID); err != nil {
			return ports.ErrForbidden
		}
		if err := ensureCatalogName(transactionContext, tx, "brands", input.BrandSlug, input.BrandName); err != nil {
			return err
		}
		if err := ensureCatalogName(transactionContext, tx, "categories", input.CategorySlug, input.CategoryName); err != nil {
			return err
		}
		imgURL := input.ImageURL
		if imgURL == "" {
			imgURL = "/assets/images/customer/saree-maroon.webp"
		}
		var productID int64
		if err := tx.QueryRow(transactionContext, `
			INSERT INTO products (slug, brand_slug, category_slug, name, description, price_cents, compare_at_cents, delivery_label, image_url, seller_id, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'draft')
			RETURNING id`, input.Slug, input.BrandSlug, input.CategorySlug, input.Name, input.Description, input.PriceCents, input.CompareAtCents, input.DeliveryLabel, imgURL, sellerID).Scan(&productID); err != nil {
			return mapCommerceError(err)
		}
		if err := tx.QueryRow(transactionContext, `
			INSERT INTO product_variants (product_id, sku, price_cents, compare_at_cents, cost_price_cents)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id`, productID, input.SKU, input.PriceCents, input.CompareAtCents, input.CostPriceCents).Scan(&product.VariantID); err != nil {
			return mapCommerceError(err)
		}
		if _, err := tx.Exec(transactionContext, `INSERT INTO inventory_stock (variant_id, available_quantity) VALUES ($1, $2)`, product.VariantID, input.InitialStock); err != nil {
			return err
		}
		product = domaincommerce.ManagedProduct{
			ID:             productID,
			VariantID:      product.VariantID,
			SellerID:       sellerID,
			Slug:           input.Slug,
			BrandSlug:      input.BrandSlug,
			BrandName:      input.BrandName,
			Name:           input.Name,
			Description:    input.Description,
			SKU:            input.SKU,
			PriceCents:     input.PriceCents,
			CompareAtCents: 0,
			HasCompareAt:   input.CompareAtCents != nil,
			Stock:          input.InitialStock,
			Status:         "draft",
			CategorySlug:   input.CategorySlug,
			CategoryName:   input.CategoryName,
			DeliveryLabel:  input.DeliveryLabel,
		}
		if input.CompareAtCents != nil {
			product.CompareAtCents = *input.CompareAtCents
		}
		_, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, request_id, metadata) VALUES ($1, 'catalog.product_created', 'product', $2, $3, $4)`, fmt.Sprint(ownerUserID), fmt.Sprint(productID), ports.RequestID(transactionContext), []byte(`{"source":"seller"}`))
		return err
	})
	return product, err
}

func (r *CommerceRepository) GetSellerProduct(ctx context.Context, ownerUserID, productID int64) (domaincommerce.ManagedProduct, error) {
	product, err := r.getSellerProduct(ctx, r.pool, ownerUserID, productID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaincommerce.ManagedProduct{}, ports.ErrForbidden
	}
	return product, err
}

func (r *CommerceRepository) UpdateDraft(ctx context.Context, ownerUserID, productID int64, input domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	var product domaincommerce.ManagedProduct
	err := WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		current, err := r.getSellerProduct(transactionContext, tx, ownerUserID, productID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrForbidden
		}
		if err != nil {
			return err
		}
		if current.Status == "archived" {
			return ports.ErrInvalidState
		}
		if err := ensureCatalogName(transactionContext, tx, "brands", input.BrandSlug, input.BrandName); err != nil {
			return err
		}
		if err := ensureCatalogName(transactionContext, tx, "categories", input.CategorySlug, input.CategoryName); err != nil {
			return err
		}
		if input.ImageURL != "" {
			if _, err := tx.Exec(transactionContext, `UPDATE products SET slug = $1, brand_slug = $2, category_slug = $3, name = $4, description = $5, price_cents = $6, compare_at_cents = $7, delivery_label = $8, image_url = $9, status = CASE WHEN status = 'rejected' THEN 'draft' WHEN status = 'approved' THEN 'pending_review' ELSE status END, updated_at = NOW() WHERE id = $10 AND seller_id = $11 AND status <> 'archived'`, input.Slug, input.BrandSlug, input.CategorySlug, input.Name, input.Description, input.PriceCents, input.CompareAtCents, input.DeliveryLabel, input.ImageURL, productID, current.SellerID); err != nil {
				return mapCommerceError(err)
			}
		} else {
			if _, err := tx.Exec(transactionContext, `UPDATE products SET slug = $1, brand_slug = $2, category_slug = $3, name = $4, description = $5, price_cents = $6, compare_at_cents = $7, delivery_label = $8, status = CASE WHEN status = 'rejected' THEN 'draft' WHEN status = 'approved' THEN 'pending_review' ELSE status END, updated_at = NOW() WHERE id = $9 AND seller_id = $10 AND status <> 'archived'`, input.Slug, input.BrandSlug, input.CategorySlug, input.Name, input.Description, input.PriceCents, input.CompareAtCents, input.DeliveryLabel, productID, current.SellerID); err != nil {
				return mapCommerceError(err)
			}
		}
		if _, err := tx.Exec(transactionContext, `UPDATE product_variants SET sku = $1, price_cents = $2, compare_at_cents = $3, cost_price_cents = $4, updated_at = NOW() WHERE id = $5 AND product_id = $6 AND status = 'active'`, input.SKU, input.PriceCents, input.CompareAtCents, input.CostPriceCents, current.VariantID, productID); err != nil {
			return mapCommerceError(err)
		}
		metadata, _ := json.Marshal(map[string]string{"source": "seller", "previous_status": current.Status})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, request_id, metadata) VALUES ($1, 'catalog.product_updated', 'product', $2, $3, $4)`, fmt.Sprint(ownerUserID), fmt.Sprint(productID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		product = current
		product.Slug = input.Slug
		product.BrandSlug = input.BrandSlug
		product.BrandName = input.BrandName
		product.Name = input.Name
		product.Description = input.Description
		product.SKU = input.SKU
		product.PriceCents = input.PriceCents
		product.CompareAtCents = 0
		product.HasCompareAt = input.CompareAtCents != nil
		if input.CompareAtCents != nil {
			product.CompareAtCents = *input.CompareAtCents
		}
		// Stock is an operational record. Product writers must use the
		// permission-scoped inventory workflow rather than changing it while
		// editing catalogue copy or price.
		product.Stock = current.Stock
		product.Status = current.Status
		if current.Status == "rejected" {
			product.Status = "draft"
		} else if current.Status == "approved" {
			product.Status = "pending_review"
		}
		product.CategorySlug = input.CategorySlug
		product.CategoryName = input.CategoryName
		product.DeliveryLabel = input.DeliveryLabel
		payload, _ := json.Marshal(map[string]any{"product_id": productID, "status": product.Status})
		if _, err := tx.Exec(transactionContext, `INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload) VALUES ('ProductIndexRequested', 'Product', $1, $2)`, fmt.Sprint(productID), payload); err != nil {
			return err
		}
		return nil
	})
	return product, err
}

func (r *CommerceRepository) CreateAdminProduct(ctx context.Context, actorID, sellerID int64, input domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	var product domaincommerce.ManagedProduct
	err := WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		if err := requireActiveSuperAdmin(transactionContext, tx, actorID); err != nil {
			return err
		}
		var sellerName string
		if err := tx.QueryRow(transactionContext, `SELECT display_name FROM sellers WHERE id=$1 AND status != 'closed'`, sellerID).Scan(&sellerName); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrSellerNotFound
			}
			return err
		}
		_, _ = tx.Exec(transactionContext, `UPDATE sellers SET status='active', updated_at=NOW() WHERE id=$1 AND status != 'closed'`, sellerID)
		if err := ensureCatalogName(transactionContext, tx, "brands", input.BrandSlug, input.BrandName); err != nil {
			return err
		}
		if err := ensureCatalogName(transactionContext, tx, "categories", input.CategorySlug, input.CategoryName); err != nil {
			return err
		}
		imgURL := input.ImageURL
		if imgURL == "" {
			imgURL = "/assets/images/customer/saree-maroon.webp"
		}
		if err := tx.QueryRow(transactionContext, `INSERT INTO products (slug,brand_slug,category_slug,name,description,price_cents,compare_at_cents,delivery_label,image_url,seller_id,status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'draft') RETURNING id`, input.Slug, input.BrandSlug, input.CategorySlug, input.Name, input.Description, input.PriceCents, input.CompareAtCents, input.DeliveryLabel, imgURL, sellerID).Scan(&product.ID); err != nil {
			return mapCommerceError(err)
		}
		if err := tx.QueryRow(transactionContext, `INSERT INTO product_variants (product_id,sku,price_cents,compare_at_cents,cost_price_cents) VALUES ($1,$2,$3,$4,$5) RETURNING id`, product.ID, input.SKU, input.PriceCents, input.CompareAtCents, input.CostPriceCents).Scan(&product.VariantID); err != nil {
			return mapCommerceError(err)
		}
		if _, err := tx.Exec(transactionContext, `INSERT INTO inventory_stock (variant_id,available_quantity) VALUES ($1,$2)`, product.VariantID, input.InitialStock); err != nil {
			return err
		}
		product.SellerID = sellerID
		product.SellerName = sellerName
		product.Slug = input.Slug
		product.BrandSlug = input.BrandSlug
		product.BrandName = input.BrandName
		product.Name = input.Name
		product.Description = input.Description
		product.SKU = input.SKU
		product.PriceCents = input.PriceCents
		product.Stock = input.InitialStock
		product.Status = "draft"
		product.CategorySlug = input.CategorySlug
		product.CategoryName = input.CategoryName
		product.DeliveryLabel = input.DeliveryLabel
		product.HasCompareAt = input.CompareAtCents != nil
		if input.CompareAtCents != nil {
			product.CompareAtCents = *input.CompareAtCents
		}
		metadata, _ := json.Marshal(map[string]any{"source": "super_admin", "seller_id": sellerID})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id,action,resource_type,resource_id,request_id,metadata) VALUES ($1,'catalog.product_created','product',$2,$3,$4)`, fmt.Sprint(actorID), fmt.Sprint(product.ID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"product_id": product.ID, "status": "draft"})
		_, err := tx.Exec(transactionContext, `INSERT INTO outbox_events (event_type,aggregate_type,aggregate_id,payload) VALUES ('ProductIndexRequested','Product',$1,$2)`, fmt.Sprint(product.ID), payload)
		return err
	})
	return product, err
}

func (r *CommerceRepository) GetAdminProduct(ctx context.Context, actorID, productID int64) (domaincommerce.ManagedProduct, error) {
	var allowed bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug IN ('super_admin', 'marketplace_admin', 'seller_owner', 'seller_staff'))`, actorID).Scan(&allowed); err != nil {
		return domaincommerce.ManagedProduct{}, err
	}
	if !allowed {
		return domaincommerce.ManagedProduct{}, ports.ErrForbidden
	}
	product, err := r.getAdminProduct(ctx, r.pool, productID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaincommerce.ManagedProduct{}, ports.ErrNotFound
	}
	return product, err
}

func (r *CommerceRepository) UpdateAdminProduct(ctx context.Context, actorID, productID int64, input domaincommerce.ProductDraftInput) (domaincommerce.ManagedProduct, error) {
	var product domaincommerce.ManagedProduct
	err := WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		if err := requireActiveSuperAdmin(transactionContext, tx, actorID); err != nil {
			return err
		}
		var currentStatus string
		if err := tx.QueryRow(transactionContext, `SELECT status FROM products WHERE id=$1 FOR UPDATE`, productID).Scan(&currentStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrNotFound
			}
			return err
		}
		if currentStatus == "archived" {
			return ports.ErrInvalidState
		}
		current, err := r.getAdminProduct(transactionContext, tx, productID)
		if err != nil {
			return err
		}
		if err := ensureCatalogName(transactionContext, tx, "brands", input.BrandSlug, input.BrandName); err != nil {
			return err
		}
		if err := ensureCatalogName(transactionContext, tx, "categories", input.CategorySlug, input.CategoryName); err != nil {
			return err
		}
		if input.ImageURL != "" {
			if _, err := tx.Exec(transactionContext, `UPDATE products SET slug=$1,brand_slug=$2,category_slug=$3,name=$4,description=$5,price_cents=$6,compare_at_cents=$7,delivery_label=$8,image_url=$9,updated_at=NOW() WHERE id=$10 AND status<>'archived'`, input.Slug, input.BrandSlug, input.CategorySlug, input.Name, input.Description, input.PriceCents, input.CompareAtCents, input.DeliveryLabel, input.ImageURL, productID); err != nil {
				return mapCommerceError(err)
			}
			_, _ = tx.Exec(transactionContext, `UPDATE product_media SET public_url=$1 WHERE product_id=$2 AND status='ready'`, input.ImageURL, productID)
		} else {
			if _, err := tx.Exec(transactionContext, `UPDATE products SET slug=$1,brand_slug=$2,category_slug=$3,name=$4,description=$5,price_cents=$6,compare_at_cents=$7,delivery_label=$8,updated_at=NOW() WHERE id=$9 AND status<>'archived'`, input.Slug, input.BrandSlug, input.CategorySlug, input.Name, input.Description, input.PriceCents, input.CompareAtCents, input.DeliveryLabel, productID); err != nil {
				return mapCommerceError(err)
			}
		}
		if _, err := tx.Exec(transactionContext, `UPDATE product_variants SET sku=$1,price_cents=$2,compare_at_cents=$3,cost_price_cents=$4,updated_at=NOW() WHERE id=$5 AND product_id=$6 AND status='active'`, input.SKU, input.PriceCents, input.CompareAtCents, input.CostPriceCents, current.VariantID, productID); err != nil {
			return mapCommerceError(err)
		}
		metadata, _ := json.Marshal(map[string]any{"source": "super_admin", "previous_status": current.Status})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id,action,resource_type,resource_id,request_id,metadata) VALUES ($1,'catalog.product_updated','product',$2,$3,$4)`, fmt.Sprint(actorID), fmt.Sprint(productID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		product = current
		product.Slug, product.BrandSlug, product.BrandName = input.Slug, input.BrandSlug, input.BrandName
		product.CategorySlug, product.CategoryName = input.CategorySlug, input.CategoryName
		product.Name, product.Description, product.SKU = input.Name, input.Description, input.SKU
		product.PriceCents, product.DeliveryLabel = input.PriceCents, input.DeliveryLabel
		product.CompareAtCents, product.HasCompareAt = 0, input.CompareAtCents != nil
		if input.CompareAtCents != nil {
			product.CompareAtCents = *input.CompareAtCents
		}
		payload, _ := json.Marshal(map[string]any{"product_id": productID, "status": current.Status})
		_, err = tx.Exec(transactionContext, `INSERT INTO outbox_events (event_type,aggregate_type,aggregate_id,payload) VALUES ('ProductIndexRequested','Product',$1,$2)`, fmt.Sprint(productID), payload)
		return err
	})
	return product, err
}

func (r *CommerceRepository) ArchiveSellerProduct(ctx context.Context, ownerUserID, productID int64, reason string) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		current, err := r.getSellerProduct(transactionContext, tx, ownerUserID, productID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrForbidden
		}
		if err != nil {
			return err
		}
		if current.Status == "archived" {
			return ports.ErrInvalidState
		}
		result, err := tx.Exec(transactionContext, `UPDATE products SET status='archived',updated_at=NOW() WHERE id=$1 AND status<>'archived' AND seller_id=$2`, productID, current.SellerID)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrInvalidState
		}
		metadata, _ := json.Marshal(map[string]string{"source": "seller", "reason": reason})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id,action,resource_type,resource_id,request_id,metadata) VALUES ($1,'catalog.product_archived','product',$2,$3,$4)`, fmt.Sprint(ownerUserID), fmt.Sprint(productID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"product_id": productID, "status": "archived"})
		_, err = tx.Exec(transactionContext, `INSERT INTO outbox_events (event_type,aggregate_type,aggregate_id,payload) VALUES ('ProductIndexRequested','Product',$1,$2)`, fmt.Sprint(productID), payload)
		return err
	})
}

func (r *CommerceRepository) RestoreSellerProduct(ctx context.Context, ownerUserID, productID int64, reason string) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var sellerID int64
		err := tx.QueryRow(transactionContext, `SELECT s.id FROM products p JOIN sellers s ON s.id=p.seller_id WHERE p.id=$1 AND p.status='archived' AND s.status='active' AND ((s.owner_user_id=$2 AND EXISTS(SELECT 1 FROM users u WHERE u.id=$2 AND u.status='active')) OR EXISTS(SELECT 1 FROM seller_users su JOIN users u ON u.id=su.user_id AND u.status='active' WHERE su.seller_id=s.id AND su.user_id=$2 AND su.status='active' AND 'PRODUCT_WRITE'=ANY(su.permissions))) FOR UPDATE OF p`, productID, ownerUserID).Scan(&sellerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrForbidden
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(transactionContext, `UPDATE products SET status='draft',updated_at=NOW() WHERE id=$1`, productID); err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]string{"source": "seller", "reason": reason, "restored_to": "draft"})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'catalog.product_restored','product',$2,$3,$4)`, fmt.Sprint(ownerUserID), fmt.Sprint(productID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"product_id": productID, "status": "draft"})
		_, err = tx.Exec(transactionContext, `INSERT INTO outbox_events(event_type,aggregate_type,aggregate_id,payload) VALUES('ProductIndexRequested','Product',$1,$2)`, fmt.Sprint(productID), payload)
		return err
	})
}

func (r *CommerceRepository) ArchiveAdminProduct(ctx context.Context, actorID, productID int64, reason string) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		if err := requireActiveSuperAdmin(transactionContext, tx, actorID); err != nil {
			return err
		}
		var status string
		if err := tx.QueryRow(transactionContext, `SELECT status FROM products WHERE id=$1 FOR UPDATE`, productID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrNotFound
			}
			return err
		}
		if status == "archived" {
			return ports.ErrInvalidState
		}
		if _, err := tx.Exec(transactionContext, `UPDATE products SET status='archived',updated_at=NOW() WHERE id=$1`, productID); err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]string{"source": "super_admin", "reason": reason})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id,action,resource_type,resource_id,request_id,metadata) VALUES ($1,'catalog.product_archived','product',$2,$3,$4)`, fmt.Sprint(actorID), fmt.Sprint(productID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"product_id": productID, "status": "archived"})
		_, err := tx.Exec(transactionContext, `INSERT INTO outbox_events (event_type,aggregate_type,aggregate_id,payload) VALUES ('ProductIndexRequested','Product',$1,$2)`, fmt.Sprint(productID), payload)
		return err
	})
}

func (r *CommerceRepository) SubmitProduct(ctx context.Context, ownerUserID, productID int64) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(transactionContext, `SELECT p.status FROM products p JOIN sellers s ON s.id=p.seller_id WHERE p.id=$1 AND s.status='active' AND ((s.owner_user_id=$2 AND EXISTS(SELECT 1 FROM users u WHERE u.id=$2 AND u.status='active')) OR EXISTS(SELECT 1 FROM seller_users su JOIN users u ON u.id=su.user_id AND u.status='active' WHERE su.seller_id=s.id AND su.user_id=$2 AND su.status='active' AND 'PRODUCT_WRITE'=ANY(su.permissions))) FOR UPDATE OF p`, productID, ownerUserID).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrForbidden
		}
		if err != nil {
			return err
		}
		if status != "draft" && status != "rejected" {
			return ports.ErrInvalidState
		}
		if _, err := tx.Exec(transactionContext, `UPDATE products SET status='pending_review',updated_at=NOW() WHERE id=$1`, productID); err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]string{"source": "seller"})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'catalog.product_submitted','product',$2,$3,$4)`, fmt.Sprint(ownerUserID), fmt.Sprint(productID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"product_id": productID, "status": "pending_review"})
		_, err = tx.Exec(transactionContext, `INSERT INTO outbox_events(event_type,aggregate_type,aggregate_id,payload) VALUES('ProductIndexRequested','Product',$1,$2)`, fmt.Sprint(productID), payload)
		return err
	})
}

func (r *CommerceRepository) SubmitAdminProduct(ctx context.Context, actorID, productID int64) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		if err := requireActiveSuperAdmin(transactionContext, tx, actorID); err != nil {
			return err
		}
		var status string
		if err := tx.QueryRow(transactionContext, `SELECT status FROM products WHERE id=$1 FOR UPDATE`, productID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrNotFound
			}
			return err
		}
		if status != "draft" && status != "rejected" {
			return ports.ErrInvalidState
		}
		if _, err := tx.Exec(transactionContext, `UPDATE products SET status='pending_review',updated_at=NOW() WHERE id=$1`, productID); err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]string{"source": "super_admin"})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'catalog.product_submitted','product',$2,$3,$4)`, fmt.Sprint(actorID), fmt.Sprint(productID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"product_id": productID, "status": "pending_review"})
		_, err := tx.Exec(transactionContext, `INSERT INTO outbox_events(event_type,aggregate_type,aggregate_id,payload) VALUES('ProductIndexRequested','Product',$1,$2)`, fmt.Sprint(productID), payload)
		return err
	})
}

func (r *CommerceRepository) RestoreAdminProduct(ctx context.Context, actorID, productID int64, reason string) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		if err := requireActiveSuperAdmin(transactionContext, tx, actorID); err != nil {
			return err
		}
		result, err := tx.Exec(transactionContext, `UPDATE products SET status='draft',updated_at=NOW() WHERE id=$1 AND status='archived'`, productID)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrInvalidState
		}
		metadata, _ := json.Marshal(map[string]string{"source": "super_admin", "reason": reason, "restored_to": "draft"})
		if _, err := tx.Exec(transactionContext, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'catalog.product_restored','product',$2,$3,$4)`, fmt.Sprint(actorID), fmt.Sprint(productID), ports.RequestID(transactionContext), metadata); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"product_id": productID, "status": "draft"})
		_, err = tx.Exec(transactionContext, `INSERT INTO outbox_events(event_type,aggregate_type,aggregate_id,payload) VALUES('ProductIndexRequested','Product',$1,$2)`, fmt.Sprint(productID), payload)
		return err
	})
}

func requireActiveSuperAdmin(ctx context.Context, tx pgx.Tx, actorID int64) error {
	var allowed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug IN ('super_admin', 'marketplace_admin', 'seller_owner', 'seller_staff'))`, actorID).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return ports.ErrForbidden
	}
	return nil
}

func (r *CommerceRepository) ApproveProduct(ctx context.Context, actorID, productID int64, approved bool, reason string) error {
	status := "rejected"
	if approved {
		status = "approved"
	}
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var allowed bool
		if err := tx.QueryRow(transactionContext, `SELECT EXISTS (SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug IN ('marketplace_admin', 'super_admin'))`, actorID).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return ports.ErrForbidden
		}
		result, err := tx.Exec(transactionContext, `UPDATE products SET status = $1, updated_at = NOW() WHERE id = $2 AND status = 'pending_review'`, status, productID)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrInvalidState
		}
		metadata, _ := json.Marshal(map[string]string{"reason": reason, "approved": fmt.Sprint(approved)})
		_, err = tx.Exec(transactionContext, `INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, request_id, metadata) VALUES ($1, $2, 'product', $3, $4, $5)`, fmt.Sprint(actorID), "catalog.product_reviewed", fmt.Sprint(productID), ports.RequestID(transactionContext), metadata)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"product_id": productID, "status": status})
		_, err = tx.Exec(transactionContext, `INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload) VALUES ('ProductIndexRequested', 'Product', $1, $2)`, fmt.Sprint(productID), payload)
		return err
	})
}

func (r *CommerceRepository) ListSellerProducts(ctx context.Context, ownerUserID int64) ([]domaincommerce.ManagedProduct, error) {
	return r.listManagedProducts(ctx, `WHERE s.owner_user_id = $1 OR EXISTS (SELECT 1 FROM seller_users su WHERE su.seller_id = s.id AND su.user_id = $1 AND su.status = 'active')`, ownerUserID)
}

func (r *CommerceRepository) ListPendingProducts(ctx context.Context, actorID int64) ([]domaincommerce.ManagedProduct, error) {
	var allowed bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug IN ('marketplace_admin', 'super_admin'))`, actorID).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ports.ErrForbidden
	}
	return r.listManagedProducts(ctx, `WHERE p.status = 'pending_review'`)
}

func (r *CommerceRepository) listManagedProducts(ctx context.Context, filter string, args ...any) ([]domaincommerce.ManagedProduct, error) {
	query := `SELECT p.id, s.id, s.display_name, p.slug, p.brand_slug, b.name, p.name, p.description, pv.sku, pv.price_cents, COALESCE(pv.compare_at_cents, 0), pv.compare_at_cents IS NOT NULL, COALESCE(i.available_quantity, 0), p.status, p.category_slug, c.name, p.delivery_label, pv.id FROM products p JOIN sellers s ON s.id = p.seller_id JOIN brands b ON b.slug = p.brand_slug JOIN categories c ON c.slug = p.category_slug JOIN LATERAL (SELECT id,sku,price_cents,compare_at_cents FROM product_variants WHERE product_id=p.id AND status='active' ORDER BY id LIMIT 1) pv ON TRUE LEFT JOIN inventory_stock i ON i.variant_id = pv.id ` + filter + ` ORDER BY p.updated_at DESC, p.id DESC LIMIT 100`
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]domaincommerce.ManagedProduct, 0)
	for rows.Next() {
		var product domaincommerce.ManagedProduct
		if err := rows.Scan(&product.ID, &product.SellerID, &product.SellerName, &product.Slug, &product.BrandSlug, &product.BrandName, &product.Name, &product.Description, &product.SKU, &product.PriceCents, &product.CompareAtCents, &product.HasCompareAt, &product.Stock, &product.Status, &product.CategorySlug, &product.CategoryName, &product.DeliveryLabel, &product.VariantID); err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

type catalogProductQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *CommerceRepository) getSellerProduct(ctx context.Context, querier catalogProductQuerier, ownerUserID, productID int64) (domaincommerce.ManagedProduct, error) {
	var product domaincommerce.ManagedProduct
	err := querier.QueryRow(ctx, `SELECT p.id, s.id, s.display_name, p.slug, p.brand_slug, b.name, p.name, p.description, pv.sku, pv.price_cents, COALESCE(pv.compare_at_cents, 0), pv.compare_at_cents IS NOT NULL, COALESCE(i.available_quantity, 0), p.status, p.category_slug, c.name, p.delivery_label, pv.id FROM products p JOIN sellers s ON s.id = p.seller_id AND s.status = 'active' JOIN brands b ON b.slug = p.brand_slug JOIN categories c ON c.slug = p.category_slug JOIN product_variants pv ON pv.product_id = p.id AND pv.status = 'active' LEFT JOIN inventory_stock i ON i.variant_id = pv.id WHERE (s.owner_user_id = $1 OR EXISTS (SELECT 1 FROM seller_users su WHERE su.seller_id = s.id AND su.user_id = $1 AND su.status = 'active')) AND p.id = $2 ORDER BY pv.id LIMIT 1`, ownerUserID, productID).Scan(&product.ID, &product.SellerID, &product.SellerName, &product.Slug, &product.BrandSlug, &product.BrandName, &product.Name, &product.Description, &product.SKU, &product.PriceCents, &product.CompareAtCents, &product.HasCompareAt, &product.Stock, &product.Status, &product.CategorySlug, &product.CategoryName, &product.DeliveryLabel, &product.VariantID)
	return product, err
}

func (r *CommerceRepository) getAdminProduct(ctx context.Context, querier catalogProductQuerier, productID int64) (domaincommerce.ManagedProduct, error) {
	var product domaincommerce.ManagedProduct
	err := querier.QueryRow(ctx, `SELECT p.id,s.id,s.display_name,p.slug,p.brand_slug,b.name,p.name,p.description,pv.sku,pv.price_cents,COALESCE(pv.compare_at_cents,0),pv.compare_at_cents IS NOT NULL,COALESCE(i.available_quantity,0),p.status,p.category_slug,c.name,p.delivery_label,pv.id FROM products p LEFT JOIN sellers s ON s.id=p.seller_id JOIN brands b ON b.slug=p.brand_slug JOIN categories c ON c.slug=p.category_slug LEFT JOIN LATERAL (SELECT id,sku,price_cents,compare_at_cents FROM product_variants WHERE product_id=p.id AND status='active' ORDER BY id LIMIT 1) pv ON TRUE LEFT JOIN inventory_stock i ON i.variant_id=pv.id WHERE p.id=$1`, productID).Scan(&product.ID, &product.SellerID, &product.SellerName, &product.Slug, &product.BrandSlug, &product.BrandName, &product.Name, &product.Description, &product.SKU, &product.PriceCents, &product.CompareAtCents, &product.HasCompareAt, &product.Stock, &product.Status, &product.CategorySlug, &product.CategoryName, &product.DeliveryLabel, &product.VariantID)
	return product, err
}

func (r *CommerceRepository) GetOrCreateCart(ctx context.Context, userID int64, guestTokenHash string) (domaincommerce.Cart, error) {
	if userID > 0 {
		guestTokenHash = ""
	}
	if _, err := r.pool.Exec(ctx, `UPDATE carts SET status='expired',updated_at=NOW() WHERE status='active' AND expires_at<=NOW() AND (($1>0 AND user_id=$1) OR ($2<>'' AND guest_token_hash=$2))`, userID, guestTokenHash); err != nil {
		return domaincommerce.Cart{}, err
	}
	var cartID int64
	query := `SELECT id FROM carts WHERE status = 'active' AND expires_at > NOW() AND (($1 > 0 AND user_id = $1) OR ($2 <> '' AND guest_token_hash = $2)) LIMIT 1`
	err := r.pool.QueryRow(ctx, query, userID, guestTokenHash).Scan(&cartID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = r.pool.QueryRow(ctx, `INSERT INTO carts (user_id, guest_token_hash, expires_at) VALUES (NULLIF($1, 0), NULLIF($2, ''), NOW() + INTERVAL '30 days') ON CONFLICT DO NOTHING RETURNING id`, userID, guestTokenHash).Scan(&cartID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = r.pool.QueryRow(ctx, query, userID, guestTokenHash).Scan(&cartID)
		}
	}
	if err != nil {
		return domaincommerce.Cart{}, mapCommerceError(err)
	}
	return r.GetCart(ctx, cartID)
}

func (r *CommerceRepository) AddCartItem(ctx context.Context, cartID, variantID int64, quantity int) error {
	if err := r.assertAvailableVariant(ctx, variantID); err != nil {
		return err
	}
	result, err := r.pool.Exec(ctx, `INSERT INTO cart_items (cart_id, variant_id, quantity) VALUES ($1, $2, $3) ON CONFLICT (cart_id, variant_id) DO UPDATE SET quantity = LEAST(99, cart_items.quantity + EXCLUDED.quantity), updated_at = NOW() WHERE cart_items.cart_id IN (SELECT id FROM carts WHERE status = 'active' AND expires_at > NOW())`, cartID, variantID, quantity)
	if err != nil {
		return mapCommerceError(err)
	}
	if result.RowsAffected() != 1 {
		return ports.ErrCartNotFound
	}
	return nil
}

func (r *CommerceRepository) UpdateCartItem(ctx context.Context, cartID, variantID int64, quantity int) error {
	result, err := r.pool.Exec(ctx, `UPDATE cart_items SET quantity = $3, updated_at = NOW() WHERE cart_id = $1 AND variant_id = $2 AND EXISTS (SELECT 1 FROM carts WHERE id = $1 AND status = 'active' AND expires_at > NOW())`, cartID, variantID, quantity)
	if err != nil {
		return mapCommerceError(err)
	}
	if result.RowsAffected() != 1 {
		return ports.ErrCartNotFound
	}
	return nil
}

func (r *CommerceRepository) RemoveCartItem(ctx context.Context, cartID, variantID int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM cart_items WHERE cart_id = $1 AND variant_id = $2`, cartID, variantID)
	return err
}

func (r *CommerceRepository) MoveCartItemToWishlist(ctx context.Context, cartID, userID, variantID, productID int64) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var cartProductID int64
		err := tx.QueryRow(transactionContext, `
			SELECT p.id
			FROM carts c
			JOIN cart_items ci ON ci.cart_id = c.id AND ci.variant_id = $3
			JOIN product_variants pv ON pv.id = ci.variant_id AND pv.status = 'active'
			JOIN products p ON p.id = pv.product_id AND p.status = 'approved'
			JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
			WHERE c.id = $1 AND c.user_id = $2 AND c.status = 'active' AND c.expires_at > NOW()
			FOR UPDATE OF c, ci`, cartID, userID, variantID).Scan(&cartProductID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrCartNotFound
			}
			return err
		}
		if cartProductID != productID {
			return ports.ErrCartNotFound
		}
		if _, err := tx.Exec(transactionContext, `INSERT INTO wishlist_items (user_id, product_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, productID); err != nil {
			return err
		}
		result, err := tx.Exec(transactionContext, `DELETE FROM cart_items WHERE cart_id = $1 AND variant_id = $2`, cartID, variantID)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrCartNotFound
		}
		return nil
	})
}

func (r *CommerceRepository) MergeGuestCart(ctx context.Context, userID int64, guestTokenHash string) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var guestID, userCartID int64
		if err := tx.QueryRow(transactionContext, `SELECT id FROM carts WHERE guest_token_hash = $1 AND status = 'active' AND expires_at > NOW() FOR UPDATE`, guestTokenHash).Scan(&guestID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if err := tx.QueryRow(transactionContext, `SELECT id FROM carts WHERE user_id = $1 AND status = 'active' AND expires_at > NOW() FOR UPDATE`, userID).Scan(&userCartID); errors.Is(err, pgx.ErrNoRows) {
			if err := tx.QueryRow(transactionContext, `INSERT INTO carts (user_id, expires_at) VALUES ($1, NOW() + INTERVAL '30 days') RETURNING id`, userID).Scan(&userCartID); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if _, err := tx.Exec(transactionContext, `INSERT INTO cart_items (cart_id, variant_id, quantity)
			SELECT $1, ci.variant_id, ci.quantity
			FROM cart_items ci
			JOIN product_variants pv ON pv.id = ci.variant_id AND pv.status = 'active'
			JOIN products p ON p.id = pv.product_id AND p.status = 'approved'
			JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
			JOIN inventory_stock i ON i.variant_id = pv.id
			WHERE ci.cart_id = $2
			ON CONFLICT (cart_id, variant_id) DO UPDATE SET quantity = LEAST(99, cart_items.quantity + EXCLUDED.quantity), updated_at = NOW()`, userCartID, guestID); err != nil {
			return err
		}
		_, err := tx.Exec(transactionContext, `UPDATE carts SET status = 'expired', updated_at = NOW() WHERE id = $1`, guestID)
		return err
	})
}

func (r *CommerceRepository) GetCart(ctx context.Context, cartID int64) (domaincommerce.Cart, error) {
	var cart domaincommerce.Cart
	if err := r.pool.QueryRow(ctx, `SELECT id, expires_at FROM carts WHERE id = $1 AND status = 'active' AND expires_at > NOW()`, cartID).Scan(&cart.ID, &cart.ExpiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domaincommerce.Cart{}, ports.ErrCartNotFound
		}
		return domaincommerce.Cart{}, err
	}
	rows, err := r.pool.Query(ctx, `SELECT ci.variant_id, p.id, p.slug, p.name, pv.name, pv.sku, ci.quantity, pv.price_cents, i.available_quantity FROM cart_items ci JOIN product_variants pv ON pv.id = ci.variant_id AND pv.status = 'active' JOIN products p ON p.id = pv.product_id AND p.status = 'approved' JOIN inventory_stock i ON i.variant_id = pv.id WHERE ci.cart_id = $1 ORDER BY ci.created_at, ci.variant_id`, cartID)
	if err != nil {
		return domaincommerce.Cart{}, err
	}
	defer rows.Close()
	cart.Items = make([]domaincommerce.CartItem, 0)
	for rows.Next() {
		var item domaincommerce.CartItem
		if err := rows.Scan(&item.VariantID, &item.ProductID, &item.ProductSlug, &item.ProductName, &item.VariantName, &item.SKU, &item.Quantity, &item.UnitPriceCents, &item.Available); err != nil {
			return domaincommerce.Cart{}, err
		}
		item.LineTotalCents = item.UnitPriceCents * int64(item.Quantity)
		cart.SubtotalCents += item.LineTotalCents
		cart.ItemCount += item.Quantity
		cart.Items = append(cart.Items, item)
	}
	return cart, rows.Err()
}

func (r *CommerceRepository) AddWishlist(ctx context.Context, userID, productID int64) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products p JOIN sellers s ON s.id = p.seller_id AND s.status = 'active' WHERE p.id = $1 AND p.status = 'approved')`, productID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ports.ErrNotFound
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO wishlist_items (user_id, product_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, productID)
	if err != nil {
		return err
	}
	return nil
}

func (r *CommerceRepository) RemoveWishlist(ctx context.Context, userID, productID int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM wishlist_items WHERE user_id = $1 AND product_id = $2`, userID, productID)
	return err
}

func (r *CommerceRepository) ListWishlist(ctx context.Context, userID int64) ([]domaincommerce.WishlistItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT wi.product_id, p.slug, p.name, b.name, p.price_cents,
		       COALESCE(i.available_quantity, 0)
		FROM wishlist_items wi
		JOIN products p ON p.id = wi.product_id AND p.status = 'approved'
		JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
		JOIN brands b ON b.slug = p.brand_slug
		LEFT JOIN LATERAL (
			SELECT pv.id
			FROM product_variants pv
			WHERE pv.product_id = p.id AND pv.status = 'active'
			ORDER BY pv.id
			LIMIT 1
		) pv ON TRUE
		LEFT JOIN inventory_stock i ON i.variant_id = pv.id
		WHERE wi.user_id = $1
		ORDER BY wi.created_at DESC, wi.product_id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domaincommerce.WishlistItem, 0)
	for rows.Next() {
		var item domaincommerce.WishlistItem
		if err := rows.Scan(&item.ProductID, &item.Slug, &item.Name, &item.Brand, &item.PriceCents, &item.Available); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *CommerceRepository) CreateOrder(ctx context.Context, userID, cartID int64, idempotencyKey string, address domaincommerce.AddressInput) (domaincommerce.Order, error) {
	return r.createOrder(ctx, userID, cartID, idempotencyKey, address, "", nil)
}

func (r *CommerceRepository) CreateOrderWithCoupon(ctx context.Context, userID, cartID int64, idempotencyKey string, address domaincommerce.AddressInput, couponCode string) (domaincommerce.Order, error) {
	return r.createOrder(ctx, userID, cartID, idempotencyKey, address, strings.ToUpper(strings.TrimSpace(couponCode)), nil)
}

func (r *CommerceRepository) CreateQuotedOrder(ctx context.Context, userID, cartID int64, key string, address domaincommerce.AddressInput, coupon string, expected int64) (domaincommerce.Order, error) {
	return r.createOrder(ctx, userID, cartID, key, address, strings.ToUpper(strings.TrimSpace(coupon)), &expected)
}

func (r *CommerceRepository) createOrder(ctx context.Context, userID, cartID int64, idempotencyKey string, address domaincommerce.AddressInput, couponCode string, expected *int64) (domaincommerce.Order, error) {
	var order domaincommerce.Order
	addressJSON, err := json.Marshal(address)
	if err != nil {
		return order, err
	}
	requestHash := checkoutRequestHash(cartID, addressJSON, couponCode)
	err = WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var existingHash *string
		if err := tx.QueryRow(transactionContext, `SELECT id, order_number, status, currency, subtotal_cents, shipping_cents, discount_cents, total_cents, created_at, idempotency_request_hash FROM orders WHERE idempotency_scope = $1 AND idempotency_key = $2`, fmt.Sprintf("checkout:%d", userID), idempotencyKey).Scan(&order.ID, &order.OrderNumber, &order.Status, &order.Currency, &order.SubtotalCents, &order.ShippingCents, &order.DiscountCents, &order.TotalCents, &order.CreatedAt, &existingHash); err == nil {
			if existingHash != nil && *existingHash != requestHash {
				return ports.ErrIdempotencyConflict
			}
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var cartOwner int64
		cartErr := tx.QueryRow(transactionContext, `SELECT COALESCE(user_id,0) FROM carts WHERE id = $1 AND status = 'active' AND expires_at > NOW() FOR UPDATE`, cartID).Scan(&cartOwner)
		if cartErr != nil && !errors.Is(cartErr, pgx.ErrNoRows) {
			return cartErr
		}
		if cartErr != nil || cartOwner != userID {
			// A concurrent identical checkout may have completed while this
			// request waited on the cart lock. Return its durable result only.
			var existingHash *string
			if err := tx.QueryRow(transactionContext, `SELECT id, order_number, status, currency, subtotal_cents, shipping_cents, discount_cents, total_cents, created_at, idempotency_request_hash FROM orders WHERE idempotency_scope = $1 AND idempotency_key = $2`, fmt.Sprintf("checkout:%d", userID), idempotencyKey).Scan(&order.ID, &order.OrderNumber, &order.Status, &order.Currency, &order.SubtotalCents, &order.ShippingCents, &order.DiscountCents, &order.TotalCents, &order.CreatedAt, &existingHash); err == nil {
				if existingHash != nil && *existingHash != requestHash {
					return ports.ErrIdempotencyConflict
				}
				return nil
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			return ports.ErrCartNotFound
		}
		var cartItemCount int
		if err := tx.QueryRow(transactionContext, `SELECT COUNT(*) FROM cart_items WHERE cart_id=$1`, cartID).Scan(&cartItemCount); err != nil {
			return err
		}
		rows, err := tx.Query(transactionContext, `SELECT ci.variant_id, p.id, p.name, pv.sku, pv.price_cents, ci.quantity, COALESCE(i.available_quantity, 0), p.status, pv.status, COALESCE(s.status, 'unavailable') FROM cart_items ci JOIN product_variants pv ON pv.id = ci.variant_id JOIN products p ON p.id = pv.product_id LEFT JOIN sellers s ON s.id = p.seller_id LEFT JOIN inventory_stock i ON i.variant_id = pv.id WHERE ci.cart_id = $1 ORDER BY ci.variant_id FOR UPDATE OF ci`, cartID)
		if err != nil {
			return err
		}
		defer rows.Close()
		type line struct {
			variantID                                  int64
			productID                                  int64
			name, sku                                  string
			price                                      int64
			quantity, available                        int
			productStatus, variantStatus, sellerStatus string
		}
		lines := make([]line, 0)
		var subtotal int64
		for rows.Next() {
			var item line
			if err := rows.Scan(&item.variantID, &item.productID, &item.name, &item.sku, &item.price, &item.quantity, &item.available, &item.productStatus, &item.variantStatus, &item.sellerStatus); err != nil {
				return err
			}
			if item.productStatus != "approved" {
				return ports.ErrInvalidState
			}
			if item.variantStatus != "active" {
				return ports.ErrInvalidState
			}
			if item.sellerStatus != "active" {
				return ports.ErrInvalidState
			}
			if item.quantity > item.available {
				return ports.ErrInsufficientStock
			}
			subtotal += item.price * int64(item.quantity)
			lines = append(lines, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(lines) == 0 {
			return ports.ErrCartNotFound
		}
		if len(lines) != cartItemCount {
			return ports.ErrInvalidState
		}
		orderNumber := newOrderNumber(userID)
		shipping := domaincommerce.CheckoutShipping(subtotal)
		couponID, promotionID, discount, err := checkoutDiscount(transactionContext, tx, userID, cartID, shipping, couponCode, true)
		if err != nil {
			return err
		}
		total := subtotal + shipping - discount
		if expected != nil && *expected != total {
			return ports.ErrQuoteChanged
		}

		err = tx.QueryRow(transactionContext, `INSERT INTO orders (order_number, user_id, status, subtotal_cents, shipping_cents, discount_cents, total_cents, address_snapshot, idempotency_scope, idempotency_key, idempotency_request_hash) VALUES ($1, $2, 'pending_payment', $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT (idempotency_scope, idempotency_key) DO NOTHING RETURNING id, order_number, status, currency, subtotal_cents, shipping_cents, discount_cents, total_cents, created_at`, orderNumber, userID, subtotal, shipping, discount, total, addressJSON, fmt.Sprintf("checkout:%d", userID), idempotencyKey, requestHash).Scan(&order.ID, &order.OrderNumber, &order.Status, &order.Currency, &order.SubtotalCents, &order.ShippingCents, &order.DiscountCents, &order.TotalCents, &order.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			var existingHash *string
			if selectErr := tx.QueryRow(transactionContext, `SELECT id, order_number, status, currency, subtotal_cents, shipping_cents, discount_cents, total_cents, created_at, idempotency_request_hash FROM orders WHERE idempotency_scope = $1 AND idempotency_key = $2`, fmt.Sprintf("checkout:%d", userID), idempotencyKey).Scan(&order.ID, &order.OrderNumber, &order.Status, &order.Currency, &order.SubtotalCents, &order.ShippingCents, &order.DiscountCents, &order.TotalCents, &order.CreatedAt, &existingHash); selectErr != nil {
				return selectErr
			} else if existingHash != nil && *existingHash != requestHash {
				return ports.ErrIdempotencyConflict
			}
			return nil
		}
		if err != nil {
			return mapCommerceError(err)
		}
		if couponID != 0 {
			if _, err := tx.Exec(transactionContext, `INSERT INTO promotion_usages (promotion_id,coupon_id,user_id,order_id,amount_cents) VALUES ($1,$2,$3,$4,$5)`, promotionID, couponID, userID, order.ID, discount); err != nil {
				return err
			}
			if _, err := tx.Exec(transactionContext, `UPDATE coupons SET usage_count=usage_count+1 WHERE id=$1`, couponID); err != nil {
				return err
			}
			if _, err := tx.Exec(transactionContext, `UPDATE promotions SET usage_count=usage_count+1,updated_at=NOW() WHERE id=$1`, promotionID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(transactionContext, `
			INSERT INTO addresses (user_id, recipient_name, phone, line1, line2, city, state, postal_code, country_code)
			SELECT $1::bigint, $2::text, $3::text, $4::text, $5::text, $6::text, $7::text, $8::text, $9::text
			WHERE NOT EXISTS (
				SELECT 1 FROM addresses
				WHERE user_id = $1::bigint AND recipient_name = $2::text AND phone = $3::text AND line1 = $4::text AND line2 = $5::text
				  AND city = $6::text AND state = $7::text AND postal_code = $8::text AND country_code = $9::text
			)`, userID, address.RecipientName, address.Phone, address.Line1, address.Line2, address.City, address.State, address.PostalCode, address.CountryCode); err != nil {
			return err
		}
		var reservationID int64
		if err := tx.QueryRow(transactionContext, `INSERT INTO inventory_reservations (order_id, expires_at) VALUES ($1, NOW() + INTERVAL '30 minutes') RETURNING id`, order.ID).Scan(&reservationID); err != nil {
			return err
		}
		for _, item := range lines {
			if _, err := tx.Exec(transactionContext, `INSERT INTO order_items (order_id, variant_id, product_name, sku, unit_price_cents, quantity, line_total_cents) VALUES ($1, $2, $3, $4, $5, $6, $7)`, order.ID, item.variantID, item.name, item.sku, item.price, item.quantity, item.price*int64(item.quantity)); err != nil {
				return err
			}
			if _, err := tx.Exec(transactionContext, `INSERT INTO inventory_reservation_items (reservation_id, variant_id, quantity) VALUES ($1, $2, $3)`, reservationID, item.variantID, item.quantity); err != nil {
				return err
			}
			result, err := tx.Exec(transactionContext, `UPDATE inventory_stock
				SET available_quantity = available_quantity - $2,
				reserved_quantity = reserved_quantity + $2, updated_at = NOW()
				WHERE variant_id = $1 AND available_quantity >= $2`, item.variantID, item.quantity)
			if err != nil {
				return err
			}
			if result.RowsAffected() != 1 {
				return ports.ErrInsufficientStock
			}
		}
		if _, err := tx.Exec(transactionContext, `INSERT INTO payment_attempts (order_id, provider, status, amount_cents) VALUES ($1, 'razorpay', 'created', $2)`, order.ID, order.TotalCents); err != nil {
			return err
		}
		if _, err := tx.Exec(transactionContext, `INSERT INTO order_status_history (order_id, to_status, actor_id, reason) VALUES ($1, 'pending_payment', $2, 'checkout created')`, order.ID, userID); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"order_id": order.ID, "order_number": order.OrderNumber, "status": order.Status})
		_, err = tx.Exec(transactionContext, `INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload) VALUES ('OrderCreated', 'Order', $1, $2)`, order.OrderNumber, payload)
		if err != nil {
			return err
		}
		_, err = tx.Exec(transactionContext, `UPDATE carts SET status = 'checked_out', updated_at = NOW() WHERE id = $1`, cartID)
		return err
	})
	return order, err
}

func (r *CommerceRepository) ListOrders(ctx context.Context, userID int64, limit int) ([]domaincommerce.Order, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, order_number, status, currency, subtotal_cents, shipping_cents, discount_cents, total_cents, created_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]domaincommerce.Order, 0)
	for rows.Next() {
		var order domaincommerce.Order
		if err := rows.Scan(&order.ID, &order.OrderNumber, &order.Status, &order.Currency, &order.SubtotalCents, &order.ShippingCents, &order.DiscountCents, &order.TotalCents, &order.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (r *CommerceRepository) GetOrder(ctx context.Context, userID int64, orderNumber string) (domaincommerce.OrderDetail, error) {
	var detail domaincommerce.OrderDetail
	if err := r.pool.QueryRow(ctx, `
		SELECT o.id, o.order_number, o.status, o.currency, o.subtotal_cents,
		       o.shipping_cents, o.discount_cents, o.total_cents, o.created_at,
		       COALESCE((SELECT pa.status FROM payment_attempts pa WHERE pa.order_id = o.id ORDER BY pa.id DESC LIMIT 1), '')
		FROM orders o
		WHERE o.user_id = $1 AND o.order_number = $2`, userID, orderNumber).Scan(&detail.ID, &detail.OrderNumber, &detail.Status, &detail.Currency, &detail.SubtotalCents, &detail.ShippingCents, &detail.DiscountCents, &detail.TotalCents, &detail.CreatedAt, &detail.PaymentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domaincommerce.OrderDetail{}, ports.ErrOrderNotFound
		}
		return domaincommerce.OrderDetail{}, err
	}
	rows, err := r.pool.Query(ctx, `SELECT product_name, sku, quantity, unit_price_cents, line_total_cents FROM order_items WHERE order_id = $1 ORDER BY id`, detail.ID)
	if err != nil {
		return domaincommerce.OrderDetail{}, err
	}
	defer rows.Close()
	detail.Items = make([]domaincommerce.OrderItem, 0)
	for rows.Next() {
		var item domaincommerce.OrderItem
		if err := rows.Scan(&item.ProductName, &item.SKU, &item.Quantity, &item.UnitPriceCents, &item.LineTotalCents); err != nil {
			return domaincommerce.OrderDetail{}, err
		}
		detail.Items = append(detail.Items, item)
	}
	if err := rows.Err(); err != nil {
		return domaincommerce.OrderDetail{}, err
	}
	fulfillmentRows, err := r.pool.Query(ctx, `
		SELECT s.display_name, sf.status, sf.carrier, sf.tracking_number,
		       sf.last_note, sf.updated_at
		FROM seller_fulfillments sf
		JOIN sellers s ON s.id = sf.seller_id
		WHERE sf.order_id = $1
		ORDER BY sf.updated_at DESC, sf.id DESC`, detail.ID)
	if err != nil {
		return domaincommerce.OrderDetail{}, err
	}
	defer fulfillmentRows.Close()
	detail.Fulfillments = make([]domaincommerce.OrderFulfillment, 0)
	for fulfillmentRows.Next() {
		var fulfillment domaincommerce.OrderFulfillment
		if err := fulfillmentRows.Scan(&fulfillment.SellerName, &fulfillment.Status, &fulfillment.Carrier, &fulfillment.TrackingNumber, &fulfillment.LastNote, &fulfillment.UpdatedAt); err != nil {
			return domaincommerce.OrderDetail{}, err
		}
		detail.Fulfillments = append(detail.Fulfillments, fulfillment)
	}
	return detail, fulfillmentRows.Err()
}

func (r *CommerceRepository) CancelOrder(ctx context.Context, userID int64, orderNumber string) error {
	return WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		var orderID, reservationID int64
		var status string
		if err := tx.QueryRow(transactionContext, `SELECT id, status FROM orders WHERE user_id = $1 AND order_number = $2 FOR UPDATE`, userID, orderNumber).Scan(&orderID, &status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ports.ErrOrderNotFound
			}
			return err
		}
		if status != "pending_payment" {
			return ports.ErrInvalidState
		}
		if err := tx.QueryRow(transactionContext, `SELECT id FROM inventory_reservations WHERE order_id = $1 AND status = 'active' FOR UPDATE`, orderID).Scan(&reservationID); err != nil {
			return err
		}
		if _, err := releaseReservationWithStatus(transactionContext, tx, reservationID, "released"); err != nil {
			return err
		}
		if _, err := tx.Exec(transactionContext, `UPDATE orders SET status = 'cancelled', updated_at = NOW() WHERE id = $1`, orderID); err != nil {
			return err
		}
		_, err := tx.Exec(transactionContext, `INSERT INTO order_status_history (order_id, from_status, to_status, actor_id, reason) VALUES ($1, 'pending_payment', 'cancelled', $2, 'customer cancelled before payment')`, orderID, userID)
		return err
	})
}

func (r *CommerceRepository) ReleaseExpiredReservations(ctx context.Context, limit int) (released int, err error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	err = WithinTransaction(ctx, r.pool, func(transactionContext context.Context, tx pgx.Tx) error {
		rows, queryErr := tx.Query(transactionContext, `
			SELECT id, order_id
			FROM inventory_reservations
			WHERE status = 'active' AND expires_at <= NOW()
			ORDER BY id
			FOR UPDATE SKIP LOCKED
			LIMIT $1`, limit)
		if queryErr != nil {
			return queryErr
		}
		type expiredReservation struct {
			id      int64
			orderID int64
		}
		reservations := make([]expiredReservation, 0, limit)
		for rows.Next() {
			var reservationID, orderID int64
			if scanErr := rows.Scan(&reservationID, &orderID); scanErr != nil {
				rows.Close()
				return scanErr
			}
			reservations = append(reservations, expiredReservation{id: reservationID, orderID: orderID})
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return rowsErr
		}
		rows.Close()
		for _, reservation := range reservations {
			if _, releaseErr := releaseReservationWithStatus(transactionContext, tx, reservation.id, "expired"); releaseErr != nil {
				return releaseErr
			}
			if _, updateErr := tx.Exec(transactionContext, `UPDATE orders SET status = 'payment_failed', updated_at = NOW() WHERE id = $1 AND status = 'pending_payment'`, reservation.orderID); updateErr != nil {
				return updateErr
			}
			if _, historyErr := tx.Exec(transactionContext, `INSERT INTO order_status_history (order_id, from_status, to_status, reason) VALUES ($1, 'pending_payment', 'payment_failed', 'payment reservation expired')`, reservation.orderID); historyErr != nil {
				return historyErr
			}
			if _, paymentErr := tx.Exec(transactionContext, `UPDATE payment_attempts SET status = 'failed', failure_code = 'reservation_expired', updated_at = NOW() WHERE order_id = $1 AND status IN ('created', 'pending')`, reservation.orderID); paymentErr != nil {
				return paymentErr
			}
			released++
		}
		return nil
	})
	return released, err
}

func (r *CommerceRepository) assertAvailableVariant(ctx context.Context, variantID int64) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM product_variants pv JOIN products p ON p.id = pv.product_id AND p.status = 'approved' JOIN sellers s ON s.id = p.seller_id AND s.status = 'active' JOIN inventory_stock i ON i.variant_id = pv.id WHERE pv.id = $1 AND pv.status = 'active')`, variantID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ports.ErrNotFound
	}
	return nil
}

func mapCommerceError(err error) error {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return fmt.Errorf("duplicate commerce record: %w", err)
	}
	return err
}

func hashGuestToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func checkoutRequestHash(cartID int64, addressJSON []byte, couponCode string) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:", cartID, strings.ToUpper(strings.TrimSpace(couponCode))) + string(addressJSON)))
	return hex.EncodeToString(digest[:])
}

func newOrderNumber(userID int64) string {
	return fmt.Sprintf("WC-%s-%d", time.Now().UTC().Format("20060102-150405.000000000"), userID)
}

func normalizeGuestToken(token string) string {
	return strings.TrimSpace(token)
}
