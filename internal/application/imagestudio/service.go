package imagestudio

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
	"github.com/wecratfs/commerce/internal/ports"
)

var (
	ErrStudioUnavailable = errors.New("image studio is temporarily unavailable")
	ErrInvalidPreset     = errors.New("background preset is invalid")
	ErrJobNotReady       = errors.New("image job is not ready for composition")
)

type MediaPublisher interface {
	PrepareUpload(ctx context.Context, sellerOwnerID, productID int64, contentType string, byteSize int64) (domaincommerce.MediaUpload, error)
	UploadObject(ctx context.Context, sellerOwnerID, mediaID int64, contentType string, byteSize int64, body io.Reader) error
	FinalizeUpload(ctx context.Context, sellerOwnerID, mediaID int64) (domaincommerce.MediaRecord, error)
}

type Service struct {
	repo      ports.ImageStudioRepository
	storage   ports.StudioObjectStorage
	processor ports.ImageProcessor
	media     MediaPublisher
	roles     ports.RoleChecker

	queue chan func(context.Context)
	wg    sync.WaitGroup
}

func NewService(repo ports.ImageStudioRepository, storage ports.StudioObjectStorage, processor ports.ImageProcessor, media MediaPublisher, roles ports.RoleChecker) *Service {
	return &Service{
		repo:      repo,
		storage:   storage,
		processor: processor,
		media:     media,
		roles:     roles,
	}
}

// StartWorkers launches a single bounded worker goroutine so 8 GB RAM hosts
// process at most one AI job at a time without blocking HTTP handlers.
func (s *Service) StartWorkers(ctx context.Context) {
	if s.queue != nil {
		return
	}
	s.queue = make(chan func(context.Context), 16)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case task := <-s.queue:
				if task != nil {
					task(ctx)
				}
			}
		}
	}()
}

func (s *Service) Wait() {
	s.wg.Wait()
}

func (s *Service) ListProducts(ctx context.Context, actorID int64) ([]domainstudio.ProductOption, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil {
		return nil, ports.ErrForbidden
	}
	return s.repo.ListSellerProducts(ctx, actorID)
}

func (s *Service) ListProductJobs(ctx context.Context, actorID, productID int64) ([]domainstudio.Job, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || productID <= 0 {
		return nil, ports.ErrForbidden
	}
	return s.repo.ListProductJobs(ctx, actorID, productID, 12)
}

func (s *Service) GetJob(ctx context.Context, actorID, jobID int64) (domainstudio.Job, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || jobID <= 0 {
		return domainstudio.Job{}, ports.ErrForbidden
	}
	return s.repo.GetJob(ctx, actorID, jobID)
}

// UploadAndStart validates the uploaded photo, preserves the original in private
// studio object storage, records the job, and runs segmentation + variations
// asynchronously when workers are started (or synchronously otherwise).
func (s *Service) UploadAndStart(ctx context.Context, actorID, productID int64, declaredType string, data []byte) (domainstudio.Job, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || s.storage == nil || s.processor == nil || productID <= 0 {
		return domainstudio.Job{}, ports.ErrForbidden
	}
	product, err := s.repo.GetOwnedProduct(ctx, actorID, productID)
	if err != nil {
		return domainstudio.Job{}, err
	}
	_, mimeType, w, h, err := s.processor.ValidateAndPrepare(data, declaredType)
	if err != nil {
		return domainstudio.Job{}, err
	}

	sourceKey, err := randomStudioKey(actorID, mimeExt(mimeType))
	if err != nil {
		return domainstudio.Job{}, err
	}
	if err := s.storage.PutObject(ctx, sourceKey, mimeType, int64(len(data)), bytes.NewReader(data)); err != nil {
		return domainstudio.Job{}, fmt.Errorf("%w: %v", ErrStudioUnavailable, err)
	}

	job, err := s.repo.CreateJob(ctx, domainstudio.Job{
		ProductID:         product.ID,
		ProductName:       product.Name,
		CategorySlug:      product.CategorySlug,
		CategoryName:      product.CategoryName,
		OwnerUserID:       actorID,
		SourceObjectKey:   sourceKey,
		SourceContentType: mimeType,
		SourceByteSize:    int64(len(data)),
		SourceWidth:       w,
		SourceHeight:      h,
		AspectRatio:       classifyAspect(w, h),
		Status:            domainstudio.JobStatusUploaded,
	})
	if err != nil {
		_ = s.storage.DeleteObject(ctx, sourceKey)
		return domainstudio.Job{}, err
	}

	if s.queue != nil {
		select {
		case s.queue <- func(bgCtx context.Context) {
			runCtx, cancel := context.WithTimeout(bgCtx, 60*time.Second)
			defer cancel()
			if _, procErr := s.ProcessJob(runCtx, actorID, job.ID); procErr != nil {
				slog.Warn("ai image studio background job failed", "job_id", job.ID, "error", procErr)
			}
		}:
			return job, nil
		default:
			// Queue full: process synchronously under request context so work is never dropped.
		}
	}

	return s.ProcessJob(ctx, actorID, job.ID)
}

