package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	engineadapter "github.com/wecratfs/commerce/internal/adapters/imagestudio"
	applicationimagestudio "github.com/wecratfs/commerce/internal/application/imagestudio"
	domainstudio "github.com/wecratfs/commerce/internal/domain/imagestudio"
	"github.com/wecratfs/commerce/internal/ports"
)

func (h *Handler) ImageStudio(w http.ResponseWriter, r *http.Request) {
	if h.imageStudio == nil {
		writeError(w, http.StatusServiceUnavailable, "STUDIO_UNAVAILABLE", "AI Image Studio is temporarily unavailable.")
		return
	}
	session, ok := h.session(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in as a seller to use AI Image Studio.")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && !h.validCSRF(r, session) {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Authentication and CSRF validation are required.")
		return
	}

	rel := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/seller/image-studio"), "/")
	switch {
	case rel == "workspace" && r.Method == http.MethodGet:
		h.imageStudioWorkspace(w, r, session.UserID)
	case rel == "upload" && r.Method == http.MethodPost:
		h.imageStudioUpload(w, r, session.UserID)
	case strings.HasPrefix(rel, "jobs/"):
		h.imageStudioJobsRoute(w, r, session.UserID, strings.TrimPrefix(rel, "jobs/"))
	case strings.HasPrefix(rel, "variations/"):
		h.imageStudioVariationsRoute(w, r, session.UserID, strings.TrimPrefix(rel, "variations/"))
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Image studio endpoint not found.")
	}
}

func (h *Handler) imageStudioWorkspace(w http.ResponseWriter, r *http.Request, userID int64) {
	products, err := h.imageStudio.ListProducts(r.Context(), userID)
	if err != nil {
		h.writeStudioError(w, err)
		return
	}
	var selectedProductID int64
	if raw := strings.TrimSpace(r.URL.Query().Get("product_id")); raw != "" {
		selectedProductID, _ = strconv.ParseInt(raw, 10, 64)
	}
	if selectedProductID <= 0 && len(products) > 0 {
		selectedProductID = products[0].ID
	}
	var jobs []domainstudio.Job
	if selectedProductID > 0 {
		jobs, err = h.imageStudio.ListProductJobs(r.Context(), userID, selectedProductID)
		if err != nil {
			h.writeStudioError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"products":            products,
			"selected_product_id": selectedProductID,
			"presets":             domainstudio.AllPresets(),
			"jobs":                jobs,
		},
	})
}

func (h *Handler) imageStudioUpload(w http.ResponseWriter, r *http.Request, userID int64) {
	r.Body = http.MaxBytesReader(w, r.Body, domainstudio.MaxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_UPLOAD", "Upload a JPEG, PNG, or WebP image up to 25 MB.")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	productID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("product_id")), 10, 64)
	if err != nil || productID <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "PRODUCT_REQUIRED", "Select one of your products before uploading.")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "IMAGE_REQUIRED", "Choose a product photograph to process.")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, domainstudio.MaxUploadBytes+1))
	if err != nil || len(data) == 0 || len(data) > domainstudio.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "IMAGE_TOO_LARGE", "Product photograph must be 25 MB or smaller.")
		return
	}
	declaredType := ""
	if header != nil {
		declaredType = header.Header.Get("Content-Type")
	}

	job, err := h.imageStudio.UploadAndStart(r.Context(), userID, productID, declaredType, data)
	if err != nil {
		h.writeStudioError(w, err)
		return
	}
	status := http.StatusOK
	if job.Status == domainstudio.JobStatusUploaded || job.Status == domainstudio.JobStatusProcessing {
		status = http.StatusAccepted
	}
	writeJSON(w, status, map[string]any{"data": job})
}

