package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	domaincatalog "github.com/wecratfs/commerce/internal/domain/catalog"
	"github.com/wecratfs/commerce/internal/ports"
)

type ProductSearchRepository struct {
	pool *pgxpool.Pool
}

func NewProductSearchRepository(pool *pgxpool.Pool) *ProductSearchRepository {
	return &ProductSearchRepository{pool: pool}
}

func (r *ProductSearchRepository) Search(ctx context.Context, query, categorySlug string, limit, offset int) ([]domaincatalog.Product, int, error) {
	return r.SearchWithOptions(ctx, ports.ProductSearchOptions{Query: query, CategorySlug: categorySlug, Limit: limit, Offset: offset})
}

func (r *ProductSearchRepository) SearchWithOptions(ctx context.Context, options ports.ProductSearchOptions) ([]domaincatalog.Product, int, error) {
	query := options.Query
	categories := append([]string(nil), options.CategorySlugs...)
	if options.CategorySlug != "" {
		categories = append(categories, options.CategorySlug)
	}
	limit := options.Limit
	offset := options.Offset
	if limit <= 0 || limit > 48 {
		limit = 24
	}
	if offset < 0 {
		offset = 0
	}
	orderBy := "p.featured_rank ASC, p.id ASC"
	switch options.Sort {
	case "price_asc":
		orderBy = "pv.price_cents ASC, p.id ASC"
	case "price_desc":
		orderBy = "pv.price_cents DESC, p.id DESC"
	case "newest":
		orderBy = "p.created_at DESC, p.id DESC"
	}
	var total int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT p.id)
		FROM products p
		JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
		JOIN LATERAL (
			SELECT candidate.id, candidate.sku, candidate.price_cents, candidate.compare_at_cents
			FROM product_variants candidate
			LEFT JOIN inventory_stock candidate_stock ON candidate_stock.variant_id = candidate.id
			WHERE candidate.product_id = p.id AND candidate.status = 'active'
			ORDER BY (COALESCE(candidate_stock.available_quantity, 0) > 0) DESC, candidate.id
			LIMIT 1
		) pv ON TRUE
		LEFT JOIN inventory_stock i ON i.variant_id = pv.id
		WHERE p.status = 'approved'
		  AND ($1 = '' OR p.name ILIKE '%' || $1 || '%' OR p.description ILIKE '%' || $1 || '%' OR p.slug ILIKE '%' || $1 || '%'
		       OR EXISTS (SELECT 1 FROM product_variants search_variant WHERE search_variant.product_id = p.id AND search_variant.status = 'active' AND search_variant.sku ILIKE '%' || $1 || '%'))
		  AND (COALESCE(cardinality($2::text[]), 0) = 0 OR p.category_slug = ANY($2::text[]))
		  AND (COALESCE(cardinality($3::text[]), 0) = 0 OR p.brand_slug = ANY($3::text[]))
		  AND ($4 = '' OR ($4 = 'under-1000' AND pv.price_cents < 100000)
		       OR ($4 = '1000-2500' AND pv.price_cents >= 100000 AND pv.price_cents < 250000)
		       OR ($4 = '2500-5000' AND pv.price_cents >= 250000 AND pv.price_cents < 500000)
		       OR ($4 = 'over-5000' AND pv.price_cents >= 500000))
		  AND (NOT $5::boolean OR COALESCE(i.available_quantity, 0) > 0)
		  AND (NOT $6::boolean OR pv.compare_at_cents > pv.price_cents)`,
		query, categories, options.BrandSlugs, options.PriceRange, options.InStock, options.OffersOnly).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count product search: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.slug, b.name, p.name, p.description, p.category_slug, c.name,
		       pv.price_cents, COALESCE(pv.compare_at_cents, 0), (pv.compare_at_cents IS NOT NULL),
		       p.rating::float8, p.review_count, p.delivery_label, COALESCE(NULLIF((SELECT pm.public_url FROM product_media pm WHERE pm.product_id = p.id AND pm.status = 'ready' AND pm.visibility = 'public' AND pm.content_type LIKE 'image/%' ORDER BY pm.id DESC LIMIT 1), ''), p.image_url), p.swatches,
		       pv.id, pv.sku, COALESCE(i.available_quantity, 0)
		FROM products p
		JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
		JOIN categories c ON c.slug = p.category_slug
		JOIN brands b ON b.slug = p.brand_slug
		JOIN LATERAL (
			SELECT candidate.id, candidate.sku, candidate.price_cents, candidate.compare_at_cents FROM product_variants candidate
			LEFT JOIN inventory_stock candidate_stock ON candidate_stock.variant_id = candidate.id
			WHERE candidate.product_id = p.id AND candidate.status = 'active'
			ORDER BY (COALESCE(candidate_stock.available_quantity, 0) > 0) DESC, candidate.id
			LIMIT 1
		) pv ON TRUE
		LEFT JOIN inventory_stock i ON i.variant_id = pv.id
		WHERE p.status = 'approved'
		  AND ($1 = '' OR p.name ILIKE '%' || $1 || '%' OR p.description ILIKE '%' || $1 || '%' OR p.slug ILIKE '%' || $1 || '%'
		       OR EXISTS (SELECT 1 FROM product_variants search_variant WHERE search_variant.product_id = p.id AND search_variant.status = 'active' AND search_variant.sku ILIKE '%' || $1 || '%'))
		  AND (COALESCE(cardinality($2::text[]), 0) = 0 OR p.category_slug = ANY($2::text[]))
		  AND (COALESCE(cardinality($3::text[]), 0) = 0 OR p.brand_slug = ANY($3::text[]))
		  AND ($4 = '' OR ($4 = 'under-1000' AND pv.price_cents < 100000)
		       OR ($4 = '1000-2500' AND pv.price_cents >= 100000 AND pv.price_cents < 250000)
		       OR ($4 = '2500-5000' AND pv.price_cents >= 250000 AND pv.price_cents < 500000)
		       OR ($4 = 'over-5000' AND pv.price_cents >= 500000))
		  AND (NOT $5::boolean OR COALESCE(i.available_quantity, 0) > 0)
		  AND (NOT $6::boolean OR pv.compare_at_cents > pv.price_cents)
		ORDER BY `+orderBy+`
		LIMIT $7 OFFSET $8`, query, categories, options.BrandSlugs, options.PriceRange, options.InStock, options.OffersOnly, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("search products: %w", err)
	}
	defer rows.Close()
	products := make([]domaincatalog.Product, 0)
	for rows.Next() {
		var product domaincatalog.Product
		if err := rows.Scan(&product.ID, &product.Slug, &product.Brand, &product.Name, &product.Description, &product.CategorySlug, &product.CategoryName, &product.PriceCents, &product.CompareAtCents, &product.HasCompareAt, &product.Rating, &product.ReviewCount, &product.DeliveryLabel, &product.ImageURL, &product.Swatches, &product.VariantID, &product.SKU, &product.Available); err != nil {
			return nil, 0, fmt.Errorf("scan product search: %w", err)
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *ProductSearchRepository) SearchFacets(ctx context.Context, options ports.ProductSearchOptions) ([]ports.SearchFacet, error) {
	query := strings.TrimSpace(options.Query)
	categories := append([]string(nil), options.CategorySlugs...)
	if options.CategorySlug != "" {
		categories = append(categories, options.CategorySlug)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.category_slug, c.name, COUNT(DISTINCT p.id)
		FROM products p
		JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
		JOIN categories c ON c.slug = p.category_slug
		JOIN LATERAL (
			SELECT candidate.id, candidate.sku, candidate.price_cents, candidate.compare_at_cents
			FROM product_variants candidate
			LEFT JOIN inventory_stock candidate_stock ON candidate_stock.variant_id = candidate.id
			WHERE candidate.product_id = p.id AND candidate.status = 'active'
			ORDER BY (COALESCE(candidate_stock.available_quantity, 0) > 0) DESC, candidate.id
			LIMIT 1
		) pv ON TRUE
		LEFT JOIN inventory_stock i ON i.variant_id = pv.id
		WHERE p.status = 'approved'
		  AND ($1 = '' OR p.name ILIKE '%' || $1 || '%' OR p.description ILIKE '%' || $1 || '%' OR p.slug ILIKE '%' || $1 || '%'
		       OR EXISTS (SELECT 1 FROM product_variants search_variant WHERE search_variant.product_id = p.id AND search_variant.status = 'active' AND search_variant.sku ILIKE '%' || $1 || '%'))
		  AND (COALESCE(cardinality($2::text[]), 0) = 0 OR p.category_slug = ANY($2::text[]))
		  AND (COALESCE(cardinality($3::text[]), 0) = 0 OR p.brand_slug = ANY($3::text[]))
		  AND ($4 = '' OR ($4 = 'under-1000' AND pv.price_cents < 100000)
		       OR ($4 = '1000-2500' AND pv.price_cents >= 100000 AND pv.price_cents < 250000)
		       OR ($4 = '2500-5000' AND pv.price_cents >= 250000 AND pv.price_cents < 500000)
		       OR ($4 = 'over-5000' AND pv.price_cents >= 500000))
		  AND (NOT $5::boolean OR COALESCE(i.available_quantity, 0) > 0)
		  AND (NOT $6::boolean OR pv.compare_at_cents > pv.price_cents)
		GROUP BY p.category_slug, c.name
		ORDER BY COUNT(DISTINCT p.id) DESC, p.category_slug
		LIMIT 24`, query, categories, options.BrandSlugs, options.PriceRange, options.InStock, options.OffersOnly)
	if err != nil {
		return nil, fmt.Errorf("search product facets: %w", err)
	}
	defer rows.Close()
	facets := make([]ports.SearchFacet, 0)
	for rows.Next() {
		var facet ports.SearchFacet
		if err := rows.Scan(&facet.Value, &facet.Label, &facet.Count); err != nil {
			return nil, fmt.Errorf("scan product facet: %w", err)
		}
		facets = append(facets, facet)
	}
	return facets, rows.Err()
}

func (r *ProductSearchRepository) SearchBySlugs(ctx context.Context, slugs []string) ([]domaincatalog.Product, error) {
	if len(slugs) == 0 {
		return []domaincatalog.Product{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.slug, b.name, p.name, p.description, p.category_slug, c.name,
		       pv.price_cents, COALESCE(pv.compare_at_cents, 0), (pv.compare_at_cents IS NOT NULL),
		       p.rating::float8, p.review_count, p.delivery_label, COALESCE(NULLIF((SELECT pm.public_url FROM product_media pm WHERE pm.product_id = p.id AND pm.status = 'ready' AND pm.visibility = 'public' AND pm.content_type LIKE 'image/%' ORDER BY pm.id DESC LIMIT 1), ''), p.image_url), p.swatches,
		       pv.id, pv.sku, COALESCE(i.available_quantity, 0)
		FROM products p
		JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
		JOIN categories c ON c.slug = p.category_slug
		JOIN brands b ON b.slug = p.brand_slug
		JOIN LATERAL (
			SELECT candidate.id, candidate.price_cents, candidate.compare_at_cents FROM product_variants candidate
			LEFT JOIN inventory_stock candidate_stock ON candidate_stock.variant_id = candidate.id
			WHERE candidate.product_id = p.id AND candidate.status = 'active'
			ORDER BY (COALESCE(candidate_stock.available_quantity, 0) > 0) DESC, candidate.id
			LIMIT 1
		) pv ON TRUE
		LEFT JOIN inventory_stock i ON i.variant_id = pv.id
		WHERE p.status = 'approved' AND p.slug = ANY($1::text[])
		ORDER BY array_position($1::text[], p.slug), pv.id`, slugs)
	if err != nil {
		return nil, fmt.Errorf("hydrate search products: %w", err)
	}
	defer rows.Close()
	products := make([]domaincatalog.Product, 0, len(slugs))
	for rows.Next() {
		var product domaincatalog.Product
		if err := rows.Scan(&product.ID, &product.Slug, &product.Brand, &product.Name, &product.Description, &product.CategorySlug, &product.CategoryName, &product.PriceCents, &product.CompareAtCents, &product.HasCompareAt, &product.Rating, &product.ReviewCount, &product.DeliveryLabel, &product.ImageURL, &product.Swatches, &product.VariantID, &product.SKU, &product.Available); err != nil {
			return nil, fmt.Errorf("scan hydrated search product: %w", err)
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}

func (r *ProductSearchRepository) ListApprovedForIndex(ctx context.Context) ([]ports.SearchDocument, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.slug, p.name, p.description, b.name, p.brand_slug, p.category_slug,
		       (SELECT ARRAY_AGG(search_variant.sku) FROM product_variants search_variant WHERE search_variant.product_id = p.id AND search_variant.status = 'active'),
		       pv.price_cents, COALESCE(pv.compare_at_cents, 0), COALESCE(pv.compare_at_cents > pv.price_cents, false), COALESCE(i.available_quantity, 0)
		FROM products p
		JOIN sellers s ON s.id = p.seller_id AND s.status = 'active'
		JOIN brands b ON b.slug = p.brand_slug
		LEFT JOIN LATERAL (
			SELECT candidate.id, candidate.sku, candidate.price_cents, candidate.compare_at_cents
			FROM product_variants candidate
			LEFT JOIN inventory_stock candidate_stock ON candidate_stock.variant_id = candidate.id
			WHERE candidate.product_id = p.id AND candidate.status = 'active'
			ORDER BY (COALESCE(candidate_stock.available_quantity, 0) > 0) DESC, candidate.id
			LIMIT 1
		) pv ON TRUE
		LEFT JOIN inventory_stock i ON i.variant_id = pv.id
		WHERE p.status = 'approved'
		ORDER BY p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	documents := make([]ports.SearchDocument, 0)
	for rows.Next() {
		var document ports.SearchDocument
		if err := rows.Scan(&document.ID, &document.Slug, &document.Name, &document.Description, &document.Brand, &document.BrandSlug, &document.CategorySlug, &document.SKUs, &document.PriceCents, &document.CompareAtCents, &document.HasOffer, &document.Available); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, rows.Err()
}
