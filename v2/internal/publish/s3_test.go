package publish

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type recordingS3Client struct {
	inputs []*s3.PutObjectInput
	err    error
}

func (c *recordingS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	copy := *input
	copy.Body = strings.NewReader(string(body))
	c.inputs = append(c.inputs, &copy)
	if c.err != nil {
		return nil, c.err
	}
	return &s3.PutObjectOutput{}, nil
}

func TestS3StoreSetsObjectHeadersAndImmutableCondition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		key          string
		options      PutOptions
		cacheControl string
		ifNoneMatch  string
	}{
		{
			name:         "immutable release",
			key:          "releases/81.3.0/1785175200/manifest.json",
			options:      PutOptions{Immutable: true},
			cacheControl: immutableCacheControl,
			ifNoneMatch:  "*",
		},
		{
			name:         "latest pointer",
			key:          "latest.json",
			cacheControl: latestCacheControl,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client := &recordingS3Client{}
			store, err := NewS3Store(client, "yourddo-data-prod")
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Put(context.Background(), test.key, []byte("{}\n"), test.options); err != nil {
				t.Fatal(err)
			}
			if len(client.inputs) != 1 {
				t.Fatalf("PutObject calls = %d", len(client.inputs))
			}
			input := client.inputs[0]
			if value(input.Bucket) != "yourddo-data-prod" || value(input.Key) != test.key {
				t.Fatalf("target = %q/%q", value(input.Bucket), value(input.Key))
			}
			if value(input.CacheControl) != test.cacheControl || value(input.ContentType) != jsonContentType {
				t.Fatalf("headers = Cache-Control %q, Content-Type %q", value(input.CacheControl), value(input.ContentType))
			}
			if value(input.IfNoneMatch) != test.ifNoneMatch {
				t.Fatalf("If-None-Match = %q", value(input.IfNoneMatch))
			}
			if input.ContentEncoding != nil {
				t.Fatalf("Content-Encoding = %q for uncompressed JSON", value(input.ContentEncoding))
			}
			if input.ACL != "" {
				t.Fatalf("ACL = %q; publisher must not make objects public", input.ACL)
			}
		})
	}
}

func TestS3StoreProtectsExistingImmutableObject(t *testing.T) {
	t.Parallel()
	client := &recordingS3Client{err: &smithy.GenericAPIError{Code: "PreconditionFailed", Message: "object exists", Fault: smithy.FaultClient}}
	store, err := NewS3Store(client, "yourddo-data-prod")
	if err != nil {
		t.Fatal(err)
	}
	key := "releases/81.3.0/1785175200/master/items.json"
	err = store.Put(context.Background(), key, []byte("[]\n"), PutOptions{Immutable: true})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("error = %v, want ErrAlreadyExists", err)
	}
	if !strings.Contains(err.Error(), key) {
		t.Fatalf("error does not identify object key: %v", err)
	}
}

func TestS3StoreRejectsUnsafeKeyBeforeCallingAWS(t *testing.T) {
	t.Parallel()
	client := &recordingS3Client{}
	store, err := NewS3Store(client, "yourddo-data-prod")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(context.Background(), "../latest.json", nil, PutOptions{}); err == nil {
		t.Fatal("Put succeeded with unsafe key")
	}
	if len(client.inputs) != 0 {
		t.Fatal("unsafe key reached S3 client")
	}
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}
