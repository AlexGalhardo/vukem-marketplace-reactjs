package catalog

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/drivers/s3"
)

// ErrBucketInProduction refuses CreateBucket in production: the production
// bucket is provisioned with the platform (Railway), never by the app.
var ErrBucketInProduction = errors.New("catalog: refusing to create a bucket in production")

// CreateBucket creates the S3_BUCKET on a path-style endpoint (RustFS in
// compose and CI). An existing bucket is not an error.
//
// workaround(vukem): drivers/s3 has no CreateBucket, so this signs the
// PUT /<bucket> request itself (SigV4).
func CreateBucket(ctx context.Context, cfg s3.Config, env config.Env) error {
	if env == config.Production {
		return ErrBucketInProduction
	}

	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return errors.New("catalog: S3_ENDPOINT and S3_BUCKET are required to create a bucket")
	}

	const emptySHA = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	now := time.Now().UTC()
	amzDate, day := now.Format("20060102T150405Z"), now.Format("20060102")

	u, err := url.Parse(strings.TrimSuffix(cfg.Endpoint, "/") + "/" + url.PathEscape(cfg.Bucket))
	if err != nil {
		return fmt.Errorf("catalog: bucket url: %w", err)
	}

	canonical := strings.Join([]string{
		http.MethodPut, u.EscapedPath(), "",
		"host:" + u.Host, "x-amz-content-sha256:" + emptySHA, "x-amz-date:" + amzDate, "",
		"host;x-amz-content-sha256;x-amz-date", emptySHA,
	}, "\n")
	scope := day + "/" + cfg.Region + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])

	key := []byte("AWS4" + cfg.SecretAccessKey.Reveal())
	for _, part := range []string{day, cfg.Region, "s3", "aws4_request"} {
		key = hmacSHA256(key, part)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), http.NoBody)
	if err != nil {
		return err
	}

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", emptySHA)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+cfg.AccessKeyID+"/"+scope+
		", SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="+hex.EncodeToString(hmacSHA256(key, toSign)))

	client := &http.Client{Timeout: 10 * time.Second}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("catalog: create bucket: %w", err)
	}
	defer res.Body.Close()

	// 409 BucketAlreadyOwnedByYou / BucketAlreadyExists: nothing to do.
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusConflict {
		return fmt.Errorf("catalog: create bucket: %s", res.Status)
	}

	return nil
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))

	return m.Sum(nil)
}
