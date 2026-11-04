package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *ManagementRepository) listManagedCategories(ctx context.Context, query string, limit, offset int) (ports.ManagementResult, error) {
	result := ports.ManagementResult{Records: []ports.ManagementRecord{}}
	filter := ` FROM categories c WHERE $1='' OR c.name ILIKE '%'||$1||'%' OR c.slug ILIKE '%'||$1||'%'`
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*)`+filter, query).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := r.pool.Query(ctx, `SELECT c.slug,c.name,c.image_url,c.image_object_key,c.display_order,c.show_in_navigation,(SELECT COUNT(*) FROM product_categories pc WHERE pc.category_slug=c.slug)`+filter+` ORDER BY c.display_order,c.slug LIMIT $2 OFFSET $3`, query, limit, offset)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var c ports.ManagementRecord
		if err := rows.Scan(&c.ID, &c.Name, &c.ImageURL, &c.ImageObjectKey, &c.DisplayOrder, &c.ShowInNavigation, &c.Products); err != nil {
			return result, err
		}
		result.Records = append(result.Records, c)
	}
	return result, rows.Err()
}

func (r *ProductSearchRepository) CatalogRevision(ctx context.Context) (int64, error) {
	var revision int64
	err := r.pool.QueryRow(ctx, `SELECT revision FROM catalog_revision WHERE singleton`).Scan(&revision)
	return revision, err
}

func (r *ManagementRepository) CategoryProducts(ctx context.Context, actorID int64, slug, query string, page int) ([]ports.CategoryProduct, int, error) {
	if err := managementAuthorize(ctx, r.pool, actorID, "category-management"); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	filter := ` FROM products p JOIN sellers s ON s.id=p.seller_id WHERE ($1='' OR p.name ILIKE '%'||$1||'%' OR p.slug ILIKE '%'||$1||'%' OR EXISTS(SELECT 1 FROM product_variants v WHERE v.product_id=p.id AND v.sku ILIKE '%'||$1||'%'))`
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*)`+filter, query).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT p.id,p.name,s.display_name,p.category_slug,EXISTS(SELECT 1 FROM product_categories pc WHERE pc.product_id=p.id AND pc.category_slug=$2)`+filter+` ORDER BY p.id DESC LIMIT 25 OFFSET $3`, query, slug, (page-1)*25)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []ports.CategoryProduct{}
	for rows.Next() {
		var p ports.CategoryProduct
		if err := rows.Scan(&p.ID, &p.Name, &p.Seller, &p.PrimaryCategory, &p.Assigned); err != nil {
			return nil, 0, err
		}
		result = append(result, p)
	}
	return result, total, rows.Err()
}

func (r *ManagementRepository) AssignCategoryProducts(ctx context.Context, actorID int64, slug, intent, reason string, ids []int64) error {
	if len(ids) == 0 || len(ids) > 25 || len(reason) < 4 || len(reason) > 1000 || (intent != "add" && intent != "remove") {
		return ports.ErrManagementValidation
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return ports.ErrManagementValidation
		}
		seen[id] = true
	}
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		if err := requireCompanyCatalogActor(ctx, tx, actorID); err != nil {
			return err
		}
		var category string
		if err := tx.QueryRow(ctx, `SELECT slug FROM categories WHERE slug=$1 FOR KEY SHARE`, slug).Scan(&category); errors.Is(err, pgx.ErrNoRows) {
			return ports.ErrNotFound
		} else if err != nil {
			return err
		}
		// Lock in stable order and validate the complete selection before changing it.
		rows, err := tx.Query(ctx, `SELECT id,category_slug FROM products WHERE id=ANY($1::bigint[]) ORDER BY id FOR UPDATE`, ids)
		if err != nil {
			return err
		}
		found := 0
		primary := false
		for rows.Next() {
			var id int64
			var category string
			if err := rows.Scan(&id, &category); err != nil {
				rows.Close()
				return err
			}
			found++
			primary = primary || category == slug
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if found != len(ids) {
			return ports.ErrNotFound
		}
		if intent == "remove" && primary {
			return ports.ErrManagementConflict
		}
		if intent == "add" {
			_, err = tx.Exec(ctx, `INSERT INTO product_categories(product_id,category_slug) SELECT unnest($1::bigint[]),$2 ON CONFLICT DO NOTHING`, ids, slug)
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM product_categories WHERE product_id=ANY($1::bigint[]) AND category_slug=$2`, ids, slug)
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,$2,'category-management',$3,$4,jsonb_build_object('reason',$5::text,'product_ids',$6::bigint[]))`, fmt.Sprint(actorID), "category.products."+intent, slug, ports.RequestID(ctx), reason, ids)
		return err
	})
}
