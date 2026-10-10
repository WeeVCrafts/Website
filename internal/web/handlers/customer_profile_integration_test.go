package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	redisadapter "github.com/wecratfs/commerce/internal/adapters/redis"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	applicationcatalog "github.com/wecratfs/commerce/internal/application/catalog"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

type failingAvatarStorage struct {
	failDelete bool
	deleted    []string
}

func (*failingAvatarStorage) CreateUploadURL(context.Context, ports.UploadRequest) (ports.UploadURL, error) {
	return ports.UploadURL{}, errors.New("unavailable")
}
func (*failingAvatarStorage) PutObject(context.Context, string, string, int64, io.Reader) error {
	return errors.New("unavailable")
}
func (*failingAvatarStorage) InspectObject(context.Context, string) (ports.ObjectInfo, error) {
	return ports.ObjectInfo{}, errors.New("unavailable")
}
func (*failingAvatarStorage) CreateDownloadURL(context.Context, string, time.Duration) (ports.DownloadURL, error) {
	return ports.DownloadURL{}, errors.New("unavailable")
}
func (s *failingAvatarStorage) DeleteObject(_ context.Context, key string) error {
	if s.failDelete {
		return errors.New("unavailable")
	}
	s.deleted = append(s.deleted, key)
	return nil
}

func TestAvatarStorageFailurePreservesProfileAndCleanupRetries(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" || os.Getenv("REDIS_ADDR") == "" {
		t.Skip("real PostgreSQL and Redis required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	client := redisclient.NewClient(&redisclient.Options{Addr: os.Getenv("REDIS_ADDR")})
	t.Cleanup(func() { _ = client.Close() })
	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,display_name,phone,status) VALUES($1,'Saved customer','+919876543210','active') RETURNING id`, fmt.Sprintf("photo-failure-%d@example.invalid", time.Now().UnixNano())).Scan(&id); err != nil {
		t.Fatal(err)
	}
	savedKey := fmt.Sprintf("avatars/%d/%032x", id, 1)
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles(user_id,role_slug) VALUES($1,'customer');`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_avatars(user_id,object_key) VALUES($1,$2)`, id, savedKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM avatar_cleanup WHERE object_key LIKE $1`, fmt.Sprintf("avatars/%d/%%", id))
	})
	sessions := applicationauth.NewSessionService(redisadapter.NewSessionStore(client))
	session, err := sessions.Start(ctx, domainidentity.User{ID: id, Roles: []domainidentity.Role{domainidentity.RoleCustomer}}, "QA", "127.0.0.1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sessions.Revoke(ctx, session.ID) })
	storage := &failingAvatarStorage{failDelete: true}
	handler := &LiveStorefrontHandler{pool: pool, sessions: sessions, profileStorage: storage, catalog: applicationcatalog.NewService(postgres.NewProductRepository(pool))}
	var imageBytes, body bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	form := multipart.NewWriter(&body)
	for name, value := range map[string]string{"csrf_token": session.CSRFToken, "name": "Changed customer", "phone": "+918888888888"} {
		if err := form.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	file, err := form.CreateFormFile("photo", "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(imageBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/account/profile", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.AddCookie(&http.Cookie{Name: "wecratfs_session", Value: session.ID})
	response := httptest.NewRecorder()
	handler.saveCustomerProfile(response, request)
	if response.Code != 503 {
		t.Fatalf("storage failure status=%d", response.Code)
	}
	var name, phone, key string
	if err := pool.QueryRow(ctx, `SELECT u.display_name,u.phone,a.object_key FROM users u JOIN user_avatars a ON a.user_id=u.id WHERE u.id=$1`, id).Scan(&name, &phone, &key); err != nil {
		t.Fatal(err)
	}
	if name != "Saved customer" || phone != "+919876543210" || key != savedKey {
		t.Fatal("failed upload changed persisted profile")
	}
	if _, err := pool.Exec(ctx, `UPDATE avatar_cleanup SET created_at=NOW()-INTERVAL '6 minutes' WHERE object_key LIKE $1`, fmt.Sprintf("avatars/%d/%%", id)); err != nil {
		t.Fatal(err)
	}
	// Even an accidentally queued live key must never be removed.
	if _, err := pool.Exec(ctx, `INSERT INTO avatar_cleanup(object_key,created_at) VALUES($1,NOW()-INTERVAL '6 minutes')`, savedKey); err != nil {
		t.Fatal(err)
	}
	if err := handler.CleanupAvatars(ctx); err == nil {
		t.Fatal("storage failure was not retained for retry")
	}
	storage.failDelete = false
	if err := handler.CleanupAvatars(ctx); err != nil {
		t.Fatal(err)
	}
	if len(storage.deleted) != 1 || storage.deleted[0] == savedKey {
		t.Fatal("cleanup removed a live avatar or failed to retry")
	}
	var queued int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM avatar_cleanup WHERE object_key LIKE $1`, fmt.Sprintf("avatars/%d/%%", id)).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if queued != 1 {
		t.Fatalf("expected only protected live key, queued=%d", queued)
	}
}
