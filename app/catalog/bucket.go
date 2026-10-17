package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/drivers/s3"
)

// ErrBucketInProduction refuses CreateBucket in production: the production
// bucket is provisioned with the platform (Railway), never by the app.
var ErrBucketInProduction = errors.New("catalog: refusing to create a bucket in production")

// CreateBucket creates the S3_BUCKET on a local S3 (RustFS in compose and
// CI). An existing bucket the credentials own is not an error.
func CreateBucket(ctx context.Context, cfg s3.Config, env config.Env) error {
	if env == config.Production {
		return ErrBucketInProduction
	}

	disk, err := s3.New(cfg)
	if err != nil {
		return fmt.Errorf("catalog: %w", err)
	}

	if err := disk.CreateBucket(ctx); err != nil {
		return fmt.Errorf("catalog: create bucket: %w", err)
	}

	return nil
}
