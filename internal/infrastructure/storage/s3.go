package storage

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	"go-api/internal/domain/port"
	"go-api/internal/infrastructure/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Store is an S3-compatible BodyStore (MinIO, AWS S3, …).
type S3Store struct {
	client *s3.Client
	bucket string
}

var _ port.BodyStore = (*S3Store)(nil)

func NewS3Store(ctx context.Context, cfg config.StorageConfig) (*S3Store, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("storage endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("storage bucket is required")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("storage access key and secret key are required")
	}

	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	return &S3Store{client: client, bucket: cfg.Bucket}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	key = strings.TrimLeft(key, "/")
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		return "", fmt.Errorf("put object %s: %w", key, err)
	}
	return fmt.Sprintf("s3://%s/%s", s.bucket, key), nil
}

// ParseRef splits an s3://bucket/key reference.
func ParseRef(ref string) (bucket, key string, err error) {
	u, err := url.Parse(ref)
	if err != nil || u.Scheme != "s3" || u.Host == "" || u.Path == "" {
		return "", "", fmt.Errorf("invalid body ref %q", ref)
	}
	return u.Host, strings.TrimLeft(u.Path, "/"), nil
}
