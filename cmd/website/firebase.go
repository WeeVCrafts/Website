package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	redisclient "github.com/redis/go-redis/v9"
	applicationauth "github.com/wecratfs/commerce/internal/application/auth"
	"github.com/wecratfs/commerce/internal/ports"
	"github.com/wecratfs/commerce/web/assets"
	"net/http"
	"os"
	"strings"
	"time"
)

func validateFirebaseEnvironment(environment string) error {
	if strings.EqualFold(environment, "production") && (os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") != "" || os.Getenv("FIREBASE_EMULATOR_HOST") != "") {
		return errors.New("Firebase emulators cannot be enabled in production")
	}
	if os.Getenv("WECRATFS_FIREBASE_PROJECT_ID") != "" {
		for _, key := range []string{"WECRATFS_FIREBASE_API_KEY", "WECRATFS_FIREBASE_APP_ID", "WECRATFS_FIREBASE_AUTH_DOMAIN", "WECRATFS_FIREBASE_SENDER_ID"} {
			if os.Getenv(key) == "" {
				return errors.New("Firebase public configuration is incomplete")
			}
		}
	}
	return nil
}

func firebaseConfiguration(enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		// These are public browser identifiers. Never expose server credentials.
		writeLocalJSON(w, 200, map[string]any{"enabled": enabled, "config": map[string]string{
			"projectId": os.Getenv("WECRATFS_FIREBASE_PROJECT_ID"), "apiKey": os.Getenv("WECRATFS_FIREBASE_API_KEY"),
			"appId": os.Getenv("WECRATFS_FIREBASE_APP_ID"), "authDomain": os.Getenv("WECRATFS_FIREBASE_AUTH_DOMAIN"),
			"messagingSenderId": os.Getenv("WECRATFS_FIREBASE_SENDER_ID"), "measurementId": os.Getenv("WECRATFS_FIREBASE_MEASUREMENT_ID"),
		}, "vapidKey": os.Getenv("WECRATFS_FIREBASE_VAPID_KEY"), "recoveryReturnURL": "https://weevcrafts.com/"})
	}
}

type customerPush struct {
	pool     *pgxpool.Pool
	redis    *redisclient.Client
	sessions *applicationauth.SessionService
	provider ports.FirebaseProvider
}

