package publish

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	immutableCacheControl = "public, max-age=31536000, immutable"
	latestCacheControl    = "no-cache"
	jsonContentType       = "application/json"
)

// s3ObjectAPI is the least-privilege subset of the AWS S3 client used by
// publication. The SDK client satisfies this interface, and tests can replace
// it without AWS credentials or network access.
type s3ObjectAPI interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type S3Store struct {
	client s3ObjectAPI
	bucket string
}

func NewS3Store(client s3ObjectAPI, bucket string) (*S3Store, error) {
	if client == nil {
		return nil, fmt.Errorf("S3 client is required")
	}
	if strings.TrimSpace(bucket) == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}
	return &S3Store{client: client, bucket: bucket}, nil
}

func (s *S3Store) get(ctx context.Context, key string) ([]byte, error) {
	if err := validateS3Key(key); err != nil {
		return nil, err
	}
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, fmt.Errorf("get S3 object %s: %w", key, err)
	}
	if output.Body == nil {
		return nil, fmt.Errorf("get S3 object %s: response body is missing", key)
	}
	data, readErr := io.ReadAll(output.Body)
	closeErr := output.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read S3 object %s: %w", key, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close S3 object %s: %w", key, closeErr)
	}
	return data, nil
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

func isNoSuchKey(err error) bool {
	if err == nil {
		return false
	}
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiError smithy.APIError
	return errors.As(err, &apiError) && apiError.ErrorCode() == "NoSuchKey"
}
