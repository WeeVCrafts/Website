package storage

import (
	"context"
	"errors"
)

// CheckHealth verifies that the configured private media bucket is accessible.
func (s *S3) CheckHealth(ctx context.Context) error {
	if s == nil || s.client == nil || s.bucket == "" {
		return errors.New("storage bucket is not configured")
	}
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("storage bucket does not exist")
	}
	return nil
}
