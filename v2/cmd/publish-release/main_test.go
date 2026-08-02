package main

import (
	"context"
	"strings"
	"testing"
)

func TestProductionRequiresS3(t *testing.T) {
	t.Parallel()
	err := run(context.Background(), []string{"--environment=production", "--backend=local", "--destination=/tmp/unused"})
	if err == nil || !strings.Contains(err.Error(), "requires the s3 backend") {
		t.Fatalf("error = %v", err)
	}
}

func TestS3BackendRequiresDeploymentConfiguration(t *testing.T) {
	t.Parallel()
	err := run(context.Background(), []string{"--environment=production", "--backend=s3", "--region=", "--bucket="})
	if err == nil || !strings.Contains(err.Error(), "AWS_REGION") || !strings.Contains(err.Error(), "DATA_BUCKET") {
		t.Fatalf("error = %v", err)
	}
}
