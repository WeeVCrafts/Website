package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/wecratfs/commerce/internal/adapters/storage"
	"github.com/wecratfs/commerce/internal/config"
)

// Include records with a stored payload. Product upload intents and deletion
// queues do not assert that a corresponding object must exist.
const requiredObjects = `
	SELECT object_key FROM product_media WHERE status='ready'
	UNION SELECT image_object_key FROM categories WHERE image_object_key<>''
	UNION SELECT object_key FROM storefront_media
	UNION SELECT value FROM storefront_media CROSS JOIN LATERAL jsonb_each_text(variants)
	UNION SELECT object_key FROM marketplace_claim_evidence
	UNION SELECT object_key FROM support_attachments
	UNION SELECT object_key FROM user_avatars
	UNION SELECT object_key FROM seller_documents WHERE status<>'deleted'
	UNION SELECT object_key FROM data_export_requests WHERE status='ready' AND object_key<>''
	UNION SELECT evidence_key FROM seller_payout_payee_versions
	UNION SELECT transfer_evidence_key FROM seller_settlements WHERE transfer_evidence_key IS NOT NULL`

type backedUpObject struct {
	Key         string `json:"key"`
	ArchivePath string `json:"archive_path"`
	ContentType string `json:"content_type"`
	SHA256      string `json:"sha256"`
	Bytes       int64  `json:"bytes"`
}

func checkReferenceInventory(ctx context.Context, tx pgx.Tx) error {
	rows, err := tx.Query(ctx, `SELECT table_name||'.'||column_name FROM information_schema.columns
		WHERE table_schema=current_schema() AND column_name ~ '(object_key|evidence_key)$'`)
	if err != nil {
		return errors.New("collect: cannot validate object reference inventory")
	}
	defer rows.Close()
	known := map[string]bool{
		"product_media.object_key": true, "categories.image_object_key": true,
		"storefront_media.object_key": true, "marketplace_claim_evidence.object_key": true,
		"support_attachments.object_key": true, "user_avatars.object_key": true,
		"avatar_cleanup.object_key": true, "seller_documents.object_key": true,
		"data_export_requests.object_key": true, "seller_payout_payee_versions.evidence_key": true,
		"seller_settlements.transfer_evidence_key": true,
	}
	for rows.Next() {
		var column string
		if err = rows.Scan(&column); err != nil {
			return errors.New("collect: cannot read object reference inventory")
		}
		if !known[column] {
			return errors.New("collect: unmapped object reference column")
		}
	}
	if rows.Err() != nil {
		return errors.New("collect: object reference inventory query failed")
	}
	return nil
}

