package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *CommerceRepository) ListingChange(ctx context.Context, actorID, productID int64) (commerce.ListingChange, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return commerce.ListingChange{}, err
	}
	defer tx.Rollback(ctx)
	actor, err := marketplaceIdentity(ctx, tx, actorID)
	if err != nil {
		return commerce.ListingChange{}, err
	}
	if actor.role != "super_admin" && actor.role != "seller" {
		return commerce.ListingChange{}, ports.ErrForbidden
	}
	out := commerce.ListingChange{ProductID: productID}
	err = tx.QueryRow(ctx, `SELECT p.name,s.display_name,c.revision,c.status,c.review_note,
		CASE WHEN c.cover_proposed IS NOT NULL THEN p.image_url ELSE '' END,COALESCE(c.cover_proposed,'')
		FROM products p JOIN sellers s ON s.id=p.seller_id JOIN listing_change_reviews c ON c.product_id=p.id
		WHERE p.id=$1 AND p.status='approved' AND ($2 OR s.id=$3)`, productID, actor.role == "super_admin", actor.seller).
		Scan(&out.Product, &out.Seller, &out.Revision, &out.State, &out.Note, &out.CurrentCover, &out.ProposedCover)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ports.ErrNotFound
	}
	if err != nil {
		return out, err
	}
	if out.State == "approved" {
		out.CurrentCover, out.ProposedCover = "", ""
		return out, tx.Commit(ctx)
	}
	rows, err := tx.Query(ctx, `SELECT v.name,v.price_cents,c.proposed_price,v.compare_at_cents,c.proposed_compare_at,c.new_variant
		FROM listing_price_changes c JOIN product_variants v ON v.id=c.variant_id
		WHERE c.product_id=$1 ORDER BY v.id`, productID)
	if err != nil {
		return out, err
	}
	out.Prices, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (commerce.ListingPriceChange, error) {
		var p commerce.ListingPriceChange
		err := row.Scan(&p.Option, &p.Current, &p.Proposed, &p.CurrentCompareAt, &p.ProposedCompareAt, &p.NewOption)
		return p, err
	})
	if err != nil {
		return out, err
	}
	rows, err = tx.Query(ctx, `SELECT m.public_url,m.content_type,c.publish FROM listing_media_changes c
		JOIN product_media m ON m.id=c.media_id WHERE c.product_id=$1 ORDER BY m.id`, productID)
	if err != nil {
		return out, err
	}
	out.Media, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (commerce.ListingMediaChange, error) {
		var m commerce.ListingMediaChange
		err := row.Scan(&m.Path, &m.ContentType, &m.Publish)
		return m, err
	})
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// The caller locks the product row before preparing or reviewing a change.
func openListingChange(ctx context.Context, tx pgx.Tx, productID int64) error {
	if _, err := tx.Exec(ctx, `INSERT INTO listing_change_reviews(product_id) VALUES($1)
		ON CONFLICT(product_id) DO UPDATE SET revision=listing_change_reviews.revision+1,
		status='pending',updated_at=now(),
		cover_base=CASE WHEN listing_change_reviews.status='approved' THEN NULL ELSE listing_change_reviews.cover_base END,
		cover_proposed=CASE WHEN listing_change_reviews.status='approved' THEN NULL ELSE listing_change_reviews.cover_proposed END,
		review_note=CASE WHEN listing_change_reviews.status='approved' THEN '' ELSE listing_change_reviews.review_note END`, productID); err != nil {
		return err
	}
	return nil
}

func stageListingPrice(ctx context.Context, tx pgx.Tx, productID, variantID, price int64, compareAt *int64, newVariant bool) error {
	var changed bool
	if err := tx.QueryRow(ctx, `SELECT $3 OR price_cents IS DISTINCT FROM $4 OR compare_at_cents IS DISTINCT FROM $5
		FROM product_variants WHERE id=$1 AND product_id=$2`, variantID, productID, newVariant, price, compareAt).Scan(&changed); err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err := openListingChange(ctx, tx, productID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO listing_price_changes(product_id,variant_id,base_price,base_compare_at,proposed_price,proposed_compare_at,new_variant)
		SELECT product_id,id,price_cents,compare_at_cents,$3,$4,$5 FROM product_variants WHERE id=$1 AND product_id=$2
		ON CONFLICT(product_id,variant_id) DO UPDATE SET base_price=EXCLUDED.base_price,base_compare_at=EXCLUDED.base_compare_at,
		proposed_price=EXCLUDED.proposed_price,proposed_compare_at=EXCLUDED.proposed_compare_at,new_variant=EXCLUDED.new_variant`,
		variantID, productID, price, compareAt, newVariant)
	return err
}

func stageListingCover(ctx context.Context, tx pgx.Tx, productID int64, path string) error {
	if path == "" {
		return nil
	}
	var current string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(image_url,'') FROM products WHERE id=$1`, productID).Scan(&current); err != nil {
		return err
	}
	if current == path {
		return nil
	}
	u, err := url.Parse(path)
	if err != nil || u.User != nil || (u.Scheme != "https" && (u.Host != "" || !strings.HasPrefix(u.Path, "/"))) {
		return ports.ErrInvalidState
	}
	if u.Host == "" && strings.HasPrefix(u.Path, "/media/products/") {
		var owned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM product_media
			WHERE product_id=$1 AND public_url=$2 AND status='ready' AND content_type LIKE 'image/%'
			AND (visibility='public' OR EXISTS(SELECT 1 FROM listing_media_changes c WHERE c.media_id=product_media.id AND c.publish)))`,
			productID, path).Scan(&owned); err != nil {
			return err
		}
		if !owned {
			return ports.ErrInvalidState
		}
	}
	if err := openListingChange(ctx, tx, productID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE listing_change_reviews SET cover_base=$2,cover_proposed=$3 WHERE product_id=$1`, productID, current, path)
	return err
}

