package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	domain "github.com/wecratfs/commerce/internal/domain/commerce"
	"github.com/wecratfs/commerce/internal/ports"
)

func (r *MediaRepository) PublicDerivative(ctx context.Context, media domain.MediaRecord, width int) (ports.MediaDerivative, error) {
	var d ports.MediaDerivative
	err := r.pool.QueryRow(ctx, `
		SELECT object_key,content_type,byte_size FROM product_image_derivatives
		WHERE media_id=$1 AND width=$2 AND source_key=$3 AND source_type=$4 AND source_bytes=$5
	`, media.ID, width, media.ObjectKey, media.ContentType, media.ByteSize).Scan(&d.ObjectKey, &d.ContentType, &d.ByteSize)
	return d, err
}

func (r *MediaRepository) SaveDerivative(ctx context.Context, d ports.MediaDerivative) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO product_image_derivatives(media_id,width,source_key,source_type,source_bytes,object_key,content_type,byte_size)
		SELECT id,$2,$3,$4,$5,$6,$7,$8 FROM product_media
		WHERE id=$1 AND object_key=$3 AND content_type=$4 AND byte_size=$5 AND status='ready' AND visibility='public'
		ON CONFLICT(media_id,width) DO UPDATE SET source_key=EXCLUDED.source_key,
			source_type=EXCLUDED.source_type,source_bytes=EXCLUDED.source_bytes,
			object_key=EXCLUDED.object_key,content_type=EXCLUDED.content_type,
			byte_size=EXCLUDED.byte_size,created_at=now()
	`, d.MediaID, d.Width, d.SourceKey, d.SourceType, d.SourceBytes, d.ObjectKey, d.ContentType, d.ByteSize)
	return err
}

func (r *MediaRepository) DerivativeCandidates(ctx context.Context, limit int) ([]domain.MediaRecord, error) {
	// Persistent periodic jobs also backfill photographs uploaded before this
	// release. The public resolver remains the authorization gate for reads.
	rows, err := r.pool.Query(ctx, mediaRecordSelect+`
		WHERE pm.status='ready' AND pm.visibility='public'
		AND pm.content_type IN ('image/jpeg','image/png','image/webp')
		AND (pm.derivatives_checked_at IS NULL OR pm.derivatives_checked_at<now()-interval '10 minutes')
		AND EXISTS(SELECT 1 FROM products p JOIN sellers s ON s.id=p.seller_id WHERE p.id=pm.product_id AND p.status='approved' AND s.status='active')
		AND (SELECT count(*) FROM product_image_derivatives d WHERE d.media_id=pm.id AND d.source_key=pm.object_key AND d.source_type=pm.content_type AND d.source_bytes=pm.byte_size)<3
		ORDER BY pm.derivatives_checked_at NULLS FIRST,pm.id LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	records, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.MediaRecord, error) { return scanMediaRecord(row) })
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if _, err := r.pool.Exec(ctx, `UPDATE product_media SET derivatives_checked_at=now() WHERE id=$1`, record.ID); err != nil {
			return nil, err
		}
	}
	return records, nil
}
