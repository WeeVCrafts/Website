package storage

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/wecratfs/commerce/internal/ports"
)

type S3 struct {
	client *minio.Client
	bucket string
}

func NewS3(endpoint, accessKey, secretKey, bucket string, secure bool) (*S3, error) {
	return NewS3WithCARegion(endpoint, accessKey, secretKey, bucket, secure, "", "")
}

// NewS3WithCA creates an S3 client and optionally trusts a private CA for a
// TLS-terminated, self-managed object store. The default constructor remains
// system-trust-only for local and public S3-compatible providers.
func NewS3WithCA(endpoint, accessKey, secretKey, bucket string, secure bool, caPEM string) (*S3, error) {
	return NewS3WithCARegion(endpoint, accessKey, secretKey, bucket, secure, caPEM, "")
}

// NewS3WithCARegion creates an S3 client with optional private CA trust and an
// explicit signing region for providers such as Tigris and Cloudflare R2.
func NewS3WithCARegion(endpoint, accessKey, secretKey, bucket string, secure bool, caPEM, region string) (*S3, error) {
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
	options := minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: secure, Region: strings.TrimSpace(region)}
	if strings.TrimSpace(caPEM) != "" {
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM([]byte(caPEM)) {
			return nil, errors.New("S3_CA_CERT must contain a valid PEM certificate")
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
		options.Transport = transport
	}
	client, err := minio.New(endpoint, &options)
	if err != nil {
		return nil, err
	}
	return &S3{client: client, bucket: bucket}, nil
}

func (s *S3) CreateUploadURL(ctx context.Context, request ports.UploadRequest) (ports.UploadURL, error) {
	if s.client == nil || s.bucket == "" || !validProductObjectKey(request.ObjectKey) || request.MaxBytes <= 0 || request.MaxBytes > 25<<20 {
		return ports.UploadURL{}, errors.New("invalid upload request")
	}
	presigned, err := s.client.PresignedPutObject(ctx, s.bucket, request.ObjectKey, 15*time.Minute)
	if err != nil {
		return ports.UploadURL{}, err
	}
	return ports.UploadURL{URL: presigned.String(), ExpiresAtUnix: time.Now().Add(15 * time.Minute).Unix()}, nil
}

func (s *S3) InspectObject(ctx context.Context, objectKey string) (ports.ObjectInfo, error) {
	if s.client == nil || s.bucket == "" || !validProductObjectKey(objectKey) {
		return ports.ObjectInfo{}, errors.New("invalid object key")
	}
	info, err := s.client.StatObject(ctx, s.bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return ports.ObjectInfo{}, err
	}
	return ports.ObjectInfo{ContentType: info.ContentType, ByteSize: info.Size}, nil
}

func (s *S3) PutObject(ctx context.Context, objectKey, contentType string, byteSize int64, body io.Reader) error {
	if s.client == nil || s.bucket == "" || !validStoredObjectKey(objectKey) || !allowedStoredMediaType(contentType) || byteSize <= 0 || byteSize > 25<<20 || body == nil {
		return errors.New("invalid object upload")
	}
	_, err := s.client.PutObject(ctx, s.bucket, objectKey, body, byteSize, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *S3) OpenObject(ctx context.Context, objectKey string) (io.ReadCloser, ports.ObjectInfo, error) {
	if s.client == nil || s.bucket == "" || !validStoredObjectKey(objectKey) {
		return nil, ports.ObjectInfo{}, errors.New("invalid object read")
	}
	object, err := s.client.GetObject(ctx, s.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, ports.ObjectInfo{}, boundedStorageError(err)
	}
	stat, err := object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, ports.ObjectInfo{}, boundedStorageError(err)
	}
	return object, ports.ObjectInfo{ContentType: stat.ContentType, ByteSize: stat.Size}, nil
}

type storageReadError struct {
	cause  error
	code   string
	status int
}

func (e storageReadError) Error() string {
	return fmt.Sprintf("object storage read failed (%s)", e.code)
}
func (e storageReadError) Unwrap() error            { return e.cause }
func (e storageReadError) StorageErrorCode() string { return e.code }
func (e storageReadError) StorageHTTPStatus() int   { return e.status }
func boundedStorageError(err error) error {
	response := minio.ToErrorResponse(err)
	code := response.Code
	if len(code) > 48 || code == "" || strings.IndexFunc(code, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') }) >= 0 {
		code = "TransportError"
	}
	return storageReadError{cause: err, code: code, status: response.StatusCode}
}

func allowedStoredMediaType(contentType string) bool {
	value := strings.ToLower(strings.TrimSpace(contentType))
	return value == "image/jpeg" || value == "image/png" || value == "image/webp" || value == "video/mp4" || value == "video/webm"
}

func (s *S3) CreateDownloadURL(ctx context.Context, objectKey string, lifetime time.Duration) (ports.DownloadURL, error) {
	if s.client == nil || s.bucket == "" || !validProductObjectKey(objectKey) || lifetime <= 0 || lifetime > 24*time.Hour {
		return ports.DownloadURL{}, errors.New("invalid download request")
	}
	presigned, err := s.client.PresignedGetObject(ctx, s.bucket, objectKey, lifetime, nil)
	if err != nil {
		return ports.DownloadURL{}, err
	}
	return ports.DownloadURL{URL: presigned.String(), ExpiresAtUnix: time.Now().Add(lifetime).Unix()}, nil
}

func (s *S3) DeleteObject(ctx context.Context, objectKey string) error {
	if s.client == nil || s.bucket == "" || !validStoredObjectKey(objectKey) {
		return errors.New("invalid object key")
	}
	return s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
}

func (s *S3) EnsureBucket(ctx context.Context) error {
	if s.client == nil || s.bucket == "" {
		return errors.New("storage bucket is not configured")
	}
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
}

func ValidateUploadURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func validProductObjectKey(value string) bool {
	return strings.HasPrefix(value, "products/") &&
		!strings.Contains(value, "..") &&
		!strings.HasPrefix(value, "/") &&
		!strings.ContainsAny(value, "\\\n\r\t")
}

var supportObjectKeyPattern = regexp.MustCompile(`^support/[1-9][0-9]*/[a-f0-9]{32}$`)
var avatarObjectKeyPattern = regexp.MustCompile(`^avatars/[1-9][0-9]*/[a-f0-9]{32}$`)
var categoryObjectKeyPattern = regexp.MustCompile(`^categories/[a-f0-9]{32}\.jpg$`)
var storefrontObjectKeyPattern = regexp.MustCompile(`^storefront/item-[a-f0-9]{24}(-(480|960))?\.jpg$`)

// Private support files are streamed through an ownership-checked application
// endpoint. Product presigned upload/download endpoints remain product-only.
func validStoredObjectKey(value string) bool {
	return validProductObjectKey(value) || supportObjectKeyPattern.MatchString(value) || avatarObjectKeyPattern.MatchString(value) || categoryObjectKeyPattern.MatchString(value) || storefrontObjectKeyPattern.MatchString(value)
}