func stageListingMedia(ctx context.Context, tx pgx.Tx, productID, mediaID int64, publish bool) error {
	if err := openListingChange(ctx, tx, productID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO listing_media_changes(product_id,media_id,publish) VALUES($1,$2,$3)
		ON CONFLICT(product_id,media_id) DO UPDATE SET publish=EXCLUDED.publish`, productID, mediaID, publish)
	return err
}

func (r *CommerceRepository) ReviewListingChange(ctx context.Context, actor, productID, revision int64, approve bool, reason string) error {
	return WithinTransaction(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		role, err := marketplaceIdentity(ctx, tx, actor)
		if err != nil {
			return err
		}
		if role.role != "super_admin" {
			return ports.ErrForbidden
		}
		var actual int64
		var cover, base *string
		err = tx.QueryRow(ctx, `SELECT c.revision,c.cover_proposed,c.cover_base
			FROM products p JOIN listing_change_reviews c ON c.product_id=p.id
			JOIN sellers s ON s.id=p.seller_id AND s.status='active'
			WHERE p.id=$1 AND p.status='approved' AND c.status='pending' FOR UPDATE OF p,c`, productID).Scan(&actual, &cover, &base)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && actual != revision) {
			return ports.ErrInvalidState
		}
		if err != nil {
			return err
		}
		status := "changes_requested"
		if approve {
			if err := applyListingChange(ctx, tx, productID, cover, base); err != nil {
				return err
			}
			status = "approved"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata)
			SELECT $2,'catalog.listing_change_reviewed','product',$1::bigint::text,$3,jsonb_build_object(
				'revision',c.revision,'status',$4::text,'reason',$5::text,'cover',c.cover_proposed,
				'prices',COALESCE((SELECT jsonb_agg(to_jsonb(pc) ORDER BY pc.variant_id)
					FROM listing_price_changes pc WHERE pc.product_id=c.product_id),'[]'::jsonb),
				'media',COALESCE((SELECT jsonb_agg(to_jsonb(mc) ORDER BY mc.media_id)
					FROM listing_media_changes mc WHERE mc.product_id=c.product_id),'[]'::jsonb))
			FROM listing_change_reviews c WHERE c.product_id=$1`, productID, fmt.Sprint(actor), ports.RequestID(ctx), status, reason); err != nil {
			return err
		}
		if approve {
			if _, err := tx.Exec(ctx, `DELETE FROM listing_price_changes WHERE product_id=$1`, productID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM listing_media_changes WHERE product_id=$1`, productID); err != nil {
				return err
			}
			if err := requestProductReindex(ctx, tx, productID, "approved"); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE listing_change_reviews SET status=$2,review_note=$3,revision=revision+1,updated_at=now() WHERE product_id=$1`, productID, status, reason)
		return err
	})
}

func applyListingChange(ctx context.Context, tx pgx.Tx, productID int64, cover, base *string) error {
	var expected int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM listing_price_changes WHERE product_id=$1`, productID).Scan(&expected); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `UPDATE product_variants v SET price_cents=c.proposed_price,compare_at_cents=c.proposed_compare_at,
		status='active',updated_at=now()
		FROM listing_price_changes c WHERE c.product_id=$1 AND v.product_id=c.product_id AND v.id=c.variant_id
		AND v.status=CASE WHEN c.new_variant THEN 'pending_review' ELSE 'active' END
		AND v.price_cents=c.base_price AND v.compare_at_cents IS NOT DISTINCT FROM c.base_compare_at`, productID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != expected {
		return ports.ErrInvalidState
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM listing_media_changes WHERE product_id=$1`, productID).Scan(&expected); err != nil {
		return err
	}
	result, err = tx.Exec(ctx, `UPDATE product_media m SET visibility=CASE WHEN c.publish THEN 'public' ELSE 'private' END,updated_at=now()
		FROM listing_media_changes c WHERE c.product_id=$1 AND m.product_id=c.product_id AND m.id=c.media_id
		AND m.status='ready' AND m.visibility=CASE WHEN c.publish THEN 'private' ELSE 'public' END`, productID)
	if err != nil {
		return err
	}
	if result.RowsAffected() != expected {
		return ports.ErrInvalidState
	}
	if cover != nil {
		result, err = tx.Exec(ctx, `UPDATE products SET image_url=$2,updated_at=now()
			WHERE id=$1 AND COALESCE(image_url,'')=$3`, productID, *cover, *base)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ports.ErrInvalidState
		}
	}
	// A reviewed removal of the cover chooses the latest remaining public still.
	if _, err := tx.Exec(ctx, `UPDATE products p SET image_url=COALESCE((
		SELECT m.public_url FROM product_media m WHERE m.product_id=p.id AND m.status='ready' AND m.visibility='public'
		AND m.content_type LIKE 'image/%' ORDER BY m.id DESC LIMIT 1),'')
		WHERE p.id=$1 AND EXISTS(SELECT 1 FROM listing_media_changes c JOIN product_media m ON m.id=c.media_id
		WHERE c.product_id=p.id AND NOT c.publish AND p.image_url=m.public_url)`, productID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE products p SET price_cents=v.price_cents,compare_at_cents=v.compare_at_cents,updated_at=now()
		FROM (SELECT price_cents,compare_at_cents FROM product_variants
			WHERE product_id=$1 AND status='active' ORDER BY id LIMIT 1) v WHERE p.id=$1`, productID)
	return err
}
