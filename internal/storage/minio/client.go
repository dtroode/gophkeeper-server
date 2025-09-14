package minio

import (
    "context"
    "fmt"
    "io"

    "github.com/minio/minio-go/v7"

    "github.com/dtroode/gophkeeper-server/internal/model"
)

// Internal adapter interface to enable mocking without a real MinIO server.
type minioAPI interface {
    BucketExists(ctx context.Context, bucketName string) (bool, error)
    MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
    PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
    GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error)
    RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
    StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
}

// Wrapper to adapt *minio.Client to minioAPI.
type minioClientWrapper struct{ c *minio.Client }

func (w minioClientWrapper) BucketExists(ctx context.Context, bucketName string) (bool, error) {
    return w.c.BucketExists(ctx, bucketName)
}
func (w minioClientWrapper) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
    return w.c.MakeBucket(ctx, bucketName, opts)
}
func (w minioClientWrapper) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
    return w.c.PutObject(ctx, bucketName, objectName, reader, objectSize, opts)
}
func (w minioClientWrapper) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, error) {
    obj, err := w.c.GetObject(ctx, bucketName, objectName, opts)
    if err != nil {
        return nil, err
    }
    return obj, nil
}
func (w minioClientWrapper) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
    return w.c.RemoveObject(ctx, bucketName, objectName, opts)
}
func (w minioClientWrapper) StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
    return w.c.StatObject(ctx, bucketName, objectName, opts)
}

var _ model.Storage = (*Client)(nil)

type Client struct {
    api    minioAPI
    bucket string
}

// NewClient creates a new MinIO storage client using a real *minio.Client instance.
func NewClient(ctx context.Context, client *minio.Client, bucket string) (*Client, error) {
    return NewClientWithAPI(ctx, minioClientWrapper{c: client}, bucket)
}

// NewClientWithAPI allows injecting a mockable API (used in tests).
func NewClientWithAPI(ctx context.Context, api minioAPI, bucket string) (*Client, error) {
    c := &Client{
        api:    api,
        bucket: bucket,
    }

    // Ensure bucket exists
    err := c.ensureBucketExists(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
    }

    return c, nil
}

// ensureBucketExists creates the bucket if it doesn't exist
func (c *Client) ensureBucketExists(ctx context.Context) error {
    exists, err := c.api.BucketExists(ctx, c.bucket)
    if err != nil {
        return fmt.Errorf("failed to check bucket existence: %w", err)
    }

    if !exists {
        err = c.api.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
        if err != nil {
            return fmt.Errorf("failed to create bucket: %w", err)
        }
    }

    return nil
}

// Upload uploads data to MinIO
func (c *Client) Upload(ctx context.Context, key string, reader io.Reader) error {
    _, err := c.api.PutObject(ctx, c.bucket, key, reader, -1, minio.PutObjectOptions{})
    if err != nil {
        return fmt.Errorf("failed to upload object: %w", err)
    }
    return nil
}

// Download downloads data from MinIO
func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
    obj, err := c.api.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to get object: %w", err)
    }
    return obj, nil
}

// Delete deletes object from MinIO
func (c *Client) Delete(ctx context.Context, key string) error {
    err := c.api.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
    if err != nil {
        return fmt.Errorf("failed to delete object: %w", err)
    }
    return nil
}

// Exists checks if object exists in MinIO
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
    _, err := c.api.StatObject(ctx, c.bucket, key, minio.StatObjectOptions{})
    if err != nil {
        // Check if it's a "not found" error
        if minio.ToErrorResponse(err).Code == "NoSuchKey" {
            return false, nil
        }
        return false, fmt.Errorf("failed to stat object: %w", err)
    }
    return true, nil
}
