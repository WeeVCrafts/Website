package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
	"github.com/wecratfs/commerce/internal/ports"
)

type ImageStudioRepository struct {
	pool *pgxpool.Pool
}

func NewImageStudioRepository(pool *pgxpool.Pool) *ImageStudioRepository {
	return &ImageStudioRepository{pool: pool}
}

func (r *ImageStudioRepository) ListSellerProducts(ctx context.Context, ownerUserID int64) ([]domainstudio.ProductOption, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.name, COALESCE(v.sku, ''), p.category_slug, c.name, p.status,
		       COALESCE(NULLIF(p.image_url, ''), (
		           SELECT pm.public_url FROM product_media pm
		           WHERE pm.product_id = p.id AND pm.status = 'ready' AND pm.content_type LIKE 'image/%'
		           ORDER BY pm.id DESC LIMIT 1
		       ), '')
		FROM products p
		JOIN sellers s ON s.id = p.seller_id
		JOIN categories c ON c.slug = p.category_slug
		LEFT JOIN LATERAL (
			SELECT sku FROM product_variants WHERE product_id = p.id AND status = 'active' ORDER BY id LIMIT 1
		) v ON TRUE
		WHERE p.status <> 'archived' AND `+strings.ReplaceAll(mediaManageCondition, "$2", "$1")+`
		ORDER BY p.updated_at DESC, p.id DESC
		LIMIT 200`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]domainstudio.ProductOption, 0)
	for rows.Next() {
		var p domainstudio.ProductOption
		if err := rows.Scan(&p.ID, &p.Name, &p.SKU, &p.CategorySlug, &p.CategoryName, &p.Status, &p.ImageURL); err != nil {
			return nil, err
		}
		if strings.HasPrefix(p.ImageURL, "/media/products/") && !strings.HasSuffix(p.ImageURL, "/preview") {
			p.ImageURL += "/preview"
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (r *ImageStudioRepository) GetOwnedProduct(ctx context.Context, ownerUserID, productID int64) (domainstudio.ProductOption, error) {
	var p domainstudio.ProductOption
	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.name, COALESCE(v.sku, ''), p.category_slug, c.name, p.status,
		       COALESCE(NULLIF(p.image_url, ''), (
		           SELECT pm.public_url FROM product_media pm
		           WHERE pm.product_id = p.id AND pm.status = 'ready' AND pm.content_type LIKE 'image/%'
		           ORDER BY pm.id DESC LIMIT 1
		       ), '')
		FROM products p
		JOIN sellers s ON s.id = p.seller_id
		JOIN categories c ON c.slug = p.category_slug
		LEFT JOIN LATERAL (
			SELECT sku FROM product_variants WHERE product_id = p.id AND status = 'active' ORDER BY id LIMIT 1
		) v ON TRUE
		WHERE p.id = $1 AND p.status <> 'archived' AND `+mediaManageCondition,
		productID, ownerUserID,
	).Scan(&p.ID, &p.Name, &p.SKU, &p.CategorySlug, &p.CategoryName, &p.Status, &p.ImageURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainstudio.ProductOption{}, ports.ErrForbidden
	}
	if err != nil {
		return domainstudio.ProductOption{}, err
	}
	if strings.HasPrefix(p.ImageURL, "/media/products/") && !strings.HasSuffix(p.ImageURL, "/preview") {
		p.ImageURL += "/preview"
	}
	return p, nil
}

