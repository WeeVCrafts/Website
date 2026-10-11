package commerce

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	domaincommerce "github.com/wecratfs/commerce/internal/domain/commerce"
	domainidentity "github.com/wecratfs/commerce/internal/domain/identity"
	"github.com/wecratfs/commerce/internal/ports"
)

const (
	maxSupportAttachments    = 3
	maxSupportAttachmentSize = 10 << 20
)

var ErrInvalidSupportAttachment = errors.New("support attachment is invalid")

type SupportAttachmentService struct {
	storage    ports.ObjectStorage
	repository ports.SupportAttachmentRepository
	roles      ports.RoleChecker
}

func NewSupportAttachmentService(storage ports.ObjectStorage, repository ports.SupportAttachmentRepository, roles ports.RoleChecker) *SupportAttachmentService {
	return &SupportAttachmentService{storage: storage, repository: repository, roles: roles}
}

// Validate checks every selected file before a ticket or reply is persisted.
// Store repeats the check because it is also the boundary used by callers
// outside the HTTP transport.
func (s *SupportAttachmentService) Validate(headers []*multipart.FileHeader) error {
	if len(headers) > maxSupportAttachments {
		return ErrInvalidSupportAttachment
	}
	for _, header := range headers {
		if header == nil || header.Size <= 0 || header.Size > maxSupportAttachmentSize {
			return ErrInvalidSupportAttachment
		}
		file, err := header.Open()
		if err != nil {
			return ErrInvalidSupportAttachment
		}
		_, _, readErr := readSupportAttachment(file, header.Size)
		_ = file.Close()
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

func (s *SupportAttachmentService) Store(ctx context.Context, actorID, ticketID, messageID int64, headers []*multipart.FileHeader) error {
	if len(headers) == 0 {
		return nil
	}
	if err := s.Validate(headers); err != nil {
		return err
	}
	writer, ok := s.storage.(ports.ObjectStorageWriter)
	if !ok || s.repository == nil || actorID <= 0 || ticketID <= 0 {
		return ErrInvalidSupportAttachment
	}
	if err := s.authorizeTicket(ctx, actorID, ticketID); err != nil {
		return err
	}
	for _, header := range headers {
		file, err := header.Open()
		if err != nil {
			return ErrInvalidSupportAttachment
		}
		body, contentType, readErr := readSupportAttachment(file, header.Size)
		_ = file.Close()
		if readErr != nil {
			return readErr
		}
		key, keyErr := supportAttachmentKey(actorID)
		if keyErr != nil {
			return keyErr
		}
		if err := writer.PutObject(ctx, key, contentType, int64(len(body)), bytes.NewReader(body)); err != nil {
			return fmt.Errorf("store support attachment: %w", err)
		}
		if _, err := s.repository.CreateSupportAttachment(ctx, ticketID, messageID, actorID, filepath.Base(header.Filename), contentType, int64(len(body)), key); err != nil {
			if reader, ok := s.storage.(ports.ObjectStorageReader); ok {
				_ = reader.DeleteObject(ctx, key)
			}
			return err
		}
	}
	return nil
}

func (s *SupportAttachmentService) List(ctx context.Context, ticketID int64) ([]domaincommerce.SupportAttachment, error) {
	if s.repository == nil || ticketID <= 0 {
		return nil, ErrInvalidSupportAttachment
	}
	return s.repository.ListSupportAttachments(ctx, ticketID)
}

func (s *SupportAttachmentService) Open(ctx context.Context, actorID, attachmentID int64) (io.ReadCloser, ports.ObjectInfo, error) {
	if s.repository == nil || actorID <= 0 || attachmentID <= 0 {
		return nil, ports.ObjectInfo{}, ErrInvalidSupportAttachment
	}
	attachment, err := s.repository.GetSupportAttachment(ctx, attachmentID)
	if err != nil {
		return nil, ports.ObjectInfo{}, err
	}
	if err := s.authorizeTicket(ctx, actorID, attachment.TicketID); err != nil {
		return nil, ports.ObjectInfo{}, err
	}
	reader, ok := s.storage.(ports.ObjectStorageContentReader)
	if !ok {
		return nil, ports.ObjectInfo{}, ErrInvalidSupportAttachment
	}
	body, info, err := reader.OpenObject(ctx, attachment.ObjectKey)
	if err != nil {
		return nil, ports.ObjectInfo{}, err
	}
	return body, info, nil
}

func (s *SupportAttachmentService) authorizeTicket(ctx context.Context, actorID, ticketID int64) error {
	owned, err := s.repository.OwnsSupportTicket(ctx, actorID, ticketID)
	if err != nil {
		return err
	}
	if owned {
		return nil
	}
	if s.roles != nil {
		allowed, err := s.roles.HasAnyRole(ctx, actorID, domainidentity.RoleAdmin, domainidentity.RoleSuperAdmin)
		if err != nil {
			return err
		}
		if allowed {
			return nil
		}
	}
	return ports.ErrForbidden
}

func readSupportAttachment(file multipart.File, declaredSize int64) ([]byte, string, error) {
	if declaredSize <= 0 || declaredSize > maxSupportAttachmentSize {
		return nil, "", ErrInvalidSupportAttachment
	}
	body, err := io.ReadAll(io.LimitReader(file, maxSupportAttachmentSize+1))
	if err != nil || len(body) == 0 || len(body) > maxSupportAttachmentSize {
		return nil, "", ErrInvalidSupportAttachment
	}
	contentType := http.DetectContentType(body)
	if strings.HasPrefix(contentType, "application/octet-stream") && len(body) >= 12 && string(body[0:4]) == "RIFF" && string(body[8:12]) == "WEBP" {
		contentType = "image/webp"
	}
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		return nil, "", ErrInvalidSupportAttachment
	}
	return body, contentType, nil
}

func supportAttachmentKey(actorID int64) (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return fmt.Sprintf("support/%d/%s", actorID, hex.EncodeToString(buffer)), nil
}
