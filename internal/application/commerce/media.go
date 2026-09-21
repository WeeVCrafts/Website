package commerce

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

var ErrInvalidMedia = errors.New("media upload is invalid")
var ErrMediaNotReady = errors.New("media upload is not ready")

type MediaService struct {
	storage     ports.ObjectStorage
	repository  ports.MediaRepository
	roles       ports.RoleChecker
	permissions ports.SellerPermissionChecker
}

func NewMediaService(storage ports.ObjectStorage, repository ports.MediaRepository, roleCheckers ...ports.RoleChecker) *MediaService {
	service := &MediaService{storage: storage, repository: repository}
	if len(roleCheckers) > 0 {
		service.roles = roleCheckers[0]
		if checker, ok := roleCheckers[0].(ports.SellerPermissionChecker); ok {
			service.permissions = checker
		}
	}
	return service
}

func (s *MediaService) PrepareUpload(ctx context.Context, sellerOwnerID, productID int64, contentType string, byteSize int64) (domaincommerce.MediaUpload, error) {
	if sellerOwnerID <= 0 || productID <= 0 || byteSize <= 0 || byteSize > 25<<20 || !allowedMediaType(contentType) || s.storage == nil || s.repository == nil || !s.canManageProduct(ctx, sellerOwnerID) {
		return domaincommerce.MediaUpload{}, ErrInvalidMedia
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return domaincommerce.MediaUpload{}, err
	}
	objectKey := fmt.Sprintf("products/%d/%s", sellerOwnerID, hex.EncodeToString(bytes))
	mediaID, err := s.repository.PrepareUpload(ctx, sellerOwnerID, productID, objectKey, contentType, byteSize)
	if err != nil {
		return domaincommerce.MediaUpload{}, err
	}
	return domaincommerce.MediaUpload{ID: mediaID, ProductID: productID, ObjectKey: objectKey, ContentType: contentType, MaxBytes: 25 << 20, UploadURL: fmt.Sprintf("/api/v1/seller/products/media-upload/%d", mediaID), ExpiresAtUnix: time.Now().Add(15 * time.Minute).Unix()}, nil
}

func (s *MediaService) UploadObject(ctx context.Context, sellerOwnerID, mediaID int64, contentType string, byteSize int64, body io.Reader) error {
	lifecycle, _, err := s.lifecycle()
	if err != nil || sellerOwnerID <= 0 || mediaID <= 0 || byteSize <= 0 || byteSize > 25<<20 || body == nil || !allowedMediaType(contentType) || !s.canManageProduct(ctx, sellerOwnerID) {
		return ErrInvalidMedia
	}
	record, err := lifecycle.GetOwnedMedia(ctx, sellerOwnerID, mediaID)
	if err != nil {
		return err
	}
	if record.Status != "pending" {
		return ErrMediaNotReady
	}
	if record.ContentType != contentType || record.ByteSize != byteSize {
		return ErrInvalidMedia
	}
	prefixLength := int64(12)
	if byteSize < prefixLength {
		return ErrInvalidMedia
	}
	prefix := make([]byte, prefixLength)
	if _, err := io.ReadFull(body, prefix); err != nil || !matchesMediaSignature(contentType, prefix) {
		return ErrInvalidMedia
	}
	writer, ok := s.storage.(ports.ObjectStorageWriter)
	if !ok {
		return ErrInvalidMedia
	}
	return writer.PutObject(ctx, record.ObjectKey, record.ContentType, record.ByteSize, io.MultiReader(bytes.NewReader(prefix), body))
}

func (s *MediaService) FinalizeUpload(ctx context.Context, sellerOwnerID, mediaID int64) (domaincommerce.MediaRecord, error) {
	lifecycle, reader, err := s.lifecycle()
	if err != nil || sellerOwnerID <= 0 || mediaID <= 0 || !s.canManageProduct(ctx, sellerOwnerID) {
		return domaincommerce.MediaRecord{}, ErrInvalidMedia
	}
	record, err := lifecycle.GetOwnedMedia(ctx, sellerOwnerID, mediaID)
	if err != nil {
		return domaincommerce.MediaRecord{}, err
	}
	if record.Status == "ready" {
		return record, nil
	}
	if record.Status != "pending" {
		return domaincommerce.MediaRecord{}, ErrMediaNotReady
	}
	info, err := reader.InspectObject(ctx, record.ObjectKey)
	if err != nil || info.ByteSize <= 0 || info.ByteSize > 25<<20 || info.ByteSize != record.ByteSize || !allowedMediaType(info.ContentType) || info.ContentType != record.ContentType {
		return domaincommerce.MediaRecord{}, ErrInvalidMedia
	}
	contentReader, ok := s.storage.(ports.ObjectStorageContentReader)
	if !ok {
		return domaincommerce.MediaRecord{}, ErrInvalidMedia
	}
	body, storedInfo, err := contentReader.OpenObject(ctx, record.ObjectKey)
	if err != nil {
		return domaincommerce.MediaRecord{}, ErrInvalidMedia
	}
	defer body.Close()
	prefix := make([]byte, 12)
	if storedInfo.ByteSize != record.ByteSize || storedInfo.ContentType != record.ContentType {
		return domaincommerce.MediaRecord{}, ErrInvalidMedia
	}
	if _, err := io.ReadFull(body, prefix); err != nil || !matchesMediaSignature(record.ContentType, prefix) {
		return domaincommerce.MediaRecord{}, ErrInvalidMedia
	}
	return lifecycle.FinalizeMedia(ctx, sellerOwnerID, mediaID, fmt.Sprintf("/media/products/%d", mediaID), info.ContentType, info.ByteSize)
}

