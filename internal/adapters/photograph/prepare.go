package photograph

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"github.com/wecratfs/commerce/internal/ports"
)

type Storage interface {
	ports.ObjectStorageContentReader
	ports.ObjectStorageWriter
}

func Prepare(ctx context.Context, store Storage, repository ports.MediaDerivativeRepository) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	records, err := repository.DerivativeCandidates(ctx, 2)
	if err != nil {
		return err
	}
	for _, media := range records {
		if media.ByteSize <= 0 || media.ByteSize > 25<<20 {
			continue
		}
		body, info, err := store.OpenObject(ctx, media.ObjectKey)
		if err != nil {
			return fmt.Errorf("image source unavailable")
		}
		data, readErr := io.ReadAll(io.LimitReader(body, media.ByteSize+1))
		_ = body.Close()
		if readErr != nil || info.ByteSize != media.ByteSize || info.ContentType != media.ContentType || int64(len(data)) != media.ByteSize {
			return fmt.Errorf("image source metadata mismatch")
		}
		identity := sha256.Sum256([]byte(fmt.Sprintf("v1/%d/%s/%s/%d", media.ID, media.ObjectKey, media.ContentType, media.ByteSize)))
		for _, width := range []int{320, 640, 960} {
			if _, err := repository.PublicDerivative(ctx, media, width); err == nil {
				continue
			}
			resized, err := Resize(data, width)
			if err != nil {
				continue
			}
			contentType := media.ContentType
			if len(resized) < len(data) {
				contentType = "image/jpeg"
			} else {
				resized = data
			}
			if len(resized) > 1<<20 {
				continue
			}
			key := fmt.Sprintf("products/derived/%x/%d", identity, width)
			if err := store.PutObject(ctx, key, contentType, int64(len(resized)), bytes.NewReader(resized)); err != nil {
				return fmt.Errorf("image derivative storage unavailable")
			}
			if err := repository.SaveDerivative(ctx, ports.MediaDerivative{MediaID: media.ID, Width: width, SourceKey: media.ObjectKey, SourceType: media.ContentType, SourceBytes: media.ByteSize, ObjectKey: key, ContentType: contentType, ByteSize: int64(len(resized))}); err != nil {
				return err
			}
		}
	}
	return nil
}
