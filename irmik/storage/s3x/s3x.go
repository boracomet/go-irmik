// Package s3x provides an S3-compatible storage.Store using the AWS SDK v2.
//
// Opt-in (keeps AWS SDK out of binaries that do not import this package):
//
//	store, err := s3x.Open(ctx, s3x.Options{Bucket: "my-bucket", Region: "us-east-1"})
//
// Credentials follow the default AWS SDK chain (env, shared config, IAM role).
package s3x

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/boracomet/go-irmik/irmik/storage"
)

// Options configures the S3 client.
type Options struct {
	Bucket   string
	Region   string
	// Endpoint overrides the S3 API endpoint (MinIO, R2, etc.).
	Endpoint string
	// PathStyle forces path-style addressing (often required for MinIO).
	PathStyle bool
	// Prefix is prepended to every key.
	Prefix string
}

type store struct {
	client *s3.Client
	bucket string
	prefix string
}

// Open builds an S3-backed storage.Store.
func Open(ctx context.Context, opts Options) (storage.Store, error) {
	if opts.Bucket == "" {
		return nil, fmt.Errorf("s3x: bucket is required")
	}
	loadOpts := []func(*config.LoadOptions) error{}
	if opts.Region != "" {
		loadOpts = append(loadOpts, config.WithRegion(opts.Region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3x: load config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if opts.Endpoint != "" {
			o.BaseEndpoint = aws.String(opts.Endpoint)
		}
		o.UsePathStyle = opts.PathStyle
	})
	return &store{client: client, bucket: opts.Bucket, prefix: strings.Trim(opts.Prefix, "/")}, nil
}

func (s *store) key(k string) string {
	k = strings.TrimPrefix(k, "/")
	if s.prefix == "" {
		return k
	}
	return s.prefix + "/" + k
}

func (s *store) Put(ctx context.Context, key string, r io.Reader, contentType string) (storage.Object, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.key(key)),
		Body:        r,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return storage.Object{}, err
	}
	st, err := s.Stat(ctx, key)
	if err != nil {
		return storage.Object{Key: key, ContentType: contentType}, nil
	}
	return st, nil
}

func (s *store) Get(ctx context.Context, key string) (io.ReadCloser, storage.Object, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(key)),
	})
	if err != nil {
		return nil, storage.Object{}, mapNotFound(err)
	}
	ct := "application/octet-stream"
	if out.ContentType != nil {
		ct = *out.ContentType
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return out.Body, storage.Object{Key: key, Size: size, ContentType: ct}, nil
}

func (s *store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(key)),
	})
	return err
}

func (s *store) Stat(ctx context.Context, key string) (storage.Object, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(key)),
	})
	if err != nil {
		return storage.Object{}, mapNotFound(err)
	}
	ct := "application/octet-stream"
	if out.ContentType != nil {
		ct = *out.ContentType
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return storage.Object{Key: key, Size: size, ContentType: ct}, nil
}

func mapNotFound(err error) error {
	var nsk *types.NoSuchKey
	var nf *types.NotFound
	if errors.As(err, &nsk) || errors.As(err, &nf) {
		return storage.ErrNotFound
	}
	// SDK sometimes wraps 404 as generic API error; keep original otherwise.
	msg := err.Error()
	if strings.Contains(msg, "NotFound") || strings.Contains(msg, "404") || strings.Contains(msg, "NoSuchKey") {
		return storage.ErrNotFound
	}
	return err
}
