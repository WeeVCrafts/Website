// import-catalog validates actual inventory by default. --apply is the explicit
// opt-in for draft creation; this tool never seeds samples or approves products.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wecratfs/commerce/internal/adapters/postgres"
	"github.com/wecratfs/commerce/internal/adapters/storage"
	"github.com/wecratfs/commerce/internal/application/catalogimport"
	"github.com/wecratfs/commerce/internal/application/commerce"
	"github.com/wecratfs/commerce/internal/config"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	source := flag.String("csv", "", "Inventory CSV using docs/catalog-import-template.csv")
	images := flag.String("images", "", "Directory containing matching product photographs")
	seller := flag.String("seller-username", "seller", "Existing designated seller username")
	admin := flag.String("admin-username", "admin", "Existing Admin or Super Admin username for the audit trail")
	apply := flag.Bool("apply", false, "Create private drafts and upload photographs (default: read-only validation)")
	launch := flag.Bool("require-launch-counts", false, "Require ten primary products in each of the seven launch categories")
	flag.Parse()
	if *source == "" || *images == "" {
		return errors.New("provide --csv and --images; omit --apply for a read-only dry run")
	}
	file, err := os.Open(*source)
	if err != nil {
		return err
	}
	records, err := catalogimport.Read(file)
	file.Close()
	if err != nil {
		return err
	}
	if *launch {
		if err := catalogimport.ValidateLaunchCounts(records); err != nil {
			return err
		}
	}
	// Validate every photo before connecting or making any changes.
	for _, record := range records {
		if _, _, err := photo(*images, record.ImageFile); err != nil {
			return fmt.Errorf("row %d: %w", record.Line, err)
		}
	}
	cfg := config.FromEnv()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	// Migration is owned by normal app startup, never by an import or dry run.
	var actorID, sellerID int64
	if err := pool.QueryRow(ctx, `SELECT u.id FROM users u JOIN user_roles ur ON ur.user_id=u.id WHERE u.username=$1 AND u.status='active' AND ur.role_slug IN('admin','super_admin')`, *admin).Scan(&actorID); err != nil {
		return errors.New("the designated admin account is unavailable; accounts and credentials are never provisioned by the importer")
	}
	if err := pool.QueryRow(ctx, `SELECT s.id FROM sellers s JOIN users u ON u.id=s.owner_user_id AND u.status='active' JOIN user_roles ur ON ur.user_id=u.id AND ur.role_slug='seller' WHERE u.username=$1 AND s.status='active'`, *seller).Scan(&sellerID); err != nil {
		return errors.New("the designated active seller account is unavailable")
	}
	repository := postgres.NewCommerceRepository(pool)
	for _, record := range records {
		if err := repository.CheckMerchantImport(ctx, actorID, sellerID, record.Input); err != nil {
			return fmt.Errorf("row %d: %w", record.Line, err)
		}
	}
	counts := catalogimport.PrimaryCounts(records)
	keys := []string{}
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Printf("Validated %d distinct products for seller %s.\n", len(records), *seller)
	for _, key := range keys {
		fmt.Printf("  %s: %d primary products\n", key, counts[key])
	}
	if !*apply {
		fmt.Println("Dry run complete. No records, stock, credentials or media were changed.")
		return nil
	}
	objectStorage, err := storage.NewS3(cfg.S3Address, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3Secure)
	if err != nil {
		return err
	}
	if err := objectStorage.CheckHealth(ctx); err != nil {
		return errors.New("persistent image storage is unavailable; no drafts have been created")
	}
	media := commerce.NewMediaService(objectStorage, postgres.NewMediaRepository(pool), postgres.NewRoleAuthorizer(pool))
	for _, record := range records {
		id, ready, err := repository.CreateMerchantImport(ctx, actorID, sellerID, record.Input)
		if err != nil {
			return fmt.Errorf("row %d: %w; previously imported drafts are safe to resume", record.Line, err)
		}
		if ready {
			fmt.Printf("Kept existing listing %s (stock unchanged).\n", record.Input.SKU)
			continue
		}
		body, contentType, err := photo(*images, record.ImageFile)
		if err != nil {
			return err
		}
		upload, err := media.PrepareUpload(ctx, actorID, id, contentType, int64(len(body)))
		if err == nil {
			err = media.UploadObject(ctx, actorID, upload.ID, contentType, int64(len(body)), bytes.NewReader(body))
		}
		if err == nil {
			_, err = media.FinalizeUpload(ctx, actorID, upload.ID)
		}
		if err != nil {
			return fmt.Errorf("photo for %s could not be saved: %w; rerun to resume this draft without resetting stock", record.Input.SKU, err)
		}
		fmt.Printf("Prepared draft %s.\n", record.Input.SKU)
	}
	fmt.Println("Import complete. Review drafts in the admin Products workspace before publishing.")
	return nil
}

func photo(directory, name string) ([]byte, string, error) {
	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, "", err
	}
	if filepath.IsAbs(name) {
		return nil, "", errors.New("photo names must be relative to --images")
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, name))
	if err != nil {
		return nil, "", fmt.Errorf("matching photograph %q is unavailable", name)
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, "", errors.New("photo must stay inside --images")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 12 || info.Size() > 25<<20 {
		return nil, "", errors.New("choose a JPEG, PNG or WebP photo up to 25 MiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	dimensions, format, err := image.DecodeConfig(bytes.NewReader(data))
	contentType := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}[format]
	if err != nil || contentType == "" || dimensions.Width < 1 || dimensions.Height < 1 || int64(dimensions.Width)*int64(dimensions.Height) > 40000000 {
		return nil, "", errors.New("photo must be a valid JPEG, PNG or WebP up to 40 megapixels")
	}
	return data, contentType, nil
}
