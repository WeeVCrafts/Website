package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	engineadapter "github.com/wecratfs/commerce/internal/adapters/imagestudio"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	applicationimagestudio "github.com/wecratfs/commerce/internal/application/imagestudio"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
	"github.com/wecratfs/commerce/internal/ports"
)

type apiSessionStore struct {
	records map[string]ports.SessionRecord
}

func (s *apiSessionStore) Create(_ context.Context, record ports.SessionRecord) error {
	s.records[record.ID] = record
	return nil
}
func (s *apiSessionStore) Get(_ context.Context, id string) (ports.SessionRecord, error) {
	r, ok := s.records[id]
	if !ok {
		return ports.SessionRecord{}, ports.ErrNotFound
	}
	return r, nil
}
func (s *apiSessionStore) Delete(_ context.Context, id string) error {
	delete(s.records, id)
	return nil
}
func (s *apiSessionStore) List(context.Context, int64) ([]ports.SessionRecord, error) {
	return nil, nil
}
func (s *apiSessionStore) DeleteAll(context.Context, int64) error { return nil }

type apiStudioStorage struct{ objs map[string]apiStored }
type apiStored struct {
	b []byte
	t string
}

func (s *apiStudioStorage) PutObject(_ context.Context, k, t string, sz int64, body io.Reader) error {
	raw, _ := io.ReadAll(body)
	s.objs[k] = apiStored{b: raw, t: t}
	return nil
}
func (s *apiStudioStorage) OpenObject(_ context.Context, k string) (io.ReadCloser, ports.ObjectInfo, error) {
	v, ok := s.objs[k]
	if !ok {
		return nil, ports.ObjectInfo{}, ports.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(v.b)), ports.ObjectInfo{ContentType: v.t, ByteSize: int64(len(v.b))}, nil
}
func (s *apiStudioStorage) DeleteObject(_ context.Context, k string) error {
	delete(s.objs, k)
	return nil
}

type apiStudioRepo struct {
	products   map[int64]domainstudio.ProductOption
	owners     map[int64]int64
	jobs       map[int64]domainstudio.Job
	variations map[int64]domainstudio.Variation
	nextJob    int64
	nextVar    int64
}