func (s *Service) ProcessJob(ctx context.Context, actorID, jobID int64) (domainstudio.Job, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || s.storage == nil || s.processor == nil {
		return domainstudio.Job{}, ports.ErrForbidden
	}
	job, err := s.repo.GetJob(ctx, actorID, jobID)
	if err != nil {
		return domainstudio.Job{}, err
	}
	if err := s.repo.MarkJobProcessing(ctx, job.ID); err != nil {
		return domainstudio.Job{}, err
	}

	fail := func(cause error) (domainstudio.Job, error) {
		_ = s.repo.MarkJobFailed(context.Background(), job.ID, cause.Error())
		updated, getErr := s.repo.GetJob(ctx, actorID, job.ID)
		if getErr == nil {
			return updated, cause
		}
		return job, cause
	}

	sourceData, _, err := s.readObjectBounded(ctx, job.SourceObjectKey, domainstudio.MaxUploadBytes)
	if err != nil {
		return fail(err)
	}
	prepared, _, w, h, err := s.processor.ValidateAndPrepare(sourceData, job.SourceContentType)
	if err != nil {
		return fail(err)
	}
	// Release raw source bytes before running segmentation + variations to save RAM.
	sourceData = nil

	cutout, err := s.processor.RemoveBackground(ctx, prepared)
	if err != nil {
		return fail(err)
	}
	prepared = nil

	cutoutKey, err := randomStudioKey(actorID, ".png")
	if err != nil {
		return fail(err)
	}
	if err := s.storage.PutObject(ctx, cutoutKey, "image/png", int64(len(cutout.CutoutPNG)), bytes.NewReader(cutout.CutoutPNG)); err != nil {
		return fail(err)
	}
	if err := s.repo.MarkJobCompleted(ctx, job.ID, cutoutKey, w, h, cutout.AspectRatio, cutout.DominantColors, cutout.EngineUsed); err != nil {
		return fail(err)
	}

	rec := domainstudio.RecommendPresets(job.CategorySlug, job.CategoryName, cutout.AspectRatio, cutout.DominantColors)
	if err := s.generateInitialVariations(ctx, actorID, job.ID, cutout.CutoutPNG, rec.RecommendedPresets); err != nil {
		return fail(err)
	}
	if err := s.repo.UpdateJobStatus(ctx, actorID, job.ID, domainstudio.JobStatusPendingApproval); err != nil {
		return fail(err)
	}
	return s.repo.GetJob(ctx, actorID, job.ID)
}

