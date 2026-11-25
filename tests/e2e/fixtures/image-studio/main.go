package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	engineadapter "github.com/wecratfs/commerce/internal/adapters/imagestudio"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	applicationimagestudio "github.com/wecratfs/commerce/internal/application/imagestudio"
	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
	"github.com/wecratfs/commerce/internal/ports"
	apiTransport "github.com/wecratfs/commerce/internal/transport/api"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/assets"
	sellerpages "github.com/wecratfs/commerce/web/selleradmin"
)

// Self-contained Playwright fixture server rendering the real templ UI,
// real CSS/JS assets, real API transport, and real local AI Image Studio engine.
type memStore struct {
	mu   sync.RWMutex
	objs map[string]memObj
}
type memObj struct {
	data []byte
	ct   string
}

func (s *memStore) PutObject(_ context.Context, k, ct string, _ int64, body io.Reader) error {
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.objs[k] = memObj{data: raw, ct: ct}
	s.mu.Unlock()
	return nil
}
func (s *memStore) OpenObject(_ context.Context, k string) (io.ReadCloser, ports.ObjectInfo, error) {
	s.mu.RLock()
	v, ok := s.objs[k]
	s.mu.RUnlock()
	if !ok {
		return nil, ports.ObjectInfo{}, ports.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(v.data)), ports.ObjectInfo{ContentType: v.ct, ByteSize: int64(len(v.data))}, nil
}
func (s *memStore) DeleteObject(_ context.Context, k string) error {
	s.mu.Lock()
	delete(s.objs, k)
	s.mu.Unlock()
	return nil
}

type memSessionStore struct {
	sess ports.SessionRecord
}

func (s memSessionStore) Create(context.Context, ports.SessionRecord) error { return nil }
func (s memSessionStore) Get(context.Context, string) (ports.SessionRecord, error) {
	return s.sess, nil
}
func (s memSessionStore) Delete(context.Context, string) error              { return nil }
func (s memSessionStore) DeleteAll(context.Context, int64) error            { return nil }
func (s memSessionStore) List(context.Context, int64) ([]ports.SessionRecord, error) {
	return nil, nil
}

type memStudioRepo struct {
	mu      sync.RWMutex
	nextJob int64
	nextVar int64
	jobs    map[int64]domainstudio.Job
	vars    map[int64]domainstudio.Variation
	product domainstudio.ProductOption
}

func (r *memStudioRepo) ListSellerProducts(context.Context, int64) ([]domainstudio.ProductOption, error) {
	return []domainstudio.ProductOption{r.product}, nil
}
func (r *memStudioRepo) GetOwnedProduct(_ context.Context, _, pid int64) (domainstudio.ProductOption, error) {
	if pid != r.product.ID {
		return domainstudio.ProductOption{}, ports.ErrForbidden
	}
	return r.product, nil
}
func (r *memStudioRepo) CreateJob(_ context.Context, job domainstudio.Job) (domainstudio.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextJob++
	job.ID = r.nextJob
	job.SellerID = 1
	job.SourcePreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/jobs/%d/source", job.ID)
	r.jobs[job.ID] = job
	return job, nil
}
func (r *memStudioRepo) GetJob(_ context.Context, _, jobID int64) (domainstudio.Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[jobID]
	if !ok {
		return domainstudio.Job{}, ports.ErrNotFound
	}
	j.Variations = nil
	for _, v := range r.vars {
		if v.JobID == jobID {
			j.Variations = append(j.Variations, v)
		}
	}
	j.Recommendation = domainstudio.RecommendPresets(j.CategorySlug, j.CategoryName, j.AspectRatio, j.DominantColors)
	return j, nil
}
func (r *memStudioRepo) ListProductJobs(_ context.Context, ownerID, pid int64, _ int) ([]domainstudio.Job, error) {
	r.mu.RLock()
	var ids []int64
	for id, j := range r.jobs {
		if j.ProductID == pid {
			ids = append(ids, id)
		}
	}
	r.mu.RUnlock()
	var out []domainstudio.Job
	for _, id := range ids {
		j, _ := r.GetJob(context.Background(), ownerID, id)
		out = append(out, j)
	}
	return out, nil
}
func (r *memStudioRepo) MarkJobProcessing(_ context.Context, jobID int64) error {
	r.mu.Lock()
	j := r.jobs[jobID]
	j.Status = domainstudio.JobStatusProcessing
	r.jobs[jobID] = j
	r.mu.Unlock()
	return nil
}
func (r *memStudioRepo) MarkJobCompleted(_ context.Context, jobID int64, key string, w, h int, ar string, cols []domainstudio.DominantColor, eng string) error {
	r.mu.Lock()
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
	r.mu.Unlock()
	return nil
}
func (r *memStudioRepo) MarkJobFailed(_ context.Context, jobID int64, msg string) error {
	r.mu.Lock()
	j := r.jobs[jobID]
	j.Status = domainstudio.JobStatusFailed
	j.ErrorMessage = msg
	r.jobs[jobID] = j
	r.mu.Unlock()
	return nil
}
func (r *memStudioRepo) UpdateJobStatus(_ context.Context, _, jobID int64, status string) error {
	r.mu.Lock()
	j := r.jobs[jobID]
	j.Status = status
	r.jobs[jobID] = j
	r.mu.Unlock()
	return nil
}
func (r *memStudioRepo) CreateVariation(_ context.Context, v domainstudio.Variation) (domainstudio.Variation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextVar++
	v.ID = r.nextVar
	v.Status = domainstudio.VariationStatusPendingApproval
	v.PreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/variations/%d/preview", v.ID)
	r.vars[v.ID] = v
	return v, nil
}
func (r *memStudioRepo) GetVariation(_ context.Context, ownerID, varID int64) (domainstudio.Variation, domainstudio.Job, error) {
	r.mu.RLock()
	v, ok := r.vars[varID]
	r.mu.RUnlock()
	if !ok {
		return domainstudio.Variation{}, domainstudio.Job{}, ports.ErrNotFound
	}
	j, err := r.GetJob(context.Background(), ownerID, v.JobID)
	return v, j, err
}
func (r *memStudioRepo) UpdateVariationRender(_ context.Context, varID int64, key, ct string, sz int64, w, h int, params domainstudio.CompositionParams) (domainstudio.Variation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.vars[varID]
	v.ObjectKey = key
	v.ContentType = ct
	v.ByteSize = sz
	v.Width = w
	v.Height = h
	v.Composition = params
	v.PreviewURL = fmt.Sprintf("/api/v1/seller/image-studio/variations/%d/preview?v=%d", v.ID, time.Now().UnixNano())
	r.vars[varID] = v
	return v, nil
}
func (r *memStudioRepo) SetVariationStatus(_ context.Context, _, varID int64, status string, mediaID *int64) (domainstudio.Variation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.vars[varID]
	v.Status = status
	v.PublishedMediaID = mediaID
	r.vars[varID] = v
	return v, nil
}