func (r *ImageStudioRepository) CreateJob(ctx context.Context, job domainstudio.Job) (domainstudio.Job, error) {
	colorsJSON, err := json.Marshal(job.DominantColors)
	if err != nil {
		colorsJSON = []byte("[]")
	}
	var out domainstudio.Job
	var rawColors []byte
	err = r.pool.QueryRow(ctx, `
		INSERT INTO ai_image_jobs (
			product_id, seller_id, owner_user_id, source_object_key, cutout_object_key,
			source_content_type, source_byte_size, source_width, source_height,
			dominant_colors, aspect_ratio, status, engine_used, error_message
		)
		SELECT p.id, s.id, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13
		FROM products p
		JOIN sellers s ON s.id = p.seller_id
		WHERE p.id = $1 AND p.status <> 'archived' AND `+mediaManageCondition+`
		RETURNING id, product_id, seller_id, owner_user_id, source_object_key, cutout_object_key,
		          source_content_type, source_byte_size, source_width, source_height,
		          dominant_colors, aspect_ratio, status, engine_used, error_message, created_at, updated_at`,
		job.ProductID, job.OwnerUserID, job.SourceObjectKey, job.CutoutObjectKey,
		job.SourceContentType, job.SourceByteSize, job.SourceWidth, job.SourceHeight,
		string(colorsJSON), fallbackAspect(job.AspectRatio), fallbackStatus(job.Status), job.EngineUsed, job.ErrorMessage,
	).Scan(
		&out.ID, &out.ProductID, &out.SellerID, &out.OwnerUserID, &out.SourceObjectKey, &out.CutoutObjectKey,
		&out.SourceContentType, &out.SourceByteSize, &out.SourceWidth, &out.SourceHeight,
		&rawColors, &out.AspectRatio, &out.Status, &out.EngineUsed, &out.ErrorMessage, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainstudio.Job{}, ports.ErrForbidden
	}
	if err != nil {
		return domainstudio.Job{}, err
	}
	_ = json.Unmarshal(rawColors, &out.DominantColors)
	product, pErr := r.GetOwnedProduct(ctx, job.OwnerUserID, out.ProductID)
	if pErr == nil {
		out.ProductName = product.Name
		out.CategorySlug = product.CategorySlug
		out.CategoryName = product.CategoryName
	}
	decorateJobURLs(&out)
	return out, nil
}

func (r *ImageStudioRepository) GetJob(ctx context.Context, ownerUserID, jobID int64) (domainstudio.Job, error) {
	var out domainstudio.Job
	var rawColors []byte
	err := r.pool.QueryRow(ctx, `
		SELECT j.id, j.product_id, p.name, p.category_slug, c.name, j.seller_id, j.owner_user_id,
		       j.source_object_key, j.cutout_object_key, j.source_content_type, j.source_byte_size,
		       j.source_width, j.source_height, j.dominant_colors, j.aspect_ratio,
		       j.status, j.engine_used, j.error_message, j.created_at, j.updated_at
		FROM ai_image_jobs j
		JOIN products p ON p.id = j.product_id
		JOIN sellers s ON s.id = p.seller_id
		JOIN categories c ON c.slug = p.category_slug
		WHERE j.id = $1 AND `+mediaManageCondition,
		jobID, ownerUserID,
	).Scan(
		&out.ID, &out.ProductID, &out.ProductName, &out.CategorySlug, &out.CategoryName, &out.SellerID, &out.OwnerUserID,
		&out.SourceObjectKey, &out.CutoutObjectKey, &out.SourceContentType, &out.SourceByteSize,
		&out.SourceWidth, &out.SourceHeight, &rawColors, &out.AspectRatio,
		&out.Status, &out.EngineUsed, &out.ErrorMessage, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainstudio.Job{}, ports.ErrForbidden
	}
	if err != nil {
		return domainstudio.Job{}, err
	}
	_ = json.Unmarshal(rawColors, &out.DominantColors)
	variations, err := r.listJobVariations(ctx, out.ID)
	if err != nil {
		return domainstudio.Job{}, err
	}
	out.Variations = variations
	decorateJobURLs(&out)
	return out, nil
}

