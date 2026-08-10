// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package objectstorage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Options configures an S3-compatible object storage client.
type Options struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

// Client wraps an S3-compatible client for uploading backup objects.
type Client struct {
	s3 *s3.Client
}

// NewClient creates a new S3-compatible storage client.
// The endpoint must be a full URL (e.g., "https://s3.eu-central-1.amazonaws.com").
// If no region is specified, "eu-central-1" is used as a default.
func NewClient(opts Options) *Client {
	region := opts.Region
	if region == "" {
		region = "eu-central-1"
	}
	svc := s3.New(s3.Options{
		Region:       region,
		Credentials:  credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, ""),
		BaseEndpoint: aws.String(opts.Endpoint),
		UsePathStyle: true,
	})
	return &Client{s3: svc}
}

// Object describes an object in the store.
type Object struct {
	Bucket string
	Key    string
	Body   []byte
	Size   int64
	// LastModified is the time the object was last modified.
	LastModified time.Time
}

// HeadBucket checks whether the bucket exists and is accessible.
func (c *Client) HeadBucket(ctx context.Context, bucket string) error {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to reach bucket s3://%s: %w", bucket, err)
	}
	return nil
}

// PutObject uploads a byte slice to the configured S3-compatible store.
func (c *Client) PutObject(ctx context.Context, obj *Object) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(obj.Bucket),
		Key:    aws.String(obj.Key),
		Body:   bytes.NewReader(obj.Body),
	})
	if err != nil {
		return fmt.Errorf("failed to upload object to s3://%s/%s: %w", obj.Bucket, obj.Key, err)
	}
	return nil
}

// ListObjects returns all objects in the bucket matching the given key prefix.
func (c *Client) ListObjects(ctx context.Context, bucket, prefix string) ([]Object, error) {
	out, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects in s3://%s/%s: %w", bucket, prefix, err)
	}
	objects := make([]Object, len(out.Contents))
	for i, obj := range out.Contents {
		objects[i] = Object{
			Bucket:       bucket,
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
		}
	}
	return objects, nil
}

// DeleteObjects removes the specified objects from the bucket.
func (c *Client) DeleteObjects(ctx context.Context, bucket string, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	objects := make([]s3types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = s3types.ObjectIdentifier{Key: aws.String(key)}
	}
	_, err := c.s3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3types.Delete{Objects: objects, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return fmt.Errorf("failed to delete objects from s3://%s: %w", bucket, err)
	}
	return nil
}
