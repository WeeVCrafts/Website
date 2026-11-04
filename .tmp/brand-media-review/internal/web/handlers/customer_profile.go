package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/internal/web/viewmodels"
	"github.com/wecratfs/commerce/web/pages"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const maxAvatarBytes = 5 << 20

func (h *LiveStorefrontHandler) SetProfileStorage(storage ports.ObjectStorage) {
	h.profileStorage = storage
}

// Decode before storing; the output contains pixels only, never EXIF metadata,
// scripts, original filenames or the untrusted bytes supplied by the browser.
func normalizeAvatar(input []byte) ([]byte, error) {
	if len(input) == 0 || len(input) > maxAvatarBytes {
		return nil, errors.New("Choose an image up to 5 MiB.")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(input))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 16000000 {
		return nil, errors.New("Choose a valid JPEG, PNG or WebP image up to 16 megapixels.")
	}
	source, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, errors.New("This image could not be read. Choose another photo.")
	}
	width, height := config.Width, config.Height
	if width > 512 || height > 512 {
		if width >= height {
			height = max(1, height*512/width)
			width = 512
		} else {
			width = max(1, width*512/height)
			height = 512
		}
	}
	output := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(output, output.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(output, output.Bounds(), source, source.Bounds(), draw.Over, nil)
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, output, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func validProfile(name, phone string) bool {
	n := len([]rune(strings.TrimSpace(name)))
	if n < 2 || n > 100 || len(phone) > 24 {
		return false
	}
	digits := 0
	for i, c := range phone {
		if c >= '0' && c <= '9' {
			digits++
			continue
		}
		if c == '+' && i == 0 {
			continue
		}
		if !strings.ContainsRune(" ()-", c) {
			return false
		}
	}
	return digits >= 7 && digits <= 15
}

func (h *LiveStorefrontHandler) profileError(w http.ResponseWriter, r *http.Request, message string, status int) {
	page, err := h.page(r)
	if err != nil {
		http.Error(w, message, status)
		return
	}
	page.ProfileError = message
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	h.render(w, r, pages.LiveAccountDetail(page))
}

func (h *LiveStorefrontHandler) saveCustomerProfile(w http.ResponseWriter, r *http.Request) {
	userID, csrf, ok := h.session(r)
	if !ok || !h.customerRole(userID, r) {
		http.Error(w, "Authentication required.", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+(64<<10))
	var err error
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		err = r.ParseMultipartForm(1 << 20)
		if r.MultipartForm != nil {
			defer r.MultipartForm.RemoveAll()
		}
	} else {
		err = r.ParseForm()
	}
	if err != nil {
		h.profileError(w, r, "Choose an image up to 5 MiB and try again.", http.StatusRequestEntityTooLarge)
		return
	}
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		token = r.Form.Get("csrf_token")
	}
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(csrf)) != 1 {
		http.Error(w, "Invalid CSRF token.", http.StatusForbidden)
		return
	}
	name, phone := strings.TrimSpace(r.Form.Get("name")), strings.TrimSpace(r.Form.Get("phone"))
	if !validProfile(name, phone) {
		h.profileError(w, r, "Enter a name of 2 to 100 characters and a phone number with 7 to 15 digits.", 422)
		return
	}
	remove := r.Form.Get("remove_photo") == "1"
	var normalized []byte
	if r.MultipartForm != nil {
		file, _, fileErr := r.FormFile("photo")
		if fileErr == nil {
			defer file.Close()
			data, readErr := io.ReadAll(io.LimitReader(file, maxAvatarBytes+1))
			if readErr != nil {
				h.profileError(w, r, "The photo could not be read. Try again.", 422)
				return
			}
			normalized, err = normalizeAvatar(data)
			if err != nil {
				h.profileError(w, r, err.Error(), 422)
				return
			}
		} else if !errors.Is(fileErr, http.ErrMissingFile) {
			h.profileError(w, r, "The photo could not be read.", 422)
			return
		}
	}
	if remove && normalized != nil {
		h.profileError(w, r, "Choose either a new photo or Remove photo.", 422)
		return
	}
	key := ""
	if normalized != nil {
		writer, ok := h.profileStorage.(ports.ObjectStorageWriter)
		if !ok {
			h.profileError(w, r, "Photo storage is unavailable. Try again later.", 503)
			return
		}
		token := make([]byte, 16)
		if _, err = rand.Read(token); err != nil {
			h.profileError(w, r, "Photo could not be saved.", 503)
			return
		}
		key = fmt.Sprintf("avatars/%d/%s", userID, hex.EncodeToString(token))
		// Queue a failed transaction's file too. The cleanup worker waits until this
		// request has finished, then verifies that no avatar references the object.
		if _, err = h.pool.Exec(r.Context(), `INSERT INTO avatar_cleanup(object_key) VALUES($1)`, key); err != nil {
			h.profileError(w, r, "Photo could not be saved.", 503)
			return
		}
		if err = writer.PutObject(r.Context(), key, "image/jpeg", int64(len(normalized)), bytes.NewReader(normalized)); err != nil {
			h.profileError(w, r, "Photo upload failed. Your profile was not changed.", 503)
			return
		}
	}
	err = postgres.WithinTransaction(r.Context(), h.pool, func(ctx context.Context, tx pgx.Tx) error {
		// Serialize updates with deletion/suspension and recheck the role inside
		// the transaction; a stale form must never resurrect a deleted profile.
		var active bool
		if err := tx.QueryRow(ctx, `SELECT status='active' AND (SELECT count(*) FROM user_roles WHERE user_id=$1)=1 AND EXISTS(SELECT 1 FROM user_roles WHERE user_id=$1 AND role_slug='customer') FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&active); err != nil {
			return err
		}
		if !active {
			return ports.ErrForbidden
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET display_name=$1,phone=$2,updated_at=NOW() WHERE id=$3`, name, phone, userID); err != nil {
			return err
		}
		if remove {
			_, err := tx.Exec(ctx, `DELETE FROM user_avatars WHERE user_id=$1`, userID)
			return err
		}
		if key != "" {
			if _, err := tx.Exec(ctx, `INSERT INTO user_avatars(user_id,object_key) VALUES($1,$2) ON CONFLICT(user_id) DO UPDATE SET object_key=EXCLUDED.object_key,updated_at=NOW()`, userID, key); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `DELETE FROM avatar_cleanup WHERE object_key=$1`, key)
			return err
		}
		return nil
	})
	if err != nil {
		h.profileError(w, r, "Profile could not be saved. Your previous details are unchanged.", 503)
		return
	}
	http.Redirect(w, r, "/account/profile?notice=Profile+updated", http.StatusSeeOther)
}

