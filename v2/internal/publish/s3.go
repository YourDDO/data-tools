package publish

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

const (
	immutableCacheControl = "public, max-age=31536000, immutable"
	latestCacheControl    = "no-cache"
	jsonContentType       = "application/json"
)

// s3PutObjectAPI is the least-privilege subset of the AWS S3 client used by
// publication. The SDK client satisfies this interface, and tests can replace
// it without AWS credentials or network access.
type s3PutObjectAPI interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type S3Store struct {
	client s3PutObjectAPI
	bucket string
}

func NewS3Store(client s3PutObjectAPI, bucket string) (*S3Store, error) {
	if client == nil {
		return nil, fmt.Errorf("S3 client is required")
	}
	if strings.TrimSpace(bucket) == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}
	return &S3Store{client: client, bucket: bucket}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, data []byte, options PutOptions) error {
	if err := validateS3Key(key); err != nil {
		return err
	}
	cacheControl := latestCacheControl
	input := &s3.PutObjectInput{
		Bucket:       aws.String(s.bucket),
		Key:          aws.String(key),
		Body:         bytes.NewReader(data),
		CacheControl: aws.String(cacheControl),
		ContentType:  aws.String(jsonContentType),
	}
	if options.Immutable {
		input.CacheControl = aws.String(immutableCacheControl)
		input.IfNoneMatch = aws.String("*")
	}

	// ContentEncoding is intentionally unset: the publisher currently sends
	// uncompressed JSON bytes. The SDK's standard retryer handles retryable S3
	// and transport failures.
	if _, err := s.client.PutObject(ctx, input); err != nil {
		if options.Immutable && isPreconditionFailed(err) {
			return fmt.Errorf("put S3 object %s: %w", key, ErrAlreadyExists)
		}
		return fmt.Errorf("put S3 object %s: %w", key, err)
	}
	return nil
}

func validateS3Key(key string) error {
	if key == "" || key != path.Clean(key) || key == ".." || strings.HasPrefix(key, "../") || strings.HasPrefix(key, "/") || strings.Contains(key, `\`) {
		return fmt.Errorf("invalid S3 object key %q", key)
	}
	return nil
}

func isPreconditionFailed(err error) bool {
	var apiError smithy.APIError
	return errors.As(err, &apiError) && apiError.ErrorCode() == "PreconditionFailed"
}
