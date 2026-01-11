package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/aminofox/zenlive/pkg/logger"
)

// S3Storage implements AWS S3 storage backend
type S3Storage struct {
	client *s3.Client
	config StorageConfig
	logger logger.Logger
}

// NewS3Storage creates a new S3 storage backend
func NewS3Storage(cfg StorageConfig, log logger.Logger) (*S3Storage, error) {
	if cfg.Type != StorageTypeS3 {
		return nil, fmt.Errorf("invalid storage type: %s", cfg.Type)
	}

	if log == nil {
		log = logger.NewDefaultLogger(logger.InfoLevel, "text")
	}

	// Load AWS configuration
	var awsConfig aws.Config
	var err error

	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		// Use static credentials
		awsConfig, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cfg.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				"",
			)),
		)
	} else {
		// Use default credential chain
		awsConfig, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cfg.Region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	s3Options := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = true // For S3-compatible services like MinIO
		},
	}

	// Set custom endpoint if provided (for S3-compatible storage)
	if cfg.Endpoint != "" {
		s3Options = append(s3Options, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := s3.NewFromConfig(awsConfig, s3Options...)

	return &S3Storage{
		client: client,
		config: cfg,
		logger: log,
	}, nil
}

// Upload uploads data to S3
func (s *S3Storage) Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	// Read data into buffer (for retry capability)
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, data); err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	// Retry logic
	var lastErr error
	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Warn("Retrying S3 upload",
				logger.Field{Key: "attempt", Value: attempt},
				logger.Field{Key: "key", Value: key},
			)
			time.Sleep(s.config.RetryDelay)
		}

		// Create upload input
		input := &s3.PutObjectInput{
			Bucket:      aws.String(s.config.Bucket),
			Key:         aws.String(s.normalizeKey(key)),
			Body:        bytes.NewReader(buf.Bytes()),
			ContentType: aws.String(contentType),
		}

		// Upload to S3
		_, err := s.client.PutObject(ctx, input)
		if err != nil {
			lastErr = err
			continue
		}

		s.logger.Info("S3 upload completed",
			logger.Field{Key: "bucket", Value: s.config.Bucket},
			logger.Field{Key: "key", Value: key},
			logger.Field{Key: "size", Value: size},
		)

		return nil
	}

	return fmt.Errorf("S3 upload failed after %d attempts: %w", s.config.MaxRetries+1, lastErr)
}

// Download downloads data from S3
func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(s.normalizeKey(key)),
	}

	result, err := s.client.GetObject(ctx, input)
	if err != nil {
		if s.isNotFoundError(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}

	return result.Body, nil
}

// Delete removes an object from S3
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(s.normalizeKey(key)),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		if s.isNotFoundError(err) {
			return ErrObjectNotFound
		}
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	s.logger.Info("S3 object deleted",
		logger.Field{Key: "bucket", Value: s.config.Bucket},
		logger.Field{Key: "key", Value: key},
	)

	return nil
}

// Exists checks if an object exists in S3
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(s.normalizeKey(key)),
	}

	_, err := s.client.HeadObject(ctx, input)
	if err != nil {
		if s.isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// List lists objects in S3 with the given prefix
func (s *S3Storage) List(ctx context.Context, prefix string, maxKeys int) ([]StorageObject, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.config.Bucket),
		Prefix: aws.String(s.normalizeKey(prefix)),
	}

	if maxKeys > 0 {
		input.MaxKeys = aws.Int32(int32(maxKeys))
	}

	objects := make([]StorageObject, 0)

	paginator := s3.NewListObjectsV2Paginator(s.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			metadata, _ := s.GetMetadata(ctx, aws.ToString(obj.Key))

			objects = append(objects, StorageObject{
				Key:          aws.ToString(obj.Key),
				Size:         aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified),
				Metadata:     metadata,
			})

			if maxKeys > 0 && len(objects) >= maxKeys {
				return objects, nil
			}
		}
	}

	return objects, nil
}

// GetMetadata retrieves metadata for an S3 object
func (s *S3Storage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(s.normalizeKey(key)),
	}

	result, err := s.client.HeadObject(ctx, input)
	if err != nil {
		if s.isNotFoundError(err) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}

	metadata := make(map[string]string)
	for k, v := range result.Metadata {
		metadata[k] = v
	}

	// Add standard metadata
	if result.ContentType != nil {
		metadata["content-type"] = *result.ContentType
	}

	return metadata, nil
}

// SetMetadata sets metadata for an S3 object
func (s *S3Storage) SetMetadata(ctx context.Context, key string, metadata map[string]string) error {
	normalizedKey := s.normalizeKey(key)

	// S3 doesn't support updating metadata directly
	// We need to copy the object to itself with new metadata
	copySource := fmt.Sprintf("%s/%s", s.config.Bucket, normalizedKey)

	input := &s3.CopyObjectInput{
		Bucket:            aws.String(s.config.Bucket),
		Key:               aws.String(normalizedKey),
		CopySource:        aws.String(copySource),
		Metadata:          metadata,
		MetadataDirective: types.MetadataDirectiveReplace,
	}

	_, err := s.client.CopyObject(ctx, input)
	if err != nil {
		if s.isNotFoundError(err) {
			return ErrObjectNotFound
		}
		return fmt.Errorf("failed to set S3 metadata: %w", err)
	}

	return nil
}

// Copy copies an object to a new location in S3
func (s *S3Storage) Copy(ctx context.Context, srcKey, dstKey string) error {
	copySource := fmt.Sprintf("%s/%s", s.config.Bucket, s.normalizeKey(srcKey))

	input := &s3.CopyObjectInput{
		Bucket:     aws.String(s.config.Bucket),
		Key:        aws.String(s.normalizeKey(dstKey)),
		CopySource: aws.String(copySource),
	}

	_, err := s.client.CopyObject(ctx, input)
	if err != nil {
		if s.isNotFoundError(err) {
			return ErrObjectNotFound
		}
		return fmt.Errorf("failed to copy S3 object: %w", err)
	}

	s.logger.Info("S3 object copied",
		logger.Field{Key: "source", Value: srcKey},
		logger.Field{Key: "destination", Value: dstKey},
	)

	return nil
}

// GetURL returns a pre-signed URL for accessing the object
func (s *S3Storage) GetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(s.normalizeKey(key)),
	}

	result, err := presignClient.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expires
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate pre-signed URL: %w", err)
	}

	return result.URL, nil
}

// Close closes the S3 storage backend
func (s *S3Storage) Close() error {
	s.logger.Info("S3 storage closed")
	return nil
}

// normalizeKey normalizes an S3 key
func (s *S3Storage) normalizeKey(key string) string {
	// Remove leading slashes
	key = strings.TrimPrefix(key, "/")

	return key
}

// isNotFoundError checks if an error is a "not found" error
func (s *S3Storage) isNotFoundError(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound"
	}
	return false
}