type fixtureMediaPub struct{ next int64 }

func (f *fixtureMediaPub) PrepareUpload(_ context.Context, _, pid int64, ct string, sz int64) (domaincommerce.MediaUpload, error) {
	f.next++
	return domaincommerce.MediaUpload{ID: f.next, ProductID: pid, ContentType: ct, MaxBytes: sz}, nil
}
func (f *fixtureMediaPub) UploadObject(context.Context, int64, int64, string, int64, io.Reader) error {
	return nil
}
func (f *fixtureMediaPub) FinalizeUpload(_ context.Context, _, mid int64) (domaincommerce.MediaRecord, error) {
	return domaincommerce.MediaRecord{ID: mid, ProductID: 12, Status: "ready", PublicPath: fmt.Sprintf("/media/products/%d", mid)}, nil
}

type fixtureRoles struct{}

func (fixtureRoles) HasAnyRole(context.Context, int64, ...domainidentity.Role) (bool, error) {
	return true, nil
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:18190"
	}
	modelPath := filepath.Join("models", "isnet-general-use.onnx")
	if _, err := os.Stat(modelPath); err != nil {
		modelPath = filepath.Join("models", "u2netp.onnx")
	}
	engine := engineadapter.NewEngine(modelPath, "", 1, 2, 45*time.Second)
	defer engine.Close()

	store := &memStore{objs: make(map[string]memObj)}
	repo := &memStudioRepo{
		jobs:    make(map[int64]domainstudio.Job),
		vars:    make(map[int64]domainstudio.Variation),
		product: domainstudio.ProductOption{ID: 12, Name: "Brass Elephant", SKU: "BR-12", CategorySlug: "brass-metalwork", CategoryName: "Brass & Metalwork", Status: "draft"},
	}
	svc := applicationimagestudio.NewService(repo, store, engine, &fixtureMediaPub{}, fixtureRoles{})
	sess := ports.SessionRecord{ID: "fixture-sess", UserID: 7, CSRFToken: "fixture-csrf", ExpiresAt: time.Now().Add(time.Hour)}
	sessions := applicationauth.NewSessionService(memSessionStore{sess: sess})
	apiHandler := apiTransport.NewFullHandler(nil, nil, nil, sessions, nil, nil).WithImageStudio(svc)

	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets.FS))))
	mux.HandleFunc("/api/v1/seller/image-studio/", apiHandler.ImageStudio)
	mux.HandleFunc("/seller-admin/image-studio", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "wecratfs_session", Value: sess.ID, Path: "/"})
		page := viewmodels.SellerAdminPage{
			Active:          "image-studio",
			Title:           "AI Image Studio",
			Subtitle:        "Remove backgrounds locally, preview studio scenes, adjust framing, and save approved photos to your product gallery.",
			DisplayName:     "Asha Rao",
			RoleLabel:       "Seller",
			Initials:        "AR",
			SellerName:      "Asha's Studio",
			SellerStatus:    "active",
			UserEmail:       "asha@weevcrafts.test",
			CSRFToken:       sess.CSRFToken,
			CanProducts:     true,
			CanProductWrite: true,
			StudioProductID: "12",
			StudioProducts:  []domainstudio.ProductOption{repo.product},
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = sellerpages.PortalImageStudio(page).Render(r.Context(), w)
	})

	log.Printf("image-studio fixture listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
