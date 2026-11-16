package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/wecratfs/commerce/internal/adapters/storage"
)

var archiveName = regexp.MustCompile(`^weevcrafts-ops-[0-9]{8}T[0-9]{6}Z-[0-9]+\.tar\.age$`)
var archiveDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func offsiteClient() (*minio.Client, string, error) {
	endpoint, bucket := os.Getenv("BACKUP_S3_ENDPOINT"), os.Getenv("BACKUP_S3_BUCKET")
	access, secret := os.Getenv("BACKUP_S3_ACCESS_KEY"), os.Getenv("BACKUP_S3_SECRET_KEY")
	if !strings.HasSuffix(endpoint, ".r2.cloudflarestorage.com") || bucket == "" || access == "" || secret == "" {
		return nil, "", errors.New("offsite: private R2 configuration incomplete")
	}
	client, err := storage.NewS3Client(endpoint, access, secret, true, "", "auto")
	if err != nil {
		return nil, "", errors.New("offsite: cannot configure destination client")
	}
	return client, bucket, nil
}

func upload(ctx context.Context, archive string) error {
	base := filepath.Base(archive)
	if !archiveName.MatchString(base) {
		return errors.New("upload: expected a scheduled encrypted archive")
	}
	client, bucket, err := offsiteClient()
	if err != nil {
		return err
	}
	f, err := os.Open(archive)
	if err != nil {
		return errors.New("upload: cannot open archive")
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() < 64 {
		return errors.New("upload: invalid archive file")
	}
	start := make([]byte, len("age-encryption.org/v1\n"))
	if _, err = io.ReadFull(f, start); err != nil || string(start) != "age-encryption.org/v1\n" {
		return errors.New("upload: expected age ciphertext")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return errors.New("upload: cannot rewind encrypted archive")
	}
	key := "daily/" + base
	hash := sha256.New()
	if _, err = io.Copy(hash, f); err != nil {
		return errors.New("upload: cannot hash local archive")
	}
	want := fmt.Sprintf("%x", hash.Sum(nil))
	info, err := client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if code := minio.ToErrorResponse(err).Code; code != "NoSuchKey" && code != "NotFound" {
			return errors.New("upload: cannot check destination object")
		}
		if _, err = client.FPutObject(ctx, bucket, key, archive, minio.PutObjectOptions{ContentType: "application/octet-stream", UserMetadata: map[string]string{"sha256": want}}); err != nil {
			return errors.New("upload: destination write failed")
		}
	} else if info.Size != st.Size() {
		return errors.New("upload: destination already has a different archive")
	}
	obj, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return errors.New("verify: cannot open remote archive")
	}
	defer obj.Close()
	hash.Reset()
	n, err := io.Copy(hash, obj)
	if err != nil || n != st.Size() || fmt.Sprintf("%x", hash.Sum(nil)) != want {
		return errors.New("verify: remote size or SHA-256 mismatch")
	}
	fmt.Printf("offsite_verified bytes=%d sha256=%s\n", n, want)
	return nil
}

func download(ctx context.Context, archive, expected string) error {
	base := filepath.Base(archive)
	if !archiveName.MatchString(base) || !archiveDigest.MatchString(expected) {
		return errors.New("download: invalid archive name or checksum")
	}
	client, bucket, err := offsiteClient()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(archive, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("download: cannot create output file")
	}
	accepted := false
	defer func() {
		_ = f.Close()
		if !accepted {
			_ = os.Remove(archive)
		}
	}()
	obj, err := client.GetObject(ctx, bucket, "daily/"+base, minio.GetObjectOptions{})
	if err != nil {
		return errors.New("download: cannot open source object")
	}
	defer obj.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash), obj)
	if err != nil || n < 64 || fmt.Sprintf("%x", hash.Sum(nil)) != expected || f.Sync() != nil {
		return errors.New("download: archive bytes or SHA-256 could not be verified")
	}
	accepted = true
	fmt.Printf("offsite_download_verified bytes=%d sha256=%s\n", n, expected)
	return nil
}