func pushHash(raw string) string {
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}
func (p *customerPush) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	cookie, err := r.Cookie("wecratfs_session")
	if err != nil {
		writeLocalJSON(w, 401, map[string]string{"error": "Sign in to manage notifications."})
		return
	}
	s, err := p.sessions.Get(r.Context(), cookie.Value)
	if err != nil || len(s.RoleSlugs) != 1 || s.RoleSlugs[0] != "customer" {
		writeLocalJSON(w, 403, map[string]string{"error": "Customer authentication required."})
		return
	}
	if r.Method != http.MethodGet && (s.CSRFToken == "" || subtle.ConstantTimeCompare([]byte(s.CSRFToken), []byte(r.Header.Get("X-CSRF-Token"))) != 1) {
		writeLocalJSON(w, 403, map[string]string{"error": "CSRF validation required."})
		return
	}
	hash := pushHash(s.ID)
	if r.URL.Path == "/api/v1/customer/push/test" {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		var token string
		if err = p.pool.QueryRow(r.Context(), `SELECT token FROM customer_push_subscriptions WHERE user_id=$1 AND session_hash=$2 AND expires_at>NOW() LIMIT 1`, s.UserID, hash).Scan(&token); err != nil {
			writeLocalJSON(w, 404, map[string]string{"error": "Enable notifications first."})
			return
		}
		if err = p.provider.SendPush(r.Context(), token); err != nil {
			writeLocalJSON(w, 503, map[string]string{"error": "Notification delivery is unavailable."})
			return
		}
		writeLocalJSON(w, 200, map[string]string{"status": "sent"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		var enabled bool
		err = p.pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM customer_push_subscriptions WHERE user_id=$1 AND session_hash=$2 AND expires_at>NOW())`, s.UserID, hash).Scan(&enabled)
		if err != nil {
			w.WriteHeader(503)
			return
		}
		writeLocalJSON(w, 200, map[string]bool{"enabled": enabled})
	case http.MethodPost:
		var in struct {
			Token string `json:"token"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.Token) < 20 || len(in.Token) > 2048 {
			writeLocalJSON(w, 422, map[string]string{"error": "Invalid browser registration."})
			return
		}
		for _, c := range in.Token {
			if c <= 32 || c > 126 {
				w.WriteHeader(422)
				return
			}
		}
		result, err := p.pool.Exec(r.Context(), `INSERT INTO customer_push_subscriptions(token_hash,token,user_id,session_hash,expires_at,last_notification_id) VALUES($1,$2,$3,$4,$5,COALESCE((SELECT max(id) FROM notifications WHERE user_id=$3),0)) ON CONFLICT(token_hash) DO UPDATE SET session_hash=EXCLUDED.session_hash,expires_at=EXCLUDED.expires_at WHERE customer_push_subscriptions.user_id=EXCLUDED.user_id`, pushHash(in.Token), in.Token, s.UserID, hash, s.ExpiresAt)
		if err != nil {
			w.WriteHeader(503)
			return
		}
		if result.RowsAffected() == 0 {
			writeLocalJSON(w, 409, map[string]string{"error": "Disable the previous account's browser notifications before switching accounts."})
			return
		}
		writeLocalJSON(w, 200, map[string]string{"status": "enabled"})
	case http.MethodDelete:
		if _, err = p.pool.Exec(r.Context(), `DELETE FROM customer_push_subscriptions WHERE user_id=$1 AND session_hash=$2`, s.UserID, hash); err != nil {
			w.WriteHeader(503)
			return
		}
		writeLocalJSON(w, 200, map[string]string{"status": "disabled"})
	default:
		w.WriteHeader(405)
	}
}
func (p *customerPush) Process(ctx context.Context) error {
	// External deletion is retried independently of local privacy deletion.
	rows, err := p.pool.Query(ctx, `SELECT firebase_uid FROM firebase_deletion_queue ORDER BY created_at LIMIT 20`)
	if err != nil {
		return err
	}
	var deleted []string
	for rows.Next() {
		var uid string
		if err = rows.Scan(&uid); err != nil {
			rows.Close()
			return err
		}
		deleted = append(deleted, uid)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, uid := range deleted {
		if err = p.provider.DeleteIdentity(ctx, uid); err == nil {
			_, err = p.pool.Exec(ctx, `DELETE FROM firebase_deletion_queue WHERE firebase_uid=$1`, uid)
		}
		if err != nil {
			return err
		}
	}
	_, err = p.pool.Exec(ctx, `DELETE FROM customer_push_subscriptions s USING users u WHERE s.user_id=u.id AND (s.expires_at<=NOW() OR u.status<>'active' OR u.sessions_revoked_at>=s.created_at OR NOT EXISTS(SELECT 1 FROM user_roles ur WHERE ur.user_id=u.id AND ur.role_slug='customer') OR (SELECT count(*) FROM user_roles ur WHERE ur.user_id=u.id)<>1)`)
	if err != nil {
		return err
	}
	rows, err = p.pool.Query(ctx, `SELECT s.token_hash,s.token,s.session_hash,s.user_id,COALESCE((SELECT max(n.id) FROM notifications n WHERE n.user_id=s.user_id AND n.kind='order' AND n.id>s.last_notification_id AND n.created_at>NOW()-INTERVAL '1 day'),0) FROM customer_push_subscriptions s LEFT JOIN notification_preferences p ON p.user_id=s.user_id WHERE s.expires_at>NOW() AND COALESCE(p.order_updates,TRUE) AND EXISTS(SELECT 1 FROM notifications n WHERE n.user_id=s.user_id AND n.kind='order' AND n.id>s.last_notification_id AND n.created_at>NOW()-INTERVAL '1 day') ORDER BY s.last_notification_id,s.created_at LIMIT 100`)
	if err != nil {
		return err
	}
	type job struct {
		hash, token, sessionHash string
		userID, notificationID   int64
	}
	var jobs []job
	for rows.Next() {
		var j job
		if err = rows.Scan(&j.hash, &j.token, &j.sessionHash, &j.userID, &j.notificationID); err != nil {
			rows.Close()
			return err
		}
		jobs = append(jobs, j)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, j := range jobs {
		// Redis stores hashed session keys. Check the original session server-side,
		// including Firebase revocation, immediately before sending any message.
		raw, err := p.redis.Get(ctx, "wecratfs:session:"+j.sessionHash).Bytes()
		if errors.Is(err, redisclient.Nil) {
			_, err = p.pool.Exec(ctx, `DELETE FROM customer_push_subscriptions WHERE token_hash=$1`, j.hash)
			if err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if j.notificationID == 0 {
			continue
		}
		var s ports.SessionRecord
		if json.Unmarshal(raw, &s) != nil || s.UserID != j.userID {
			continue
		}
		if _, err = p.sessions.Get(ctx, s.ID); err != nil {
			continue
		}
		if err = p.provider.SendPush(ctx, j.token); errors.Is(err, ports.ErrCredentialsNotFound) {
			_, err = p.pool.Exec(ctx, `DELETE FROM customer_push_subscriptions WHERE token_hash=$1`, j.hash)
		} else if err == nil {
			_, err = p.pool.Exec(ctx, `UPDATE customer_push_subscriptions SET last_notification_id=$2 WHERE token_hash=$1`, j.hash, j.notificationID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func firebaseWorker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(405)
		return
	}
	data, err := assets.FS.ReadFile("js/firebase-messaging-sw.js")
	if err != nil {
		w.WriteHeader(404)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Service-Worker-Allowed", "/")
	publicConfig, _ := json.Marshal(map[string]string{"projectId": os.Getenv("WECRATFS_FIREBASE_PROJECT_ID"), "apiKey": os.Getenv("WECRATFS_FIREBASE_API_KEY"), "appId": os.Getenv("WECRATFS_FIREBASE_APP_ID"), "messagingSenderId": os.Getenv("WECRATFS_FIREBASE_SENDER_ID")})
	_, _ = w.Write([]byte("const WEEVCRAFTS_FIREBASE_CONFIG=" + string(publicConfig) + ";\n"))
	_, _ = w.Write(data)
}

// A timeout bounds retries and prevents a stalled provider from blocking shutdown.
func runFirebaseProcessing(ctx context.Context, p *customerPush) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return p.Process(ctx)
}