func (h *LiveStorefrontHandler) profilePhoto(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	userID, _, ok := h.session(r)
	if !ok || !h.customerRole(userID, r) {
		w.WriteHeader(403)
		return
	}
	var key string
	if err := h.pool.QueryRow(r.Context(), `SELECT object_key FROM user_avatars WHERE user_id=$1`, userID).Scan(&key); err != nil {
		w.WriteHeader(404)
		return
	}
	reader, ok := h.profileStorage.(ports.ObjectStorageContentReader)
	if !ok {
		w.WriteHeader(503)
		return
	}
	body, info, err := reader.OpenObject(r.Context(), key)
	if err != nil {
		w.WriteHeader(503)
		return
	}
	defer body.Close()
	if info.ByteSize <= 0 || info.ByteSize > maxAvatarBytes || info.ContentType != "image/jpeg" {
		w.WriteHeader(404)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = io.Copy(w, io.LimitReader(body, maxAvatarBytes))
}

func (h *LiveStorefrontHandler) CleanupAvatars(ctx context.Context) error {
	reader, ok := h.profileStorage.(ports.ObjectStorageReader)
	if !ok {
		return nil
	}
	rows, err := h.pool.Query(ctx, `SELECT c.object_key FROM avatar_cleanup c WHERE c.created_at<NOW()-INTERVAL '5 minutes' AND NOT EXISTS(SELECT 1 FROM user_avatars a WHERE a.object_key=c.object_key) ORDER BY c.created_at LIMIT 20`)
	if err != nil {
		return err
	}
	var keys []string
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err = reader.DeleteObject(ctx, key); err != nil {
			return err
		}
		if _, err = h.pool.Exec(ctx, `DELETE FROM avatar_cleanup WHERE object_key=$1`, key); err != nil {
			return err
		}
	}
	return nil
}

// A minimal identity summary is needed on every storefront page; loading all
// orders and addresses solely for the header would expose and query too much.
func (h *LiveStorefrontHandler) loadProfileSummary(ctx context.Context, userID int64, pageName, phone *string, hasPhoto *bool) error {
	return h.pool.QueryRow(ctx, `SELECT display_name,phone,EXISTS(SELECT 1 FROM user_avatars WHERE user_id=$1) FROM users WHERE id=$1 AND status='active'`, userID).Scan(pageName, phone, hasPhoto)
}

func (h *LiveStorefrontHandler) AccessUnavailable(w http.ResponseWriter, r *http.Request) {
	userID, csrf, ok := h.session(r)
	if !ok {
		http.Redirect(w, r, "/login", 303)
		return
	}
	if destination := applicationauth.LoginDestination(h.userRoles(r, userID)); destination != "/access-unavailable" {
		http.Redirect(w, r, destination, 303)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	h.render(w, r, pages.AccessUnavailable(viewmodels.CustomerPage{Route: r.URL.Path, Title: "Account access needs attention", Authenticated: true, CSRFToken: csrf}))
}
