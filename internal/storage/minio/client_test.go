package minio

import (
    "bytes"
    "context"
    "errors"
    "io"
    "testing"

    minioLib "github.com/minio/minio-go/v7"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// fakeMinio implements minioAPI for testing without network.
type fakeMinio struct {
    bucketExists    bool
    bucketExistsErr error
    makeBucketErr   error

    putInfo minioLib.UploadInfo
    putErr  error

    getRC  io.ReadCloser
    getErr error

    removeErr error

    statInfo       minioLib.ObjectInfo
    statErr        error
}

func (f *fakeMinio) BucketExists(_ context.Context, _ string) (bool, error) {
    return f.bucketExists, f.bucketExistsErr
}
func (f *fakeMinio) MakeBucket(_ context.Context, _ string, _ minioLib.MakeBucketOptions) error {
    return f.makeBucketErr
}
func (f *fakeMinio) PutObject(_ context.Context, _ string, _ string, _ io.Reader, _ int64, _ minioLib.PutObjectOptions) (minioLib.UploadInfo, error) {
    return f.putInfo, f.putErr
}
func (f *fakeMinio) GetObject(_ context.Context, _ string, _ string, _ minioLib.GetObjectOptions) (io.ReadCloser, error) {
    return f.getRC, f.getErr
}
func (f *fakeMinio) RemoveObject(_ context.Context, _ string, _ string, _ minioLib.RemoveObjectOptions) error {
    return f.removeErr
}
func (f *fakeMinio) StatObject(_ context.Context, _ string, _ string, _ minioLib.StatObjectOptions) (minioLib.ObjectInfo, error) {
    return f.statInfo, f.statErr
}

func TestNewClientWithAPI_BucketExists(t *testing.T) {
    ctx := context.Background()
    api := &fakeMinio{bucketExists: true}
    c, err := NewClientWithAPI(ctx, api, "b")
    require.NoError(t, err)
    assert.NotNil(t, c)
    assert.Equal(t, "b", c.bucket)
}

func TestNewClientWithAPI_CreateBucket(t *testing.T) {
    ctx := context.Background()
    api := &fakeMinio{bucketExists: false}
    c, err := NewClientWithAPI(ctx, api, "bucket")
    require.NoError(t, err)
    assert.Equal(t, "bucket", c.bucket)
}

func TestNewClientWithAPI_BucketExistsError(t *testing.T) {
    ctx := context.Background()
    api := &fakeMinio{bucketExistsErr: errors.New("boom")}
    c, err := NewClientWithAPI(ctx, api, "bucket")
    assert.Nil(t, c)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to ensure bucket exists")
}

func TestNewClientWithAPI_MakeBucketError(t *testing.T) {
    ctx := context.Background()
    api := &fakeMinio{bucketExists: false, makeBucketErr: errors.New("fail")}
    c, err := NewClientWithAPI(ctx, api, "bucket")
    assert.Nil(t, c)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to ensure bucket exists")
}

func TestClient_Upload(t *testing.T) {
    ctx := context.Background()

    t.Run("success", func(t *testing.T) {
        api := &fakeMinio{}
        c := &Client{api: api, bucket: "b"}
        err := c.Upload(ctx, "k", bytes.NewReader([]byte("data")))
        assert.NoError(t, err)
    })

    t.Run("error", func(t *testing.T) {
        api := &fakeMinio{putErr: errors.New("put-fail")}
        c := &Client{api: api, bucket: "b"}
        err := c.Upload(ctx, "k", bytes.NewReader([]byte("data")))
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "failed to upload object")
    })
}

func TestClient_Download(t *testing.T) {
    ctx := context.Background()

    t.Run("success", func(t *testing.T) {
        api := &fakeMinio{getRC: io.NopCloser(bytes.NewReader([]byte("abc")))}
        c := &Client{api: api, bucket: "b"}
        rc, err := c.Download(ctx, "k")
        require.NoError(t, err)
        defer rc.Close()
        buf := make([]byte, 3)
        n, _ := rc.Read(buf)
        assert.Equal(t, 3, n)
        assert.Equal(t, []byte("abc"), buf)
    })

    t.Run("error", func(t *testing.T) {
        api := &fakeMinio{getErr: errors.New("get-fail")}
        c := &Client{api: api, bucket: "b"}
        rc, err := c.Download(ctx, "k")
        assert.Nil(t, rc)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "failed to get object")
    })
}

func TestClient_Delete(t *testing.T) {
    ctx := context.Background()

    t.Run("success", func(t *testing.T) {
        api := &fakeMinio{}
        c := &Client{api: api, bucket: "b"}
        err := c.Delete(ctx, "k")
        assert.NoError(t, err)
    })

    t.Run("error", func(t *testing.T) {
        api := &fakeMinio{removeErr: errors.New("remove-fail")}
        c := &Client{api: api, bucket: "b"}
        err := c.Delete(ctx, "k")
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "failed to delete object")
    })
}

func TestClient_Exists(t *testing.T) {
    ctx := context.Background()

    t.Run("exists", func(t *testing.T) {
        api := &fakeMinio{}
        c := &Client{api: api, bucket: "b"}
        ok, err := c.Exists(ctx, "k")
        assert.NoError(t, err)
        assert.True(t, ok)
    })

    t.Run("not found", func(t *testing.T) {
        api := &fakeMinio{statErr: minioLib.ErrorResponse{Code: "NoSuchKey"}}
        c := &Client{api: api, bucket: "b"}
        ok, err := c.Exists(ctx, "absent")
        assert.NoError(t, err)
        assert.False(t, ok)
    })

    t.Run("other error", func(t *testing.T) {
        api := &fakeMinio{statErr: errors.New("stat-fail")}
        c := &Client{api: api, bucket: "b"}
        ok, err := c.Exists(ctx, "k")
        assert.Error(t, err)
        assert.False(t, ok)
        assert.Contains(t, err.Error(), "failed to stat object")
    })
}