func (h *Handler) imageStudioJobsRoute(w http.ResponseWriter, r *http.Request, userID int64, rest string) {
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	jobID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || jobID <= 0 {
		writeError(w, http.StatusNotFound, "JOB_NOT_FOUND", "Image job not found.")
		return
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		job, err := h.imageStudio.GetJob(r.Context(), userID, jobID)
		if err != nil {
			h.writeStudioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": job})
		return
	}
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Unknown image job action.")
		return
	}

	switch parts[1] {
	case "source":
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET to view source image.")
			return
		}
		body, info, err := h.imageStudio.OpenSource(r.Context(), userID, jobID)
		if err != nil {
			h.writeStudioError(w, err)
			return
		}
		serveStudioBinary(w, r, body, info)
	case "cutout":
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			body, info, err := h.imageStudio.OpenCutout(r.Context(), userID, jobID)
			if err != nil {
				h.writeStudioError(w, err)
				return
			}
			serveStudioBinary(w, r, body, info)
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET or POST for cutout image.")
			return
		}
		h.imageStudioUpdateCutout(w, r, userID, jobID)
	case "compose":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST to compose a variation.")
			return
		}
		var input struct {
			PresetSlug     string  `json:"preset_slug"`
			OffsetX        float64 `json:"offset_x"`
			OffsetY        float64 `json:"offset_y"`
			Scale          float64 `json:"scale"`
			FitMode        string  `json:"fit_mode"`
			ShadowStrength float64 `json:"shadow_strength"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		variation, err := h.imageStudio.ComposeVariation(r.Context(), userID, jobID, input.PresetSlug, domainstudio.CompositionParams{
			OffsetX:        input.OffsetX,
			OffsetY:        input.OffsetY,
			Scale:          input.Scale,
			FitMode:        input.FitMode,
			ShadowStrength: input.ShadowStrength,
		})
		if err != nil {
			h.writeStudioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": variation})
	case "regenerate":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST to regenerate previews.")
			return
		}
		job, err := h.imageStudio.RegenerateVariations(r.Context(), userID, jobID)
		if err != nil {
			h.writeStudioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": job})
	case "reject":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST to reject this job.")
			return
		}
		if err := h.imageStudio.RejectJob(r.Context(), userID, jobID); err != nil {
			h.writeStudioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"status": domainstudio.JobStatusRejected}})
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Unknown image job action.")
	}
}

func (h *Handler) imageStudioUpdateCutout(w http.ResponseWriter, r *http.Request, userID, jobID int64) {
	r.Body = http.MaxBytesReader(w, r.Body, domainstudio.MaxUploadBytes+(1<<20))
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	var data []byte
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_CUTOUT", "Upload a valid PNG cutout mask.")
			return
		}
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
		file, _, err := r.FormFile("cutout")
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "CUTOUT_REQUIRED", "PNG cutout field is required.")
			return
		}
		defer file.Close()
		data, err = io.ReadAll(io.LimitReader(file, domainstudio.MaxUploadBytes+1))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_CUTOUT", "Failed to read PNG cutout.")
			return
		}
	} else {
		var err error
		data, err = io.ReadAll(io.LimitReader(r.Body, domainstudio.MaxUploadBytes+1))
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_CUTOUT", "Failed to read PNG cutout.")
			return
		}
	}
	if len(data) == 0 || len(data) > domainstudio.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "IMAGE_TOO_LARGE", "Cutout PNG must be 25 MB or smaller.")
		return
	}
	job, err := h.imageStudio.UpdateCutout(r.Context(), userID, jobID, data)
	if err != nil {
		h.writeStudioError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": job})
}

func (h *Handler) imageStudioVariationsRoute(w http.ResponseWriter, r *http.Request, userID int64, rest string) {
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Unknown variation action.")
		return
	}
	variationID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || variationID <= 0 {
		writeError(w, http.StatusNotFound, "VARIATION_NOT_FOUND", "Image variation not found.")
		return
	}
	switch parts[1] {
	case "preview":
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use GET to preview variation.")
			return
		}
		body, info, err := h.imageStudio.OpenVariation(r.Context(), userID, variationID)
		if err != nil {
			h.writeStudioError(w, err)
			return
		}
		serveStudioBinary(w, r, body, info)
	case "approve":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST to approve variation.")
			return
		}
		variation, media, err := h.imageStudio.ApproveVariation(r.Context(), userID, variationID)
		if err != nil {
			h.writeStudioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"variation": variation, "media": media}})
	case "reject":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Use POST to reject variation.")
			return
		}
		variation, err := h.imageStudio.RejectVariation(r.Context(), userID, variationID)
		if err != nil {
			h.writeStudioError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": variation})
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Unknown variation action.")
	}
}

func serveStudioBinary(w http.ResponseWriter, r *http.Request, body io.ReadCloser, info ports.ObjectInfo) {
	defer body.Close()
	if info.ByteSize <= 0 || info.ByteSize > domainstudio.MaxUploadBytes {
		writeError(w, http.StatusNotFound, "MEDIA_NOT_FOUND", "Studio asset not found.")
		return
	}
	data, err := io.ReadAll(io.LimitReader(body, info.ByteSize+1))
	if err != nil || int64(len(data)) != info.ByteSize {
		writeError(w, http.StatusServiceUnavailable, "STUDIO_MEDIA_UNAVAILABLE", "Studio asset could not be read.")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	http.ServeContent(w, r, "studio-image", time.Time{}, bytes.NewReader(data))
}

func (h *Handler) writeStudioError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ports.ErrForbidden):
		writeError(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this product image.")
	case errors.Is(err, ports.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "The requested studio image was not found.")
	case errors.Is(err, engineadapter.ErrInvalidImage):
		writeError(w, http.StatusUnprocessableEntity, "IMAGE_INVALID", "Upload a valid JPEG, PNG, or WebP photograph.")
	case errors.Is(err, engineadapter.ErrImageTooSmall):
		writeError(w, http.StatusUnprocessableEntity, "IMAGE_TOO_SMALL", "Image dimensions must be at least 64x64 pixels.")
	case errors.Is(err, engineadapter.ErrImageTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "IMAGE_TOO_LARGE", "Image exceeds maximum 25 MB or 8192x8192 pixel limits.")
	case errors.Is(err, engineadapter.ErrEngineBusy):
		writeError(w, http.StatusServiceUnavailable, "STUDIO_BUSY", "The image engine is busy processing another photograph. Please retry in a moment.")
	case errors.Is(err, applicationimagestudio.ErrInvalidPreset):
		writeError(w, http.StatusUnprocessableEntity, "INVALID_PRESET", "Choose one of the supported studio background presets.")
	case errors.Is(err, applicationimagestudio.ErrJobNotReady):
		writeError(w, http.StatusConflict, "JOB_NOT_READY", "Wait for background removal to finish before editing.")
	default:
		writeError(w, http.StatusInternalServerError, "STUDIO_FAILED", "Image studio processing could not be completed.")
	}
}