func (s *MediaService) ResolvePublicURL(ctx context.Context, mediaID int64) (string, error) {
	lifecycle, reader, err := s.lifecycle()
	if err != nil || mediaID <= 0 {
		return "", ErrInvalidMedia
	}
	record, err := lifecycle.GetPublicMedia(ctx, mediaID)
	if err != nil {
		return "", err
	}
	download, err := reader.CreateDownloadURL(ctx, record.ObjectKey, 15*time.Minute)
	if err != nil {
		return "", err
	}
	return download.URL, nil
}

func (s *MediaService) OpenPublicMedia(ctx context.Context, mediaID int64) (io.ReadCloser, ports.ObjectInfo, error) {
	lifecycle, _, err := s.lifecycle()
	if err != nil || mediaID <= 0 {
		return nil, ports.ObjectInfo{}, ErrInvalidMedia
	}
	record, err := lifecycle.GetPublicMedia(ctx, mediaID)
	if err != nil {
		return nil, ports.ObjectInfo{}, err
	}
	reader, ok := s.storage.(ports.ObjectStorageContentReader)
	if !ok {
		return nil, ports.ObjectInfo{}, ErrInvalidMedia
	}
	body, info, err := reader.OpenObject(ctx, record.ObjectKey)
	if err != nil {
		return nil, ports.ObjectInfo{}, err
	}
	if info.ContentType != record.ContentType || info.ByteSize != record.ByteSize {
		_ = body.Close()
		return nil, ports.ObjectInfo{}, ErrInvalidMedia
	}
	return body, info, nil
}

func (s *MediaService) DeleteUpload(ctx context.Context, sellerOwnerID, mediaID int64) error {
	lifecycle, reader, err := s.lifecycle()
	if err != nil || sellerOwnerID <= 0 || mediaID <= 0 || !s.canManageProduct(ctx, sellerOwnerID) {
		return ErrInvalidMedia
	}
	record, err := lifecycle.ClaimOwnedMedia(ctx, sellerOwnerID, mediaID)
	if err != nil {
		return err
	}
	if err := reader.DeleteObject(ctx, record.ObjectKey); err != nil {
		_ = lifecycle.RestoreMedia(ctx, record.ID)
		return err
	}
	return lifecycle.MarkMediaDeleted(ctx, mediaID)
}

func (s *MediaService) canManageProduct(ctx context.Context, userID int64) bool {
	if s.roles == nil {
		return true
	}
	allowed, err := s.roles.HasAnyRole(ctx, userID, domainidentity.RoleSuperAdmin, domainidentity.RoleSellerOwner)
	if err == nil && allowed {
		return true
	}
	if s.permissions == nil {
		return false
	}
	allowed, err = s.permissions.HasSellerPermission(ctx, userID, domaincommerce.SellerPermissionProductWrite)
	return err == nil && allowed
}

func (s *MediaService) CleanupExpired(ctx context.Context, limit int) (int, error) {
	lifecycle, reader, err := s.lifecycle()
	if err != nil {
		return 0, ErrInvalidMedia
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	media, err := lifecycle.ClaimExpiredMedia(ctx, time.Now().Add(-time.Hour), limit)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, record := range media {
		if err := reader.DeleteObject(ctx, record.ObjectKey); err != nil {
			_ = lifecycle.RestoreMedia(ctx, record.ID)
			continue
		}
		if err := lifecycle.MarkMediaDeleted(ctx, record.ID); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func (s *MediaService) lifecycle() (ports.MediaLifecycleRepository, ports.ObjectStorageReader, error) {
	lifecycle, ok := s.repository.(ports.MediaLifecycleRepository)
	if !ok || s.storage == nil {
		return nil, nil, ErrInvalidMedia
	}
	reader, ok := s.storage.(ports.ObjectStorageReader)
	if !ok {
		return nil, nil, ErrInvalidMedia
	}
	return lifecycle, reader, nil
}

func allowedMediaType(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "image/jpeg" || value == "image/png" || value == "image/webp" || value == "video/mp4" || value == "video/webm"
}

func matchesMediaSignature(contentType string, prefix []byte) bool {
	if len(prefix) < 12 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return prefix[0] == 0xff && prefix[1] == 0xd8 && prefix[2] == 0xff
	case "image/png":
		return bytes.Equal(prefix[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	case "image/webp":
		return string(prefix[:4]) == "RIFF" && string(prefix[8:12]) == "WEBP"
	case "video/mp4":
		return string(prefix[4:8]) == "ftyp"
	case "video/webm":
		return bytes.Equal(prefix[:4], []byte{0x1a, 0x45, 0xdf, 0xa3})
	default:
		return false
	}
}
