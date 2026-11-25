package ports

import (
	"context"
	"image"
	"io"

	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
)

type BackgroundSegmenter interface {
	Segment(ctx context.Context, src image.Image) (mask *image.Alpha, engine string, err error)
}

type ProcessedCutout struct {
	CutoutPNG       []byte
	ContentType     string
	Width           int
	Height          int
	AspectRatio     string
	DominantColors  []domainstudio.DominantColor
	EngineUsed      string
	ForegroundRatio float64
	QualityWarning  string
}

type RenderedComposition struct {
	Bytes       []byte
	ContentType string
	Width       int
	Height      int
}

type ImageProcessor interface {
	ValidateAndPrepare(data []byte, declaredType string) (image.Image, string, int, int, error)
	RemoveBackground(ctx context.Context, src image.Image) (ProcessedCutout, error)
	SanitizeCutout(data []byte) (ProcessedCutout, error)
	Compose(ctx context.Context, cutoutPNG []byte, presetSlug string, params domainstudio.CompositionParams) (RenderedComposition, error)
}

type ImageStudioRepository interface {
	ListSellerProducts(ctx context.Context, ownerUserID int64) ([]domainstudio.ProductOption, error)
	GetOwnedProduct(ctx context.Context, ownerUserID, productID int64) (domainstudio.ProductOption, error)
	CreateJob(ctx context.Context, job domainstudio.Job) (domainstudio.Job, error)
	GetJob(ctx context.Context, ownerUserID, jobID int64) (domainstudio.Job, error)
	ListProductJobs(ctx context.Context, ownerUserID, productID int64, limit int) ([]domainstudio.Job, error)
	MarkJobProcessing(ctx context.Context, jobID int64) error
	MarkJobCompleted(ctx context.Context, jobID int64, cutoutKey string, width, height int, aspectRatio string, colors []domainstudio.DominantColor, engine string) error
	MarkJobFailed(ctx context.Context, jobID int64, errMsg string) error
	UpdateJobStatus(ctx context.Context, ownerUserID, jobID int64, status string) error
	CreateVariation(ctx context.Context, variation domainstudio.Variation) (domainstudio.Variation, error)
	GetVariation(ctx context.Context, ownerUserID, variationID int64) (domainstudio.Variation, domainstudio.Job, error)
	UpdateVariationRender(ctx context.Context, variationID int64, objectKey, contentType string, byteSize int64, width, height int, params domainstudio.CompositionParams) (domainstudio.Variation, error)
	SetVariationStatus(ctx context.Context, ownerUserID, variationID int64, status string, publishedMediaID *int64) (domainstudio.Variation, error)
}

type StudioObjectStorage interface {
	PutObject(ctx context.Context, objectKey, contentType string, byteSize int64, body io.Reader) error
	OpenObject(ctx context.Context, objectKey string) (io.ReadCloser, ObjectInfo, error)
	DeleteObject(ctx context.Context, objectKey string) error
}
