package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/domain/commerce"
)

// Caller holds the product/actor locks. Nil preserves existing curation for
// older clients; an explicit category list replaces secondary memberships.
func applyAdminCategories(ctx context.Context, tx pgx.Tx, id int64, input commerce.ProductDraftInput) error {
	if err := saveProductTags(ctx, tx, id, input.SearchTags); err != nil {
		return err
	}
	if input.FeaturedPinned != nil {
		if _, err := tx.Exec(ctx, `UPDATE products SET featured_pinned=$2 WHERE id=$1`, id, *input.FeaturedPinned); err != nil {
			return err
		}
	}
	if input.CategorySlugs != nil {
		slugs := append([]string{input.CategorySlug}, input.CategorySlugs...)
		if _, err := tx.Exec(ctx, `DELETE FROM product_categories WHERE product_id=$1 AND NOT(category_slug=ANY($2::text[]))`, id, slugs); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO product_categories(product_id,category_slug) SELECT DISTINCT $1::bigint,unnest($2::text[]) ON CONFLICT DO NOTHING`, id, slugs); err != nil {
			return err
		}
	}
	if input.FeaturedRank != nil {
		_, err := tx.Exec(ctx, `UPDATE products SET featured_rank=$2 WHERE id=$1`, id, *input.FeaturedRank)
		return err
	}
	return nil
}

func saveProductTags(ctx context.Context, tx pgx.Tx, id int64, tags []string) error {
	if tags == nil {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE products SET search_tags=$2 WHERE id=$1`, id, tags)
	return err
}
