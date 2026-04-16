package s3

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client     *minio.Client
	bucketName string
}

func New(ctx context.Context, endpoint, accessKey, secretKey, bucket string) (*Storage, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	exists, err := mc.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("failed to create bucket %q: %w", bucket, err)
		}
	}

	return &Storage{client: mc, bucketName: bucket}, nil
}

func (s *Storage) Upload(ctx context.Context, objectKey string, r io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, s.bucketName, objectKey, r, size, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return err
}

func (s *Storage) Download(ctx context.Context, objectKey string) (io.ReadCloser, error) {
    object, err := s.client.GetObject(ctx, s.bucketName, objectKey, minio.GetObjectOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to get object from s3: %w", err)
    }

    return object, nil
}

func (s *Storage) Delete(ctx context.Context, objectKey string) error {
	return s.client.RemoveObject(ctx, s.bucketName, objectKey, minio.RemoveObjectOptions{})
}

func (s *Storage) DeleteObjects(ctx context.Context, keys []string) error {
    if len(keys) == 0 {
        return nil
    }

    objectsCh := make(chan minio.ObjectInfo, len(keys))
    
    for _, key := range keys {
        objectsCh <- minio.ObjectInfo{Key: key}
    }
    close(objectsCh)

    opts := minio.RemoveObjectsOptions{}
    errorCh := s.client.RemoveObjects(ctx, s.bucketName, objectsCh, opts)

    var errs []error
    for err := range errorCh {
        errs = append(errs, fmt.Errorf("failed to delete object %q: %v", err.ObjectName, err.Err))
    }

    if len(errs) > 0 {
        return errors.Join(errs...)
    }

    return nil
}