func collect(ctx context.Context, dst io.Writer) error {
	cfg := config.FromEnv()
	settingsDir, revision := os.Getenv("BACKUP_CONFIG_DIRECTORY"), os.Getenv("SOURCE_COMMIT")
	if settingsDir == "" || revision == "" || os.Getenv("S3_BUCKET") == "" || os.Getenv("DATABASE_URL") == "" {
		return errors.New("collect: database, storage or release configuration incomplete")
	}
	pgConfig, err := cfg.DatabasePoolConfig()
	if err != nil {
		return errors.New("collect: invalid database configuration")
	}
	conn, err := pgx.ConnectConfig(ctx, pgConfig.ConnConfig)
	if err != nil {
		return errors.New("collect: database connection failed")
	}
	defer conn.Close(context.Background())
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return errors.New("collect: cannot start database snapshot")
	}
	defer tx.Rollback(context.Background())
	if err = checkReferenceInventory(ctx, tx); err != nil {
		return err
	}
	var snapshot string
	if err = tx.QueryRow(ctx, "SELECT pg_export_snapshot()").Scan(&snapshot); err != nil {
		return errors.New("collect: cannot export database snapshot")
	}
	rows, err := tx.Query(ctx, requiredObjects)
	if err != nil {
		return errors.New("collect: cannot inventory required object references")
	}
	required := make(map[string]struct{})
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			rows.Close()
			return errors.New("collect: cannot read required object references")
		}
		required[key] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return errors.New("collect: object reference query failed")
	}
	work, err := os.MkdirTemp("", "recovery-")
	if err != nil {
		return errors.New("collect: cannot create temporary database dump")
	}
	defer os.RemoveAll(work)
	dump := filepath.Join(work, "database.dump")
	databaseURL, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		return errors.New("collect: cannot read connection mode")
	}
	mode := databaseURL.Query().Get("sslmode")
	if mode == "" {
		mode = "prefer"
	}
	env := append(os.Environ(),
		"PGHOST="+pgConfig.ConnConfig.Host,
		fmt.Sprintf("PGPORT=%d", pgConfig.ConnConfig.Port),
		"PGUSER="+pgConfig.ConnConfig.User,
		"PGPASSWORD="+pgConfig.ConnConfig.Password,
		"PGDATABASE="+pgConfig.ConnConfig.Database,
		"PGSSLMODE="+mode,
	)
	if cfg.DatabaseCACert != "" {
		caPath := filepath.Join(work, "database-ca.pem")
		if err = os.WriteFile(caPath, []byte(cfg.DatabaseCACert), 0600); err != nil {
			return errors.New("collect: cannot stage database trust certificate")
		}
		env = append(env, "PGSSLROOTCERT="+caPath)
	}
	cmd := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-privileges", "--snapshot="+snapshot, "--file="+dump)
	cmd.Env = env
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	if err = cmd.Run(); err != nil {
		for _, category := range []string{"server version mismatch", "password authentication failed", "certificate verify failed", "no pg_hba.conf entry", "permission denied", "invalid snapshot", "No space left on device", "could not translate host name", "Connection refused"} {
			if strings.Contains(diagnostic.String(), category) {
				return fmt.Errorf("collect: pg_dump failed (%s)", category)
			}
		}
		if cmd.ProcessState == nil {
			return errors.New("collect: pg_dump could not be started")
		}
		return fmt.Errorf("collect: pg_dump failed (exit %d)", cmd.ProcessState.ExitCode())
	}
	if err = tx.Commit(ctx); err != nil {
		return errors.New("collect: cannot complete database snapshot")
	}
	client, err := storage.NewS3Client(cfg.S3Address, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Secure, cfg.S3CACert, cfg.S3Region)
	if err != nil {
		return errors.New("collect: invalid source storage configuration")
	}
	w := tar.NewWriter(dst)
	if err = archiveFile(w, dump, "database.dump"); err != nil {
		return errors.New("collect: cannot archive database dump")
	}
	if err = filepath.WalkDir(settingsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(settingsDir, path)
		if err != nil {
			return err
		}
		return archiveFile(w, path, "config/"+filepath.ToSlash(relative))
	}); err != nil {
		return errors.New("collect: cannot archive release configuration")
	}
	keys := make([]string, 0, len(required))
	for key := range required {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	objects := make([]backedUpObject, 0, len(keys))
	for i, key := range keys {
		saved, err := archiveObject(ctx, w, client, cfg.S3Bucket, key, work)
		if err != nil {
			return fmt.Errorf("%w (object %d of %d)", err, i+1, len(keys))
		}
		objects = append(objects, saved)
	}
	manifest, err := json.Marshal(struct {
		Format   int              `json:"format"`
		Revision string           `json:"source_commit"`
		Objects  []backedUpObject `json:"objects"`
	}{1, revision, objects})
	if err != nil || w.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(manifest))}) != nil {
		return errors.New("collect: cannot create archive manifest")
	}
	if _, err = w.Write(manifest); err != nil || w.Close() != nil {
		return errors.New("collect: cannot finish archive")
	}
	return nil
}

func archiveFile(w *tar.Writer, path, name string) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return errors.New("non-regular recovery source")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = w.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: st.Size()}); err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

func archiveObject(ctx context.Context, w *tar.Writer, client *minio.Client, bucket, key, work string) (backedUpObject, error) {
	object, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return backedUpObject{}, errors.New("collect: cannot open source object")
	}
	defer object.Close()
	f, err := os.CreateTemp(work, "object-")
	if err != nil {
		return backedUpObject{}, errors.New("collect: cannot stage object in private workspace")
	}
	defer f.Close()
	defer os.Remove(f.Name())
	hash := sha256.New()
	bytesRead, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(object, (200<<20)+1))
	if err != nil {
		if minio.ToErrorResponse(err).Code == "AccessDenied" {
			return backedUpObject{}, errors.New("collect: source object read access denied")
		}
		return backedUpObject{}, errors.New("collect: source object read failed")
	}
	if bytesRead <= 0 || bytesRead > 200<<20 {
		return backedUpObject{}, errors.New("collect: downloaded object size is invalid")
	}
	var prefix [512]byte
	n, err := f.ReadAt(prefix[:], 0)
	if err != nil && err != io.EOF {
		return backedUpObject{}, errors.New("collect: cannot read staged object")
	}
	name := fmt.Sprintf("objects/%x", sha256.Sum256([]byte(key)))
	if err = w.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: bytesRead}); err != nil {
		return backedUpObject{}, errors.New("collect: cannot write source object header")
	}
	if n, err := io.Copy(w, io.NewSectionReader(f, 0, bytesRead)); err != nil || n != bytesRead {
		return backedUpObject{}, errors.New("collect: source object download incomplete")
	}
	return backedUpObject{Key: key, ArchivePath: name, ContentType: http.DetectContentType(prefix[:n]), SHA256: fmt.Sprintf("%x", hash.Sum(nil)), Bytes: bytesRead}, nil
}