func (r *apiStudioRepo) ListSellerProducts(_ context.Context, ownerID int64) ([]domainstudio.ProductOption, error) {
	var out []domainstudio.ProductOption
	for id, p := range r.products {
		if r.owners[id] == ownerID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *apiStudioRepo) GetOwnedProduct(_ context.Context, ownerID, pid int64) (domainstudio.ProductOption, error) {
	if r.owners[pid] != ownerID {
		return domainstudio.ProductOption{}, ports.ErrForbidden
	}
	return r.products[pid], nil
}
func (r *apiStudioRepo) CreateJob(_ context.Context, job domainstudio.Job) (domainstudio.Job, error) {
	r.nextJob++
	job.ID = r.nextJob
	job.SellerID = 1
	job.SourcePreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/jobs/%d/source", job.ID)
	r.jobs[job.ID] = job
	return job, nil
}
func (r *apiStudioRepo) GetJob(_ context.Context, ownerID, jobID int64) (domainstudio.Job, error) {
	j, ok := r.jobs[jobID]
	if !ok || r.owners[j.ProductID] != ownerID {
		return domainstudio.Job{}, ports.ErrForbidden
	}
	j.Variations = nil
	for _, v := range r.variations {
		if v.JobID == jobID {
			j.Variations = append(j.Variations, v)
		}
	}
	j.Recommendation = domainstudio.RecommendPresets(j.CategorySlug, j.CategoryName, j.AspectRatio, j.DominantColors)
	return j, nil
}
func (r *apiStudioRepo) ListProductJobs(_ context.Context, ownerID, pid int64, _ int) ([]domainstudio.Job, error) {
	if r.owners[pid] != ownerID {
		return nil, ports.ErrForbidden
	}
	var out []domainstudio.Job
	for id, j := range r.jobs {
		if j.ProductID == pid {
			full, _ := r.GetJob(context.Background(), ownerID, id)
			out = append(out, full)
		}
	}
	return out, nil
}
func (r *apiStudioRepo) MarkJobProcessing(_ context.Context, jobID int64) error {
	j := r.jobs[jobID]
	j.Status = domainstudio.JobStatusProcessing
	r.jobs[jobID] = j
	return nil
}
func (r *apiStudioRepo) MarkJobCompleted(_ context.Context, jobID int64, key string, w, h int, ar string, cols []domainstudio.DominantColor, eng string) error {
	j := r.jobs[jobID]
	j.CutoutObjectKey = key
	j.CutoutPreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/jobs/%d/cutout", jobID)
	j.SourceWidth = w
	j.SourceHeight = h
	j.AspectRatio = ar
	j.DominantColors = cols
	j.EngineUsed = eng
	j.Status = domainstudio.JobStatusCompleted
	r.jobs[jobID] = j
	return nil
}
func (r *apiStudioRepo) MarkJobFailed(_ context.Context, jobID int64, msg string) error {
	j := r.jobs[jobID]
	j.Status = domainstudio.JobStatusFailed
	j.ErrorMessage = msg
	r.jobs[jobID] = j
	return nil
}
func (r *apiStudioRepo) UpdateJobStatus(_ context.Context, ownerID, jobID int64, status string) error {
	j, ok := r.jobs[jobID]
	if !ok || r.owners[j.ProductID] != ownerID {
		return ports.ErrForbidden
	}
	j.Status = status
	r.jobs[jobID] = j
	return nil
}
func (r *apiStudioRepo) CreateVariation(_ context.Context, v domainstudio.Variation) (domainstudio.Variation, error) {
	r.nextVar++
	v.ID = r.nextVar
	v.Status = domainstudio.VariationStatusPendingApproval
	v.PreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/variations/%d/preview", v.ID)
	r.variations[v.ID] = v
	return v, nil
}
func (r *apiStudioRepo) GetVariation(_ context.Context, ownerID, varID int64) (domainstudio.Variation, domainstudio.Job, error) {
	v, ok := r.variations[varID]
	if !ok {
		return domainstudio.Variation{}, domainstudio.Job{}, ports.ErrForbidden
	}
	j, err := r.GetJob(context.Background(), ownerID, v.JobID)
	if err != nil {
		return domainstudio.Variation{}, domainstudio.Job{}, err
	}
	return v, j, nil
}
func (r *apiStudioRepo) UpdateVariationRender(_ context.Context, varID int64, key, ct string, sz int64, w, h int, params domainstudio.CompositionParams) (domainstudio.Variation, error) {
	v := r.variations[varID]
	v.ObjectKey = key
	v.ContentType = ct
	v.ByteSize = sz
	v.Width = w
	v.Height = h
	v.Composition = params
	v.Status = domainstudio.VariationStatusPendingApproval
	r.variations[varID] = v
	return v, nil
}
func (r *apiStudioRepo) SetVariationStatus(_ context.Context, ownerID, varID int64, status string, mediaID *int64) (domainstudio.Variation, error) {
	v, ok := r.variations[varID]
	if !ok {
		return domainstudio.Variation{}, ports.ErrForbidden
	}
	if r.owners[r.jobs[v.JobID].ProductID] != ownerID {
		return domainstudio.Variation{}, ports.ErrForbidden
	}
	v.Status = status
	v.PublishedMediaID = mediaID
	r.variations[varID] = v
	return v, nil
}

type apiFakePublisher struct{}

func (apiFakePublisher) PrepareUpload(_ context.Context, _, pid int64, ct string, sz int64) (domaincommerce.MediaUpload, error) {
	return domaincommerce.MediaUpload{ID: 77, ProductID: pid, ContentType: ct, MaxBytes: sz}, nil
}
func (apiFakePublisher) UploadObject(context.Context, int64, int64, string, int64, io.Reader) error {
	return nil
}
func (apiFakePublisher) FinalizeUpload(context.Context, int64, int64) (domaincommerce.MediaRecord, error) {
	return domaincommerce.MediaRecord{ID: 77, ProductID: 5, Status: "ready", PublicPath: "/media/products/77"}, nil
}

type apiRoleChecker struct{}

func (apiRoleChecker) HasAnyRole(_ context.Context, userID int64, _ ...domainidentity.Role) (bool, error) {
	return userID == 7 || userID == 8, nil
}

func makeTestPNGBody(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 180, 180))
	for y := 0; y < 180; y++ {
		for x := 0; x < 180; x++ {
			img.SetRGBA(x, y, color.RGBA{245, 245, 243, 255})
		}
	}
	for y := 45; y < 135; y++ {
		for x := 45; x < 135; x++ {
			img.SetRGBA(x, y, color.RGBA{140, 45, 45, 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestImageStudioAPIEndToEndAndCSRFSecurity(t *testing.T) {
	sessions := applicationauth.NewSessionService(&apiSessionStore{records: map[string]ports.SessionRecord{
		"sess-7": {ID: "sess-7", UserID: 7, CSRFToken: "csrf-7", ExpiresAt: time.Now().Add(time.Hour)},
		"sess-8": {ID: "sess-8", UserID: 8, CSRFToken: "csrf-8", ExpiresAt: time.Now().Add(time.Hour)},
	}})
	repo := &apiStudioRepo{
		products:   map[int64]domainstudio.ProductOption{5: {ID: 5, Name: "Saree", CategorySlug: "handloom-sarees", CategoryName: "Handloom Sarees", Status: "draft"}},
		owners:     map[int64]int64{5: 7},
		jobs:       make(map[int64]domainstudio.Job),
		variations: make(map[int64]domainstudio.Variation),
	}
	store := &apiStudioStorage{objs: make(map[string]apiStored)}
	engine := engineadapter.NewEngine("", "", 1, 2, 10*time.Second)
	defer engine.Close()

	studioSvc := applicationimagestudio.NewService(repo, store, engine, apiFakePublisher{}, apiRoleChecker{})
	h := NewFullHandler(nil, nil, nil, sessions, nil, nil).WithImageStudio(studioSvc)

	// Unauthenticated request blocked.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/seller/image-studio/workspace", nil)
	rec := httptest.NewRecorder()
	h.ImageStudio(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	// Upload without CSRF rejected.
	var mp bytes.Buffer
	mw := multipart.NewWriter(&mp)
	_ = mw.WriteField("product_id", "5")
	part, _ := mw.CreateFormFile("image", "saree.png")
	_, _ = part.Write(makeTestPNGBody(t))
	_ = mw.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/v1/seller/image-studio/upload", bytes.NewReader(mp.Bytes()))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: "sess-7"})
	rec = httptest.NewRecorder()
	h.ImageStudio(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF, got %d", rec.Code)
	}

	// Authenticated upload with CSRF succeeds.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/seller/image-studio/upload", bytes.NewReader(mp.Bytes()))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-CSRF-Token", "csrf-7")
	req.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: "sess-7"})
	rec = httptest.NewRecorder()
	h.ImageStudio(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 upload, got %d: %s", rec.Code, rec.Body.String())
	}

	var uploadResp struct {
		Data domainstudio.Job `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatal(err)
	}
	if len(uploadResp.Data.Variations) == 0 {
		t.Fatal("expected variations in upload response")
	}
	varID := uploadResp.Data.Variations[0].ID

	// Another seller (user 8) cannot preview or approve user 7's variation.
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/seller/image-studio/variations/%d/preview", varID), nil)
	req.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: "sess-8"})
	rec = httptest.NewRecorder()
	h.ImageStudio(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-seller preview, got %d", rec.Code)
	}

	// Compose custom positioning.
	composeBody := strings.NewReader(`{"preset_slug":"minimalist","offset_x":0.05,"offset_y":0.02,"scale":0.85,"fit_mode":"contain","shadow_strength":0.4}`)
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/seller/image-studio/jobs/%d/compose", uploadResp.Data.ID), composeBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "csrf-7")
	req.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: "sess-7"})
	rec = httptest.NewRecorder()
	h.ImageStudio(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 compose, got %d: %s", rec.Code, rec.Body.String())
	}

	// Update manual cutout preserves original source object while re-rendering pending variations.
	origSourceKey := uploadResp.Data.SourceObjectKey
	origCutoutKey := uploadResp.Data.CutoutObjectKey
	var cutMP bytes.Buffer
	cutMW := multipart.NewWriter(&cutMP)
	cutPart, _ := cutMW.CreateFormFile("cutout", "cutout.png")
	_, _ = cutPart.Write(makeTestPNGBody(t))
	_ = cutMW.Close()
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/seller/image-studio/jobs/%d/cutout", uploadResp.Data.ID), bytes.NewReader(cutMP.Bytes()))
	req.Header.Set("Content-Type", cutMW.FormDataContentType())
	req.Header.Set("X-CSRF-Token", "csrf-7")
	req.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: "sess-7"})
	rec = httptest.NewRecorder()
	h.ImageStudio(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 cutout update, got %d: %s", rec.Code, rec.Body.String())
	}
	var updatedResp struct {
		Data domainstudio.Job `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &updatedResp); err != nil {
		t.Fatal(err)
	}
	if updatedResp.Data.SourceObjectKey != origSourceKey {
		t.Fatalf("source object key must be preserved: got %q want %q", updatedResp.Data.SourceObjectKey, origSourceKey)
	}
	if updatedResp.Data.CutoutObjectKey == "" || updatedResp.Data.CutoutObjectKey == origCutoutKey {
		t.Fatalf("expected new cutout object key after manual edit, got %q", updatedResp.Data.CutoutObjectKey)
	}

	// Approve variation -> publishes to MediaService.
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/seller/image-studio/variations/%d/approve", varID), nil)
	req.Header.Set("X-CSRF-Token", "csrf-7")
	req.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: "sess-7"})
	rec = httptest.NewRecorder()
	h.ImageStudio(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 approve, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "/media/products/77") {
		t.Fatalf("expected published media path in approve response, got %s", rec.Body.String())
	}
	_ = errors.New("")
}
