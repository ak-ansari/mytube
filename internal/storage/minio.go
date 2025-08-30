package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Store struct {
	client     *minio.Client
	bucket     string
	publicBase string
}

func NewS3Store() (*S3Store, error) {
	conf, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	s3Conf := conf.S3
	endpoint := fmt.Sprintf("%s:%s", s3Conf.MinioHost, s3Conf.MinioPort)
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(s3Conf.MinioAccessKey, s3Conf.MinioSecretKey, ""), Secure: false, Region: "us-est-1"})
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, s3Conf.MinioBucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		fmt.Printf("Bucket: %s does not exists creating new.... \n", s3Conf.MinioBucket)
		if err := client.MakeBucket(ctx, s3Conf.MinioBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
		fmt.Printf("Bucket: %s Created Successfully \n", s3Conf.MinioBucket)
	}
	fmt.Printf("Connected To Bucket: %s \n", s3Conf.MinioBucket)
	s3 := &S3Store{
		client:     client,
		bucket:     s3Conf.MinioBucket,
		publicBase: fmt.Sprintf("http://%s/%s", endpoint, s3Conf.MinioBucket),
	}
	return s3, nil
}

func (s3 *S3Store) Put(ctx context.Context, key string, file io.Reader, size int64) (string, error) {
	res, err := s3.client.PutObject(ctx, s3.bucket, key, file, size, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}
	return res.Key, nil
}
func (s3 *S3Store) Get(ctx context.Context, key string) (io.Reader, int64, error) {
	obj, err := s3.client.GetObject(ctx, s3.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, err
	}
	st, err := obj.Stat()
	if err != nil {
		return nil, 0, err
	}
	return obj, st.Size, nil
}
func (s3 *S3Store) Delete(ctx context.Context, key string) error {
	return s3.client.RemoveObject(ctx, s3.bucket, key, minio.RemoveObjectOptions{})
}
func (s3 *S3Store) GetUrl(ctx context.Context, key string) (string, error) {
	url, err := s3.client.PresignedGetObject(ctx, s3.bucket, key, 12*time.Hour, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}
func (s3 *S3Store) SaveLocally(ctx context.Context, key string, path string) error {
	return s3.client.FGetObject(ctx, s3.bucket, key, path, minio.GetObjectOptions{})
}
func (s3 *S3Store) UploadLocalFile(ctx context.Context, key string, path string, contentType string) (string, error) {
	options := minio.PutObjectOptions{}
	if contentType != "" {
		options.ContentType = contentType
	}
	i, err := s3.client.FPutObject(ctx, s3.bucket, key, path, options)
	if err != nil {
		return "", err
	}
	return i.Key, nil
}
