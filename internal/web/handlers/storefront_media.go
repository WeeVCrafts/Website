package handlers

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"context"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"golang.org/x/image/draw"
)

func normalizeStorefrontImage(input []byte, width int) ([]byte, error) {
	c, format, e := image.DecodeConfig(bytes.NewReader(input))
	if e != nil || len(input) > 8<<20 || c.Width < 1 || c.Height < 1 || c.Width > 8192 || c.Height > 8192 || int64(c.Width)*int64(c.Height) > 16000000 || (format != "jpeg" && format != "png" && format != "webp") {
		return nil, fmt.Errorf("Choose a JPEG, PNG or WebP image up to 8 MiB and 16 megapixels.")
	}
	source, _, e := image.Decode(bytes.NewReader(input))
	if e != nil {
		return nil, fmt.Errorf("This photograph could not be read. Choose a different image.")
	}
	w, h := c.Width, c.Height
	if w > width {
		h = max(1, h*width/w)
		w = width
	}
	output := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(output, output.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(output, output.Bounds(), source, source.Bounds(), draw.Over, nil)
	var b bytes.Buffer
	e = jpeg.Encode(&b, output, &jpeg.Options{Quality: 88})
	return b.Bytes(), e
}
func (h *LivePortalHandler) storefrontUpload(w http.ResponseWriter, r *http.Request, actor int64, csrf, root string) {
	r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
	if e := r.ParseMultipartForm(9 << 20); e != nil {
		http.Error(w, "Choose an image smaller than 8 MiB.", 413)
		return
	}
	defer r.MultipartForm.RemoveAll()
	token := r.FormValue("csrf_token")
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(csrf)) != 1 {
		http.Error(w, "Your session changed. Reload before uploading.", 403)
		return
	}
	alt := strings.TrimSpace(r.FormValue("alt"))
	x, y := formInt(r, "focal_x"), formInt(r, "focal_y")
	if alt == "" || len(alt) > 300 || x < 0 || x > 100 || y < 0 || y > 100 {
		http.Error(w, "Add an image description and focal coordinates from 0 to 100.", 422)
		return
	}
	file, _, e := r.FormFile("image_file")
	if e != nil {
		http.Error(w, "Choose a photograph to upload.", 422)
		return
	}
	defer file.Close()
	input, e := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	if e != nil {
		http.Error(w, "Image could not be read. Select it again.", 422)
		return
	}
	writer, ok := h.categoryStorage.(ports.ObjectStorageWriter)
	if !ok {
		http.Error(w, "Media storage is unavailable. Try again later.", 503)
		return
	}
	id := editorID()
	key := "storefront/" + id + ".jpg"
	variants := map[string]string{}
	var storedBytes int
	for _, width := range []int{1920, 960, 480} {
		output, e := normalizeStorefrontImage(input, width)
		if e != nil {
			http.Error(w, e.Error(), 422)
			return
		}
		variantKey := key
		if width == 1920 {
			storedBytes = len(output)
		}
		if width != 1920 {
			variantKey = "storefront/" + id + "-" + strconv.Itoa(width) + ".jpg"
		}
		if e = writer.PutObject(r.Context(), variantKey, "image/jpeg", int64(len(output)), bytes.NewReader(output)); e != nil {
			http.Error(w, "The photograph could not be stored. Check object storage and retry this upload.", 503)
			return
		}
		variants[strconv.Itoa(width)] = variantKey
	}
	data, _ := json.Marshal(variants)
	// Retain media: drafts and immutable releases may reference any upload.
	e = postgres.WithinTransaction(r.Context(), h.pool, func(ctx context.Context, tx pgx.Tx) error {
		var allowed bool
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, actor).Scan(&actor); err != nil {
			return ports.ErrForbidden
		}
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.id=$1 AND u.status='active' AND ur.role_slug IN ('admin','super_admin'))`, actor).Scan(&allowed); err != nil {
			return err
		}
		if !allowed {
			return ports.ErrForbidden
		}
		_, err := tx.Exec(ctx, `INSERT INTO storefront_media(id,object_key,content_type,byte_size,alt,focal_x,focal_y,variants,created_by) VALUES($1,$2,'image/jpeg',$3,$4,$5,$6,$7,$8)`, id, key, storedBytes, alt, x, y, data, actor)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_id,action,resource_type,resource_id,request_id,metadata) VALUES($1,'storefront.media_uploaded','storefront_media',$2,$3,'{}')`, fmt.Sprint(actor), id, ports.RequestID(ctx))
		return err
	})
	if e != nil {
		http.Error(w, "The upload could not be recorded. Try again later.", 503)
		return
	}
	http.Redirect(w, r, root+"/storefront?"+url.Values{"draft": {r.FormValue("draft")}, "tab": {"media"}, "notice": {"Image uploaded. Select its URL in a draft; publishing controls when it appears."}}.Encode(), 303)
}
func (h *LivePortalHandler) storefrontMediaList(r *http.Request, m *viewmodels.StorefrontEditor) ([]viewmodels.StorefrontMedia, error) {
	query := r.URL.Query().Get("media_q")
	page, _ := strconv.Atoi(r.URL.Query().Get("media_page"))
	if page < 1 || page > 100000 {
		page = 1
	}
	var count int
	if e := h.pool.QueryRow(r.Context(), `SELECT count(*) FROM storefront_media WHERE $1='' OR alt ILIKE '%'||$1||'%'`, query).Scan(&count); e != nil {
		return nil, e
	}
	page = min(page, max(1, (count+23)/24))
	m.MediaPage = page
	m.MediaQuery = query
	link := func(n int) string {
		return m.Admin.PortalRoot + "/storefront?" + url.Values{"draft": {fmt.Sprint(m.Draft.ID)}, "tab": {"media"}, "media_q": {query}, "media_page": {fmt.Sprint(n)}}.Encode()
	}
	if page > 1 {
		m.PreviousMedia = link(page - 1)
	}
	if page*24 < count {
		m.NextMedia = link(page + 1)
	}
	rows, e := h.pool.Query(r.Context(), `SELECT id,alt,focal_x,focal_y,to_char(created_at,'DD Mon YYYY') FROM storefront_media WHERE $1='' OR alt ILIKE '%'||$1||'%' ORDER BY created_at DESC,id DESC LIMIT 24 OFFSET $2`, query, (page-1)*24)
	if e != nil {
		return nil, e
	}

	defer rows.Close()
	var result []viewmodels.StorefrontMedia
	for rows.Next() {
		var m viewmodels.StorefrontMedia
		if rows.Scan(&m.ID, &m.Alt, &m.FocalX, &m.FocalY, &m.Created) == nil {
			m.URL = "/media/storefront/" + m.ID
			result = append(result, m)
		}
	}
	return result, rows.Err()
}
func (h *LivePortalHandler) StorefrontMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", 405)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/media/storefront/")
	var key string
	var variantsData []byte
	if e := h.pool.QueryRow(r.Context(), `SELECT object_key,variants FROM storefront_media WHERE id=$1`, id).Scan(&key, &variantsData); e != nil {
		http.NotFound(w, r)
		return
	}
	// Unpublished photos are private to current company administrators.
	var published bool
	if e := h.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM storefront_releases WHERE strpos(config::text,$1)>0)`, "/media/storefront/"+id).Scan(&published); e != nil {
		http.Error(w, "Image unavailable", 503)
		return
	}
	if !published {
		session, ok := h.session(r)
		if !ok || !h.hasRole(r, session.UserID, "admin", "super_admin") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	var variants map[string]string
	_ = json.Unmarshal(variantsData, &variants)
	if width := r.URL.Query().Get("w"); width == "480" || width == "960" {
		if v := variants[width]; v != "" {
			key = v
		}
	}
	reader, ok := h.categoryStorage.(ports.ObjectStorageContentReader)
	if !ok {
		http.Error(w, "Image storage unavailable", 503)
		return
	}
	body, info, e := reader.OpenObject(r.Context(), key)
	if e != nil {
		http.Error(w, "Photograph temporarily unavailable", 503)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if info.ByteSize > 0 {
		w.Header().Set("Content-Length", fmt.Sprint(info.ByteSize))
	}
	if r.Method == "GET" {
		_, _ = io.Copy(w, io.LimitReader(body, 16<<20))
	}
}
