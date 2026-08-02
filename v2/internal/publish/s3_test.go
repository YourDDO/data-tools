package publish

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type recordingS3Client struct {
	inputs    []*s3.PutObjectInput
	getInputs []*s3.GetObjectInput
	objects   map[string]string
	getErrors map[string]error
	err       error
}

func (c *recordingS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	copy := *input
	c.getInputs = append(c.getInputs, &copy)
	key := aws.ToString(input.Key)
	if err := c.getErrors[key]; err != nil {
		return nil, err
	}
	value, exists := c.objects[key]
	if !exists {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(value))}, nil
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

func TestS3StoreReadsActiveMaster(t *testing.T) {
	t.Parallel()
	const (
		latestKey    = "latest.json"
		manifestKey  = "releases/81.3.0/1/manifest.json"
		masterHash   = "76e7e35aa6cd2ae8620667b5ab1dd275a67cb6138208dd97e5c2c1e5e80ddb5e"
		latestJSON   = `{"gameVersion":"81.3.0","dataVersion":1,"baseUrl":"/releases/81.3.0/1"}`
		manifestJSON = `{"schemaVersion":1,"gameVersion":"81.3.0","dataVersion":1,"masterDatasetSha256":"` + masterHash + `","domains":[],"generatedFiles":[]}`
	)
	client := &recordingS3Client{objects: map[string]string{latestKey: latestJSON, manifestKey: manifestJSON}}
	store, err := NewS3Store(client, "yourddo-data-prod")
	if err != nil {
		t.Fatal(err)
	}
	active, available, err := store.ActiveMasterHash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !available || active.LatestObjectKey != latestKey || active.ActiveManifestKey != manifestKey || active.MasterSHA256 != masterHash {
		t.Fatalf("active = %#v, available = %t", active, available)
	}
	if len(client.getInputs) != 2 || aws.ToString(client.getInputs[0].Key) != latestKey || aws.ToString(client.getInputs[1].Key) != manifestKey {
		t.Fatalf("GetObject calls = %#v", client.getInputs)
	}
}

func TestS3StoreActiveMasterFailureHandling(t *testing.T) {
	t.Parallel()
	const (
		latestKey   = "latest.json"
		manifestKey = "releases/81.3.0/1/manifest.json"
		latestJSON  = `{"gameVersion":"81.3.0","dataVersion":1,"baseUrl":"/releases/81.3.0/1"}`
	)
	tests := []struct {
		name      string
		objects   map[string]string
		getErrors map[string]error
		wantError string
	}{
		{name: "missing latest permits initial publication", objects: map[string]string{}},
		{name: "malformed latest fails", objects: map[string]string{latestKey: "{"}, wantError: "decode active release pointer"},
		{name: "missing manifest fails", objects: map[string]string{latestKey: latestJSON}, wantError: "read active release manifest"},
		{name: "malformed manifest fails", objects: map[string]string{latestKey: latestJSON, manifestKey: "{"}, wantError: "decode active release manifest"},
		{name: "permission failure fails", getErrors: map[string]error{latestKey: &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied", Fault: smithy.FaultClient}}, wantError: "AccessDenied"},
		{name: "network failure fails", getErrors: map[string]error{latestKey: errors.New("connection reset")}, wantError: "connection reset"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client := &recordingS3Client{objects: test.objects, getErrors: test.getErrors}
			store, err := NewS3Store(client, "yourddo-data-prod")
			if err != nil {
				t.Fatal(err)
			}
			_, available, err := store.ActiveMasterHash(context.Background())
			if test.wantError == "" {
				if err != nil || available {
					t.Fatalf("available = %t, error = %v", available, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}