func (s *Service) generateInitialVariations(ctx context.Context, actorID, jobID int64, cutoutPNG []byte, recommended []string) error {
	seen := make(map[string]bool, 5)
	ordered := make([]string, 0, 5)
	// Always include pure-white plus top category recommendations (up to 4 unique variations).
	ordered = append(ordered, "pure-white")
	seen["pure-white"] = true
	for _, slug := range recommended {
		if !seen[slug] && len(ordered) < 4 {
			ordered = append(ordered, slug)
			seen[slug] = true
		}
	}
	recSet := make(map[string]bool, len(recommended))
	for _, slug := range recommended {
		recSet[slug] = true
	}

	defaults := domainstudio.DefaultComposition()
	for _, slug := range ordered {
		preset, ok := domainstudio.FindPreset(slug)
		if !ok {
			continue
		}
		rendered, err := s.processor.Compose(ctx, cutoutPNG, preset.Slug, defaults)
		if err != nil {
			return err
		}
		key, err := randomStudioKey(actorID, ".jpg")
		if err != nil {
			return err
		}
		if err := s.storage.PutObject(ctx, key, rendered.ContentType, int64(len(rendered.Bytes)), bytes.NewReader(rendered.Bytes)); err != nil {
			return err
		}
		if _, err := s.repo.CreateVariation(ctx, domainstudio.Variation{
			JobID:               jobID,
			PresetSlug:          preset.Slug,
			PresetLabel:         preset.Label,
			CategoryRecommended: recSet[preset.Slug],
			ObjectKey:           key,
			ContentType:         rendered.ContentType,
			ByteSize:            int64(len(rendered.Bytes)),
			Width:               rendered.Width,
			Height:              rendered.Height,
			Composition:         defaults,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ComposeVariation(ctx context.Context, actorID, jobID int64, presetSlug string, params domainstudio.CompositionParams) (domainstudio.Variation, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || s.storage == nil || s.processor == nil {
		return domainstudio.Variation{}, ports.ErrForbidden
	}
	preset, ok := domainstudio.FindPreset(presetSlug)
	if !ok {
		return domainstudio.Variation{}, ErrInvalidPreset
	}
	job, err := s.repo.GetJob(ctx, actorID, jobID)
	if err != nil {
		return domainstudio.Variation{}, err
	}
	if job.CutoutObjectKey == "" {
		return domainstudio.Variation{}, ErrJobNotReady
	}
	cutoutPNG, _, err := s.readObjectBounded(ctx, job.CutoutObjectKey, domainstudio.MaxUploadBytes)
	if err != nil {
		return domainstudio.Variation{}, err
	}
	params = params.Normalize()
	rendered, err := s.processor.Compose(ctx, cutoutPNG, preset.Slug, params)
	if err != nil {
		return domainstudio.Variation{}, err
	}
	key, err := randomStudioKey(actorID, ".jpg")
	if err != nil {
		return domainstudio.Variation{}, err
	}
	if err := s.storage.PutObject(ctx, key, rendered.ContentType, int64(len(rendered.Bytes)), bytes.NewReader(rendered.Bytes)); err != nil {
		return domainstudio.Variation{}, err
	}

	// Update existing variation with matching preset if not already approved, or create a new one.
	for _, existing := range job.Variations {
		if existing.PresetSlug == preset.Slug && existing.Status != domainstudio.VariationStatusApproved {
			oldKey := existing.ObjectKey
			updated, upErr := s.repo.UpdateVariationRender(ctx, existing.ID, key, rendered.ContentType, int64(len(rendered.Bytes)), rendered.Width, rendered.Height, params)
			if upErr != nil {
				_ = s.storage.DeleteObject(ctx, key)
				return domainstudio.Variation{}, upErr
			}
			if oldKey != "" && oldKey != key {
				_ = s.storage.DeleteObject(ctx, oldKey)
			}
			_ = s.repo.UpdateJobStatus(ctx, actorID, job.ID, domainstudio.JobStatusPendingApproval)
			return updated, nil
		}
	}

	rec := false
	for _, slug := range job.Recommendation.RecommendedPresets {
		if slug == preset.Slug {
			rec = true
			break
		}
	}
	created, err := s.repo.CreateVariation(ctx, domainstudio.Variation{
		JobID:               job.ID,
		PresetSlug:          preset.Slug,
		PresetLabel:         preset.Label,
		CategoryRecommended: rec,
		ObjectKey:           key,
		ContentType:         rendered.ContentType,
		ByteSize:            int64(len(rendered.Bytes)),
		Width:               rendered.Width,
		Height:              rendered.Height,
		Composition:         params,
	})
	if err != nil {
		_ = s.storage.DeleteObject(ctx, key)
		return domainstudio.Variation{}, err
	}
	_ = s.repo.UpdateJobStatus(ctx, actorID, job.ID, domainstudio.JobStatusPendingApproval)
	return created, nil
}

func (s *Service) RegenerateVariations(ctx context.Context, actorID, jobID int64) (domainstudio.Job, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || s.storage == nil || s.processor == nil {
		return domainstudio.Job{}, ports.ErrForbidden
	}
	job, err := s.repo.GetJob(ctx, actorID, jobID)
	if err != nil {
		return domainstudio.Job{}, err
	}
	if job.CutoutObjectKey == "" {
		return domainstudio.Job{}, ErrJobNotReady
	}
	cutoutPNG, _, err := s.readObjectBounded(ctx, job.CutoutObjectKey, domainstudio.MaxUploadBytes)
	if err != nil {
		return domainstudio.Job{}, err
	}
	if err := s.generateInitialVariations(ctx, actorID, job.ID, cutoutPNG, job.Recommendation.RecommendedPresets); err != nil {
		return domainstudio.Job{}, err
	}
	if err := s.repo.UpdateJobStatus(ctx, actorID, job.ID, domainstudio.JobStatusPendingApproval); err != nil {
		return domainstudio.Job{}, err
	}
	return s.repo.GetJob(ctx, actorID, job.ID)
}

// UpdateCutout stores a seller-refined transparent PNG mask (never altering the
// preserved source photograph) and re-renders pending variations to match.
func (s *Service) UpdateCutout(ctx context.Context, actorID, jobID int64, pngData []byte) (domainstudio.Job, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || s.storage == nil || s.processor == nil {
		return domainstudio.Job{}, ports.ErrForbidden
	}
	job, err := s.repo.GetJob(ctx, actorID, jobID)
	if err != nil {
		return domainstudio.Job{}, err
	}
	cut, err := s.processor.SanitizeCutout(pngData)
	if err != nil {
		return domainstudio.Job{}, err
	}
	newKey, err := randomStudioKey(actorID, ".png")
	if err != nil {
		return domainstudio.Job{}, err
	}
	if err := s.storage.PutObject(ctx, newKey, "image/png", int64(len(cut.CutoutPNG)), bytes.NewReader(cut.CutoutPNG)); err != nil {
		return domainstudio.Job{}, err
	}
	engineLabel := job.EngineUsed
	if engineLabel == "" {
		engineLabel = cut.EngineUsed
	}
	oldCutoutKey := job.CutoutObjectKey
	if err := s.repo.MarkJobCompleted(ctx, job.ID, newKey, cut.Width, cut.Height, cut.AspectRatio, cut.DominantColors, engineLabel); err != nil {
		_ = s.storage.DeleteObject(ctx, newKey)
		return domainstudio.Job{}, err
	}
	if oldCutoutKey != "" && oldCutoutKey != newKey {
		_ = s.storage.DeleteObject(ctx, oldCutoutKey)
	}

	reRendered := false
	for _, existing := range job.Variations {
		if existing.Status == domainstudio.VariationStatusApproved {
			continue
		}
		rendered, rErr := s.processor.Compose(ctx, cut.CutoutPNG, existing.PresetSlug, existing.Composition)
		if rErr != nil {
			return domainstudio.Job{}, rErr
		}
		vKey, kErr := randomStudioKey(actorID, ".jpg")
		if kErr != nil {
			return domainstudio.Job{}, kErr
		}
		if pErr := s.storage.PutObject(ctx, vKey, rendered.ContentType, int64(len(rendered.Bytes)), bytes.NewReader(rendered.Bytes)); pErr != nil {
			return domainstudio.Job{}, pErr
		}
		if _, uErr := s.repo.UpdateVariationRender(ctx, existing.ID, vKey, rendered.ContentType, int64(len(rendered.Bytes)), rendered.Width, rendered.Height, existing.Composition); uErr != nil {
			_ = s.storage.DeleteObject(ctx, vKey)
			return domainstudio.Job{}, uErr
		}
		if existing.ObjectKey != "" && existing.ObjectKey != vKey {
			_ = s.storage.DeleteObject(ctx, existing.ObjectKey)
		}
		reRendered = true
	}
	if !reRendered {
		rec := domainstudio.RecommendPresets(job.CategorySlug, job.CategoryName, cut.AspectRatio, cut.DominantColors)
		if err := s.generateInitialVariations(ctx, actorID, job.ID, cut.CutoutPNG, rec.RecommendedPresets); err != nil {
			return domainstudio.Job{}, err
		}
	}
	if err := s.repo.UpdateJobStatus(ctx, actorID, job.ID, domainstudio.JobStatusPendingApproval); err != nil {
		return domainstudio.Job{}, err
	}
	return s.repo.GetJob(ctx, actorID, job.ID)
}

func (s *Service) RejectVariation(ctx context.Context, actorID, variationID int64) (domainstudio.Variation, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil {
		return domainstudio.Variation{}, ports.ErrForbidden
	}
	return s.repo.SetVariationStatus(ctx, actorID, variationID, domainstudio.VariationStatusRejected, nil)
}

func (s *Service) RejectJob(ctx context.Context, actorID, jobID int64) error {
	if !s.canManage(ctx, actorID) || s.repo == nil {
		return ports.ErrForbidden
	}
	return s.repo.UpdateJobStatus(ctx, actorID, jobID, domainstudio.JobStatusRejected)
}

func (s *Service) ApproveVariation(ctx context.Context, actorID, variationID int64) (domainstudio.Variation, domaincommerce.MediaRecord, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || s.storage == nil || s.media == nil {
		return domainstudio.Variation{}, domaincommerce.MediaRecord{}, ports.ErrForbidden
	}
	variation, job, err := s.repo.GetVariation(ctx, actorID, variationID)
	if err != nil {
		return domainstudio.Variation{}, domaincommerce.MediaRecord{}, err
	}
	if variation.Status == domainstudio.VariationStatusApproved && variation.PublishedMediaID != nil {
		return variation, domaincommerce.MediaRecord{ID: *variation.PublishedMediaID, ProductID: job.ProductID, PublicPath: fmt.Sprintf("/media/products/%d", *variation.PublishedMediaID)}, nil
	}
	data, info, err := s.readObjectBounded(ctx, variation.ObjectKey, domainstudio.MaxUploadBytes)
	if err != nil {
		return domainstudio.Variation{}, domaincommerce.MediaRecord{}, err
	}

	// Promote through the existing MediaService lifecycle so product_media,
	// products.image_url, and listing_change_reviews rules are respected.
	upload, err := s.media.PrepareUpload(ctx, actorID, job.ProductID, info.ContentType, int64(len(data)))
	if err != nil {
		return domainstudio.Variation{}, domaincommerce.MediaRecord{}, err
	}
	if err := s.media.UploadObject(ctx, actorID, upload.ID, info.ContentType, int64(len(data)), bytes.NewReader(data)); err != nil {
		return domainstudio.Variation{}, domaincommerce.MediaRecord{}, err
	}
	mediaRecord, err := s.media.FinalizeUpload(ctx, actorID, upload.ID)
	if err != nil {
		return domainstudio.Variation{}, domaincommerce.MediaRecord{}, err
	}

	updated, err := s.repo.SetVariationStatus(ctx, actorID, variation.ID, domainstudio.VariationStatusApproved, &mediaRecord.ID)
	if err != nil {
		return domainstudio.Variation{}, domaincommerce.MediaRecord{}, err
	}
	_ = s.repo.UpdateJobStatus(ctx, actorID, job.ID, domainstudio.JobStatusApproved)
	return updated, mediaRecord, nil
}

func (s *Service) OpenSource(ctx context.Context, actorID, jobID int64) (io.ReadCloser, ports.ObjectInfo, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || s.storage == nil {
		return nil, ports.ObjectInfo{}, ports.ErrForbidden
	}
	job, err := s.repo.GetJob(ctx, actorID, jobID)
	if err != nil {
		return nil, ports.ObjectInfo{}, err
	}
	return s.storage.OpenObject(ctx, job.SourceObjectKey)
}

func (s *Service) OpenCutout(ctx context.Context, actorID, jobID int64) (io.ReadCloser, ports.ObjectInfo, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || s.storage == nil {
		return nil, ports.ObjectInfo{}, ports.ErrForbidden
	}
	job, err := s.repo.GetJob(ctx, actorID, jobID)
	if err != nil {
		return nil, ports.ObjectInfo{}, err
	}
	if job.CutoutObjectKey == "" {
		return nil, ports.ObjectInfo{}, ports.ErrNotFound
	}
	return s.storage.OpenObject(ctx, job.CutoutObjectKey)
}

func (s *Service) OpenVariation(ctx context.Context, actorID, variationID int64) (io.ReadCloser, ports.ObjectInfo, error) {
	if !s.canManage(ctx, actorID) || s.repo == nil || s.storage == nil {
		return nil, ports.ObjectInfo{}, ports.ErrForbidden
	}
	v, _, err := s.repo.GetVariation(ctx, actorID, variationID)
	if err != nil {
		return nil, ports.ObjectInfo{}, err
	}
	return s.storage.OpenObject(ctx, v.ObjectKey)
}

func (s *Service) readObjectBounded(ctx context.Context, key string, limit int64) ([]byte, ports.ObjectInfo, error) {
	body, info, err := s.storage.OpenObject(ctx, key)
	if err != nil {
		return nil, ports.ObjectInfo{}, err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, ports.ObjectInfo{}, err
	}
	if int64(len(data)) > limit || len(data) == 0 {
		return nil, ports.ObjectInfo{}, ErrStudioUnavailable
	}
	return data, info, nil
}

func (s *Service) canManage(ctx context.Context, userID int64) bool {
	if userID <= 0 {
		return false
	}
	if s.roles == nil {
		return true
	}
	allowed, err := s.roles.HasAnyRole(ctx, userID, domainidentity.RoleAdmin, domainidentity.RoleSuperAdmin, domainidentity.RoleSeller)
	return err == nil && allowed
}

func randomStudioKey(actorID int64, ext string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("studio/%d/%s%s", actorID, hex.EncodeToString(buf), ext), nil
}

func mimeExt(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

func classifyAspect(w, h int) string {
	if w <= 0 || h <= 0 {
		return "square"
	}
	r := float64(w) / float64(h)
	switch {
	case r < 0.85:
		return "portrait"
	case r > 1.18:
		return "landscape"
	default:
		return "square"
	}
}
