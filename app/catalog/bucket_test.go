package catalog

import (
	"errors"
	"testing"

	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/drivers/s3"
)

func TestCreateBucketRefusesProduction(t *testing.T) {
	t.Parallel()

	// A closed port: a request would fail with a different error.
	cfg := s3.Config{Endpoint: "http://127.0.0.1:1", Bucket: "b", Region: "us-east-1", InsecureHTTP: true}

	if err := CreateBucket(t.Context(), cfg, config.Production); !errors.Is(err, ErrBucketInProduction) {
		t.Fatalf("err = %v, want ErrBucketInProduction", err)
	}
}