func (r *ImageStudioRepository) ListProductJobs(ctx context.Context, ownerUserID, productID int64, limit int) ([]domainstudio.Job, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	rows, err := r.pool.Query(ctx, `
		SELECT j.id, j.product_id, p.name, p.category_slug, c.name, j.seller_id, j.owner_user_id,
		       j.source_object_key, j.cutout_object_key, j.source_content_type, j.source_byte_size,
		       j.source_width, j.source_height, j.dominant_colors, j.aspect_ratio,
		       j.status, j.engine_used, j.error_message, j.created_at, j.updated_at
		FROM ai_image_jobs j
		JOIN products p ON p.id = j.product_id
		JOIN sellers s ON s.id = p.seller_id
		JOIN categories c ON c.slug = p.category_slug
		WHERE j.product_id = $1 AND `+mediaManageCondition+`
		ORDER BY j.created_at DESC, j.id DESC
		LIMIT $3`,
		productID, ownerUserID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]domainstudio.Job, 0)
	for rows.Next() {
		var item domainstudio.Job
		var rawColors []byte
		if err := rows.Scan(
			&item.ID, &item.ProductID, &item.ProductName, &item.CategorySlug, &item.CategoryName, &item.SellerID, &item.OwnerUserID,
			&item.SourceObjectKey, &item.CutoutObjectKey, &item.SourceContentType, &item.SourceByteSize,
			&item.SourceWidth, &item.SourceHeight, &rawColors, &item.AspectRatio,
			&item.Status, &item.EngineUsed, &item.ErrorMessage, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rawColors, &item.DominantColors)
		jobs = append(jobs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range jobs {
		vars, vErr := r.listJobVariations(ctx, jobs[i].ID)
		if vErr != nil {
			return nil, vErr
		}
		jobs[i].Variations = vars
		decorateJobURLs(&jobs[i])
	}
	return jobs, nil
}

func (r *ImageStudioRepository) MarkJobProcessing(ctx context.Context, jobID int64) error {
	_, err := r.pool.Exec(ctx, `UPDATE ai_image_jobs SET status = 'processing', error_message = '', updated_at = NOW() WHERE id = $1`, jobID)
	return err
}

func (r *ImageStudioRepository) MarkJobCompleted(ctx context.Context, jobID int64, cutoutKey string, width, height int, aspectRatio string, colors []domainstudio.DominantColor, engine string) error {
	colorsJSON, err := json.Marshal(colors)
	if err != nil {
		colorsJSON = []byte("[]")
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE ai_image_jobs
		SET cutout_object_key = $2,
		    source_width = $3,
		    source_height = $4,
		    aspect_ratio = $5,
		    dominant_colors = $6::jsonb,
		    engine_used = $7,
		    status = 'completed',
		    error_message = '',
		    updated_at = NOW()
		WHERE id = $1`,
		jobID, cutoutKey, width, height, fallbackAspect(aspectRatio), string(colorsJSON), engine,
	)
	return err
}

func (r *ImageStudioRepository) MarkJobFailed(ctx context.Context, jobID int64, errMsg string) error {
	if len(errMsg) > 500 {
		cutoff := 0
		for i := range errMsg {
			if i > 500 {
				break
			}
			cutoff = i
		}
		errMsg = errMsg[:cutoff]
	}
	_, err := r.pool.Exec(ctx, `UPDATE ai_image_jobs SET status = 'failed', error_message = $2, updated_at = NOW() WHERE id = $1`, jobID, errMsg)
	return err
}

func (r *ImageStudioRepository) UpdateJobStatus(ctx context.Context, ownerUserID, jobID int64, status string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE ai_image_jobs j
		SET status = $3, updated_at = NOW()
		FROM products p JOIN sellers s ON s.id = p.seller_id
		WHERE j.id = $1 AND p.id = j.product_id AND `+mediaManageCondition,
		jobID, ownerUserID, status,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ports.ErrForbidden
	}
	return nil
}

func (r *ImageStudioRepository) CreateVariation(ctx context.Context, v domainstudio.Variation) (domainstudio.Variation, error) {
	params := v.Composition.Normalize()
	var out domainstudio.Variation
	err := r.pool.QueryRow(ctx, `
		INSERT INTO ai_image_variations (
			job_id, preset_slug, preset_label, category_recommended, object_key,
			content_type, byte_size, width, height, offset_x, offset_y, scale, fit_mode, shadow_strength, status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, job_id, preset_slug, preset_label, category_recommended, object_key,
		          content_type, byte_size, width, height, offset_x, offset_y, scale, fit_mode, shadow_strength,
		          status, published_media_id, created_at, updated_at`,
		v.JobID, v.PresetSlug, v.PresetLabel, v.CategoryRecommended, v.ObjectKey,
		v.ContentType, v.ByteSize, v.Width, v.Height, params.OffsetX, params.OffsetY, params.Scale, params.FitMode, params.ShadowStrength,
		domainstudio.VariationStatusPendingApproval,
	).Scan(
		&out.ID, &out.JobID, &out.PresetSlug, &out.PresetLabel, &out.CategoryRecommended, &out.ObjectKey,
		&out.ContentType, &out.ByteSize, &out.Width, &out.Height,
		&out.Composition.OffsetX, &out.Composition.OffsetY, &out.Composition.Scale, &out.Composition.FitMode, &out.Composition.ShadowStrength,
		&out.Status, &out.PublishedMediaID, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return domainstudio.Variation{}, err
	}
	out.PreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/variations/%d/preview", out.ID)
	return out, nil
}

func (r *ImageStudioRepository) GetVariation(ctx context.Context, ownerUserID, variationID int64) (domainstudio.Variation, domainstudio.Job, error) {
	var v domainstudio.Variation
	var jobID int64
	err := r.pool.QueryRow(ctx, `
		SELECT v.id, v.job_id, v.preset_slug, v.preset_label, v.category_recommended, v.object_key,
		       v.content_type, v.byte_size, v.width, v.height, v.offset_x, v.offset_y, v.scale, v.fit_mode, v.shadow_strength,
		       v.status, v.published_media_id, v.created_at, v.updated_at
		FROM ai_image_variations v
		JOIN ai_image_jobs j ON j.id = v.job_id
		JOIN products p ON p.id = j.product_id
		JOIN sellers s ON s.id = p.seller_id
		WHERE v.id = $1 AND `+mediaManageCondition,
		variationID, ownerUserID,
	).Scan(
		&v.ID, &jobID, &v.PresetSlug, &v.PresetLabel, &v.CategoryRecommended, &v.ObjectKey,
		&v.ContentType, &v.ByteSize, &v.Width, &v.Height,
		&v.Composition.OffsetX, &v.Composition.OffsetY, &v.Composition.Scale, &v.Composition.FitMode, &v.Composition.ShadowStrength,
		&v.Status, &v.PublishedMediaID, &v.CreatedAt, &v.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainstudio.Variation{}, domainstudio.Job{}, ports.ErrForbidden
	}
	if err != nil {
		return domainstudio.Variation{}, domainstudio.Job{}, err
	}
	v.JobID = jobID
	v.PreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/variations/%d/preview?v=%d", v.ID, v.UpdatedAt.UnixNano())
	job, err := r.GetJob(ctx, ownerUserID, jobID)
	if err != nil {
		return domainstudio.Variation{}, domainstudio.Job{}, err
	}
	return v, job, nil
}

func (r *ImageStudioRepository) UpdateVariationRender(ctx context.Context, variationID int64, objectKey, contentType string, byteSize int64, width, height int, params domainstudio.CompositionParams) (domainstudio.Variation, error) {
	params = params.Normalize()
	var out domainstudio.Variation
	err := r.pool.QueryRow(ctx, `
		UPDATE ai_image_variations
		SET object_key = $2, content_type = $3, byte_size = $4, width = $5, height = $6,
		    offset_x = $7, offset_y = $8, scale = $9, fit_mode = $10, shadow_strength = $11,
		    status = 'pending_approval', updated_at = NOW()
		WHERE id = $1
		RETURNING id, job_id, preset_slug, preset_label, category_recommended, object_key,
		          content_type, byte_size, width, height, offset_x, offset_y, scale, fit_mode, shadow_strength,
		          status, published_media_id, created_at, updated_at`,
		variationID, objectKey, contentType, byteSize, width, height,
		params.OffsetX, params.OffsetY, params.Scale, params.FitMode, params.ShadowStrength,
	).Scan(
		&out.ID, &out.JobID, &out.PresetSlug, &out.PresetLabel, &out.CategoryRecommended, &out.ObjectKey,
		&out.ContentType, &out.ByteSize, &out.Width, &out.Height,
		&out.Composition.OffsetX, &out.Composition.OffsetY, &out.Composition.Scale, &out.Composition.FitMode, &out.Composition.ShadowStrength,
		&out.Status, &out.PublishedMediaID, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return domainstudio.Variation{}, err
	}
	out.PreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/variations/%d/preview?v=%d", out.ID, out.UpdatedAt.UnixNano())
	return out, nil
}

func (r *ImageStudioRepository) SetVariationStatus(ctx context.Context, ownerUserID, variationID int64, status string, publishedMediaID *int64) (domainstudio.Variation, error) {
	var out domainstudio.Variation
	err := r.pool.QueryRow(ctx, `
		UPDATE ai_image_variations v
		SET status = $3, published_media_id = COALESCE($4, v.published_media_id), updated_at = NOW()
		FROM ai_image_jobs j
		JOIN products p ON p.id = j.product_id
		JOIN sellers s ON s.id = p.seller_id
		WHERE v.id = $1 AND j.id = v.job_id AND `+mediaManageCondition+`
		RETURNING v.id, v.job_id, v.preset_slug, v.preset_label, v.category_recommended, v.object_key,
		          v.content_type, v.byte_size, v.width, v.height, v.offset_x, v.offset_y, v.scale, v.fit_mode, v.shadow_strength,
		          v.status, v.published_media_id, v.created_at, v.updated_at`,
		variationID, ownerUserID, status, publishedMediaID,
	).Scan(
		&out.ID, &out.JobID, &out.PresetSlug, &out.PresetLabel, &out.CategoryRecommended, &out.ObjectKey,
		&out.ContentType, &out.ByteSize, &out.Width, &out.Height,
		&out.Composition.OffsetX, &out.Composition.OffsetY, &out.Composition.Scale, &out.Composition.FitMode, &out.Composition.ShadowStrength,
		&out.Status, &out.PublishedMediaID, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainstudio.Variation{}, ports.ErrForbidden
	}
	if err != nil {
		return domainstudio.Variation{}, err
	}
	out.PreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/variations/%d/preview?v=%d", out.ID, out.UpdatedAt.UnixNano())
	return out, nil
}

func (r *ImageStudioRepository) listJobVariations(ctx context.Context, jobID int64) ([]domainstudio.Variation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, job_id, preset_slug, preset_label, category_recommended, object_key,
		       content_type, byte_size, width, height, offset_x, offset_y, scale, fit_mode, shadow_strength,
		       status, published_media_id, created_at, updated_at
		FROM ai_image_variations
		WHERE job_id = $1
		ORDER BY category_recommended DESC, id ASC`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	variations := make([]domainstudio.Variation, 0)
	for rows.Next() {
		var v domainstudio.Variation
		if err := rows.Scan(
			&v.ID, &v.JobID, &v.PresetSlug, &v.PresetLabel, &v.CategoryRecommended, &v.ObjectKey,
			&v.ContentType, &v.ByteSize, &v.Width, &v.Height,
			&v.Composition.OffsetX, &v.Composition.OffsetY, &v.Composition.Scale, &v.Composition.FitMode, &v.Composition.ShadowStrength,
			&v.Status, &v.PublishedMediaID, &v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		v.PreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/variations/%d/preview?v=%d", v.ID, v.UpdatedAt.UnixNano())
		variations = append(variations, v)
	}
	return variations, rows.Err()
}

func decorateJobURLs(job *domainstudio.Job) {
	job.SourcePreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/jobs/%d/source", job.ID)
	if job.CutoutObjectKey != "" {
		job.CutoutPreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/jobs/%d/cutout", job.ID)
	}
	job.Recommendation = domainstudio.RecommendPresets(job.CategorySlug, job.CategoryName, job.AspectRatio, job.DominantColors)
}

func fallbackAspect(v string) string {
	switch v {
	case "portrait", "landscape":
		return v
	default:
		return "square"
	}
}

func fallbackStatus(v string) string {
	if v == "" {
		return domainstudio.JobStatusUploaded
	}
	return v
}
