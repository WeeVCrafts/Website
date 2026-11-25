package imagestudio

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	engineadapter "github.com/wecratfs/commerce/internal/adapters/imagestudio"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
	"github.com/wecratfs/commerce/internal/ports"
)

type memoryStudioStorage struct {
	objects map[string]storedObj
}

type storedObj struct {
	data        []byte
	contentType string
}

func newMemoryStudioStorage() *memoryStudioStorage {
	return &memoryStudioStorage{objects: make(map[string]storedObj)}
}

func (s *memoryStudioStorage) PutObject(_ context.Context, key, contentType string, byteSize int64, body io.Reader) error {
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if int64(len(raw)) != byteSize {
		return errors.New("size mismatch")
	}
	s.objects[key] = storedObj{data: raw, contentType: contentType}
	return nil
}

func (s *memoryStudioStorage) OpenObject(_ context.Context, key string) (io.ReadCloser, ports.ObjectInfo, error) {
	obj, ok := s.objects[key]
	if !ok {
		return nil, ports.ObjectInfo{}, ports.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(obj.data)), ports.ObjectInfo{ContentType: obj.contentType, ByteSize: int64(len(obj.data))}, nil
}

func (s *memoryStudioStorage) DeleteObject(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

type memoryStudioRepo struct {
	products   map[int64]domainstudio.ProductOption
	owners     map[int64]int64
	jobs       map[int64]domainstudio.Job
	variations map[int64]domainstudio.Variation
	nextJobID  int64
	nextVarID  int64
}

func newMemoryStudioRepo() *memoryStudioRepo {
	return &memoryStudioRepo{
		products:   map[int64]domainstudio.ProductOption{10: {ID: 10, Name: "Brass Elephant", SKU: "BR-10", CategorySlug: "brass-metalwork", CategoryName: "Brass & Metalwork", Status: "draft"}},
		owners:     map[int64]int64{10: 7},
		jobs:       make(map[int64]domainstudio.Job),
		variations: make(map[int64]domainstudio.Variation),
	}
}

func (r *memoryStudioRepo) ListSellerProducts(_ context.Context, ownerID int64) ([]domainstudio.ProductOption, error) {
	var out []domainstudio.ProductOption
	for id, p := range r.products {
		if r.owners[id] == ownerID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *memoryStudioRepo) GetOwnedProduct(_ context.Context, ownerID, productID int64) (domainstudio.ProductOption, error) {
	if r.owners[productID] != ownerID {
		return domainstudio.ProductOption{}, ports.ErrForbidden
	}
	return r.products[productID], nil
}

func (r *memoryStudioRepo) CreateJob(_ context.Context, job domainstudio.Job) (domainstudio.Job, error) {
	if r.owners[job.ProductID] != job.OwnerUserID {
		return domainstudio.Job{}, ports.ErrForbidden
	}
	r.nextJobID++
	job.ID = r.nextJobID
	job.SellerID = 100
	job.CreatedAt = time.Now()
	job.UpdatedAt = job.CreatedAt
	r.jobs[job.ID] = job
	return job, nil
}

func (r *memoryStudioRepo) GetJob(_ context.Context, ownerID, jobID int64) (domainstudio.Job, error) {
	job, ok := r.jobs[jobID]
	if !ok || r.owners[job.ProductID] != ownerID {
		return domainstudio.Job{}, ports.ErrForbidden
	}
	var vars []domainstudio.Variation
	for _, v := range r.variations {
		if v.JobID == jobID {
			vars = append(vars, v)
		}
	}
	job.Variations = vars
	job.Recommendation = domainstudio.RecommendPresets(job.CategorySlug, job.CategoryName, job.AspectRatio, job.DominantColors)
	return job, nil
}

func (r *memoryStudioRepo) ListProductJobs(_ context.Context, ownerID, productID int64, _ int) ([]domainstudio.Job, error) {
	if r.owners[productID] != ownerID {
		return nil, ports.ErrForbidden
	}
	var out []domainstudio.Job
	for id, j := range r.jobs {
		if j.ProductID == productID {
			full, _ := r.GetJob(context.Background(), ownerID, id)
			out = append(out, full)
		}
	}
	return out, nil
}

func (r *memoryStudioRepo) MarkJobProcessing(_ context.Context, jobID int64) error {
	j := r.jobs[jobID]
	j.Status = domainstudio.JobStatusProcessing
	r.jobs[jobID] = j
	return nil
}

func (r *memoryStudioRepo) MarkJobCompleted(_ context.Context, jobID int64, cutoutKey string, width, height int, aspectRatio string, colors []domainstudio.DominantColor, engine string) error {
	j := r.jobs[jobID]
	j.CutoutObjectKey = cutoutKey
	j.SourceWidth = width
	j.SourceHeight = height
	j.AspectRatio = aspectRatio
	j.DominantColors = colors
	j.EngineUsed = engine
	j.Status = domainstudio.JobStatusCompleted
	r.jobs[jobID] = j
	return nil
}

func (r *memoryStudioRepo) MarkJobFailed(_ context.Context, jobID int64, errMsg string) error {
	j := r.jobs[jobID]
	j.Status = domainstudio.JobStatusFailed
	j.ErrorMessage = errMsg
	r.jobs[jobID] = j
	return nil
}

func (r *memoryStudioRepo) UpdateJobStatus(_ context.Context, ownerID, jobID int64, status string) error {
	j, ok := r.jobs[jobID]
	if !ok || r.owners[j.ProductID] != ownerID {
		return ports.ErrForbidden
	}
	j.Status = status
	r.jobs[jobID] = j
	return nil
}

func (r *memoryStudioRepo) CreateVariation(_ context.Context, v domainstudio.Variation) (domainstudio.Variation, error) {
	r.nextVarID++
	v.ID = r.nextVarID
	v.Status = domainstudio.VariationStatusPendingApproval
	v.CreatedAt = time.Now()
	v.UpdatedAt = v.CreatedAt
	r.variations[v.ID] = v
	return v, nil
}

func (r *memoryStudioRepo) GetVariation(_ context.Context, ownerID, variationID int64) (domainstudio.Variation, domainstudio.Job, error) {
	v, ok := r.variations[variationID]
	if !ok {
		return domainstudio.Variation{}, domainstudio.Job{}, ports.ErrForbidden
	}
	j, err := r.GetJob(context.Background(), ownerID, v.JobID)
	if err != nil {
		return domainstudio.Variation{}, domainstudio.Job{}, err
	}
	return v, j, nil
}

func (r *memoryStudioRepo) UpdateVariationRender(_ context.Context, variationID int64, objectKey, contentType string, byteSize int64, width, height int, params domainstudio.CompositionParams) (domainstudio.Variation, error) {
	v := r.variations[variationID]
	v.ObjectKey = objectKey
	v.ContentType = contentType
	v.ByteSize = byteSize
	v.Width = width
	v.Height = height
	v.Composition = params
	v.Status = domainstudio.VariationStatusPendingApproval
	r.variations[variationID] = v
	return v, nil
}

func (r *memoryStudioRepo) SetVariationStatus(_ context.Context, ownerID, variationID int64, status string, publishedMediaID *int64) (domainstudio.Variation, error) {
	v, ok := r.variations[variationID]
	if !ok {
		return domainstudio.Variation{}, ports.ErrForbidden
	}
	j := r.jobs[v.JobID]
	if r.owners[j.ProductID] != ownerID {
		return domainstudio.Variation{}, ports.ErrForbidden
	}
	v.Status = status
	if publishedMediaID != nil {
		v.PublishedMediaID = publishedMediaID
	}
	r.variations[variationID] = v
	return v, nil
}

type fakeMediaPublisher struct {
	nextID   int64
	uploaded map[int64][]byte
}

func (f *fakeMediaPublisher) PrepareUpload(_ context.Context, _, productID int64, contentType string, byteSize int64) (domaincommerce.MediaUpload, error) {
	f.nextID++
	return domaincommerce.MediaUpload{ID: f.nextID, ProductID: productID, ContentType: contentType, MaxBytes: byteSize}, nil
}

func (f *fakeMediaPublisher) UploadObject(_ context.Context, _, mediaID int64, _ string, _ int64, body io.Reader) error {
	if f.uploaded == nil {
		f.uploaded = make(map[int64][]byte)
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.uploaded[mediaID] = raw
	return nil
}

func (f *fakeMediaPublisher) FinalizeUpload(_ context.Context, _, mediaID int64) (domaincommerce.MediaRecord, error) {
	return domaincommerce.MediaRecord{ID: mediaID, Status: "ready", PublicPath: "/media/products/1"}, nil
}

type fakeRoleChecker struct {
	allowedUsers map[int64]bool
}

func (f fakeRoleChecker) HasAnyRole(_ context.Context, userID int64, _ ...domainidentity.Role) (bool, error) {
	return f.allowedUsers[userID], nil
}

func samplePNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 260, 260))
	for y := 0; y < 260; y++ {
		for x := 0; x < 260; x++ {
			img.SetRGBA(x, y, color.RGBA{244, 244, 242, 255})
		}
	}
	for y := 70; y < 190; y++ {
		for x := 70; x < 190; x++ {
			img.SetRGBA(x, y, color.RGBA{185, 130, 50, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestStudioUploadComposeApproveAndOwnershipChecks(t *testing.T) {
	repo := newMemoryStudioRepo()
	store := newMemoryStudioStorage()
	engine := engineadapter.NewEngine("", "", 1, 2, 10*time.Second)
	defer engine.Close()
	publisher := &fakeMediaPublisher{}
	roles := fakeRoleChecker{allowedUsers: map[int64]bool{7: true, 99: true}}

	svc := NewService(repo, store, engine, publisher, roles)
	raw := samplePNGBytes(t)

	// Non-owner seller cannot upload for product 10.
	if _, err := svc.UploadAndStart(context.Background(), 99, 10, "image/png", raw); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for non-owner seller, got %v", err)
	}

	job, err := svc.UploadAndStart(context.Background(), 7, 10, "image/png", raw)
	if err != nil {
		t.Fatalf("UploadAndStart failed: %v", err)
	}
	if job.Status != domainstudio.JobStatusPendingApproval {
		t.Fatalf("expected status pending_approval, got %q", job.Status)
	}
	if len(job.Variations) < 3 {
		t.Fatalf("expected multiple generated variations, got %d", len(job.Variations))
	}
	// Original source bytes must remain preserved and unaltered in storage.
	if !bytes.Equal(store.objects[job.SourceObjectKey].data, raw) {
		t.Fatal("original source object was modified or missing")
	}

	// Non-owner cannot view or approve variations.
	targetVar := job.Variations[0]
	if _, _, err := svc.ApproveVariation(context.Background(), 99, targetVar.ID); !errors.Is(err, ports.ErrForbidden) {
		t.Fatalf("expected ErrForbidden on non-owner approve, got %v", err)
	}

	// Re-compose with manual coordinates.
	edited, err := svc.ComposeVariation(context.Background(), 7, job.ID, "luxury-studio", domainstudio.CompositionParams{
		OffsetX:        0.10,
		OffsetY:        -0.08,
		Scale:          0.90,
		FitMode:        domainstudio.FitContain,
		ShadowStrength: 0.5,
	})
	if err != nil {
		t.Fatalf("ComposeVariation failed: %v", err)
	}
	if edited.PresetSlug != "luxury-studio" || edited.Composition.Scale != 0.90 {
		t.Fatalf("unexpected edited variation: %+v", edited)
	}

	// Reject one variation, approve the edited variation into product gallery.
	rejected, err := svc.RejectVariation(context.Background(), 7, targetVar.ID)
	if err != nil || rejected.Status != domainstudio.VariationStatusRejected {
		t.Fatalf("RejectVariation failed: %v %+v", err, rejected)
	}

	approved, mediaRec, err := svc.ApproveVariation(context.Background(), 7, edited.ID)
	if err != nil {
		t.Fatalf("ApproveVariation failed: %v", err)
	}
	if approved.Status != domainstudio.VariationStatusApproved || approved.PublishedMediaID == nil || *approved.PublishedMediaID != mediaRec.ID {
		t.Fatalf("unexpected approved variation: %+v media=%+v", approved, mediaRec)
	}
	if len(publisher.uploaded[mediaRec.ID]) == 0 || !strings.HasPrefix(mediaRec.PublicPath, "/media/products/") {
		t.Fatalf("expected published gallery bytes, got media=%+v", mediaRec)
	}
	// Original source still intact after approval.
	if !bytes.Equal(store.objects[job.SourceObjectKey].data, raw) {
		t.Fatal("original source overwritten after approval")
	}
}
