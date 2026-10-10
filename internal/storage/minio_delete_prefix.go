package storage

import (
	"context"

	"github.com/ak-ansari/mytube/pkg/logger"
	"github.com/minio/minio-go/v7"
)

// DeletePrefix removes every object whose key starts with prefix.
func (s3 *S3Store) DeletePrefix(ctx context.Context, prefix string) error {
	objects := s3.client.ListObjects(ctx, s3.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	for res := range s3.client.RemoveObjects(ctx, s3.bucket, objects, minio.RemoveObjectsOptions{}) {
		if res.Err != nil {
			s3.log.Error("Failed to delete object", logger.String("key", res.ObjectName), logger.Error(res.Err))
			return res.Err
		}
	}
	s3.log.Success("Objects deleted", logger.String("prefix", prefix))
	return nil
}
