// Package r2 wraps the AWS S3 SDK for Cloudflare R2 (or any S3-compatible store).
package r2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Client struct {
	s3       *s3.Client
	bucket   string
	cdnBase  string
	endpoint string
}

type Options struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Region          string
	CDNBaseURL      string
}

func New(ctx context.Context, opts Options) (*Client, error) {
	if opts.Endpoint == "" || opts.Bucket == "" {
		return nil, errors.New("r2: endpoint and bucket required")
	}
	region := opts.Region
	if region == "" {
		region = "auto"
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			opts.AccessKeyID, opts.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("r2: load config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(opts.Endpoint)
		o.UsePathStyle = true
	})

	return &Client{
		s3:       s3Client,
		bucket:   opts.Bucket,
		cdnBase:  strings.TrimRight(opts.CDNBaseURL, "/"),
		endpoint: opts.Endpoint,
	}, nil
}

// CDNURL builds the public CDN URL for a given key. Returns "" if no CDN base configured.
func (c *Client) CDNURL(key string) string {
	if c.cdnBase == "" {
		return ""
	}
	return c.cdnBase + "/" + strings.TrimLeft(key, "/")
}

// Bucket exposes the configured bucket name.
func (c *Client) Bucket() string { return c.bucket }

// PutFile uploads a local file to R2 with public-cache headers.
// The verify flag does a HeadObject after upload to confirm size.
func (c *Client) PutFile(ctx context.Context, localPath, key, contentType string, verify bool) (int64, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return 0, fmt.Errorf("r2: open %s: %w", localPath, err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return 0, err
	}
	size := stat.Size()
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          f,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
		CacheControl:  aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return 0, fmt.Errorf("r2: put %s: %w", key, err)
	}

	if verify {
		head, headErr := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(c.bucket),
			Key:    aws.String(key),
		})
		if headErr != nil {
			return 0, fmt.Errorf("r2: head verify %s: %w", key, headErr)
		}
		remote := aws.ToInt64(head.ContentLength)
		if remote < 1 || (size > 0 && abs64(remote-size) > 1024) {
			return 0, fmt.Errorf("r2: verify size mismatch (remote=%d local=%d)", remote, size)
		}
		return remote, nil
	}

	return size, nil
}

// GetToFile downloads an object to localPath. Parent dirs are created.
func (c *Client) GetToFile(ctx context.Context, key, localPath string) (int64, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("r2: get %s: %w", key, err)
	}
	defer out.Body.Close()

	if dir := path.Dir(localPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, err
		}
	}
	tmp := localPath + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(f, out.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return 0, closeErr
	}
	if err := os.Rename(tmp, localPath); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return n, nil
}

// Delete removes a single object. Returns nil even if the key didn't exist.
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}

// DeleteBatch removes up to 1000 objects in a single S3 DeleteObjects call.
// S3 caps a single batch at 1000 keys; callers chunk larger lists.
func (c *Client) DeleteBatch(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if len(keys) > 1000 {
		return fmt.Errorf("r2: DeleteBatch limit is 1000 keys, got %d", len(keys))
	}
	objs := make([]s3types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objs[i] = s3types.ObjectIdentifier{Key: aws.String(k)}
	}
	out, err := c.s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(c.bucket),
		Delete: &s3types.Delete{Objects: objs, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return err
	}
	if len(out.Errors) > 0 {
		return fmt.Errorf("r2: %d delete errors (first: %s: %s)",
			len(out.Errors),
			aws.ToString(out.Errors[0].Key),
			aws.ToString(out.Errors[0].Message))
	}
	return nil
}

// Head returns object size or zero if not found (and !exists).
func (c *Client) Head(ctx context.Context, key string) (size int64, exists bool, err error) {
	out, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nf *s3types.NotFound
		if errors.As(err, &nf) {
			return 0, false, nil
		}
		// Some S3 implementations return generic 404; fall back to string match.
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
			return 0, false, nil
		}
		return 0, false, err
	}
	return aws.ToInt64(out.ContentLength), true, nil
}

// ListAll iterates every object under a prefix and yields it to the callback.
// Returning an error from cb halts the iteration.
type ObjectInfo struct {
	Key  string
	Size int64
}

func (c *Client) ListAll(ctx context.Context, prefix string, cb func(ObjectInfo) error) error {
	var token *string
	for {
		out, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(c.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return fmt.Errorf("r2: list %q: %w", prefix, err)
		}
		for _, o := range out.Contents {
			if err := cb(ObjectInfo{
				Key:  aws.ToString(o.Key),
				Size: aws.ToInt64(o.Size),
			}); err != nil {
				return err
			}
		}
		if out.IsTruncated == nil || !*out.IsTruncated {
			break
		}
		token = out.NextContinuationToken
	}
	return nil
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
