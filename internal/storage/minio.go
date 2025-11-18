package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type EventRecord struct {
	Records []struct {
		EventVersion string `json:"eventVersion"`
		EventSource  string `json:"eventSource"`
		AWSRegion    string `json:"awsRegion"`
		EventTime    string `json:"eventTime"`
		EventName    string `json:"eventName"`
		UserIdentity struct {
			PrincipalID string `json:"principalId"`
		} `json:"userIdentity"`
		RequestParameters struct {
			PrincipalID string `json:"principalId"`
			Region      string `json:"region"`
			SourceIP    string `json:"sourceIPAddress"`
		} `json:"requestParameters"`
		ResponseElements struct {
			XAmzID2              string `json:"x-amz-id-2"`
			XAmzRequestID        string `json:"x-amz-request-id"`
			XMinioDeploymentID   string `json:"x-minio-deployment-id"`
			XMinioOriginEndpoint string `json:"x-minio-origin-endpoint"`
		} `json:"responseElements"`
		S3 struct {
			S3SchemaVersion string `json:"s3SchemaVersion"`
			ConfigurationID string `json:"configurationId"`
			Bucket          struct {
				Name          string `json:"name"`
				OwnerIdentity struct {
					PrincipalID string `json:"principalId"`
				} `json:"ownerIdentity"`
				ARN string `json:"arn"`
			} `json:"bucket"`
			Object struct {
				Key          string `json:"key"`
				Size         int64  `json:"size"`
				ETag         string `json:"eTag"`
				ContentType  string `json:"contentType"`
				UserMetadata struct {
					ContentType string `json:"content-type"`
				} `json:"userMetadata"`
				Sequencer string `json:"sequencer"`
			} `json:"object"`
		} `json:"s3"`
		Source struct {
			Host      string `json:"host"`
			Port      string `json:"port"`
			UserAgent string `json:"userAgent"`
		} `json:"source"`
	} `json:"Records"`
}

type S3Store struct {
	client *minio.Client
	bucket string
	log    logger.Logger // use your interface, not *zap.Logger directly
}

func NewS3Store(log logger.Logger) (*S3Store, error) {
	conf, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	s3Conf := conf.S3
	log.Info("minio endpoint ", logger.String("endpoint", s3Conf.MinioEndpoint))
	client, err := minio.New(s3Conf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s3Conf.MinioAccessKey, s3Conf.MinioSecretKey, ""),
		Secure: false,
		Region: "us-est-1",
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, s3Conf.MinioBucket)
	if err != nil {
		return nil, err
	}

	if !exists {
		log.Info("Bucket does not exist, creating new one...",
			logger.String("bucket", s3Conf.MinioBucket))

		if err := client.MakeBucket(ctx, s3Conf.MinioBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}

		log.Success("Bucket created successfully",
			logger.String("bucket", s3Conf.MinioBucket))
	}

	log.Info("Connected to bucket",
		logger.String("bucket", s3Conf.MinioBucket))

	s3 := &S3Store{
		client: client,
		bucket: s3Conf.MinioBucket,
		log:    log,
	}
	return s3, nil
}

func (s3 *S3Store) Put(ctx context.Context, fileId string, key string, file io.Reader, size int64) (string, error) {
	metadata := make(map[string]string)
	metadata["fileId"] = fileId
	res, err := s3.client.PutObject(ctx, s3.bucket, key, file, size, minio.PutObjectOptions{
		UserMetadata: metadata,
	})
	if err != nil {
		s3.log.Error("Failed to put object",
			logger.String("key", key),
			logger.Error(err))
		return "", err
	}
	s3.log.Success("File uploaded successfully",
		logger.String("key", res.Key),
		logger.Int64("size", res.Size))
	return res.Key, nil
}
func (s3 *S3Store) GerPreSignedPutUrl(ctx context.Context, key string, size int64) (string, error) {
	res, err := s3.client.PresignedPutObject(ctx, s3.bucket, key, 60*time.Minute)
	if err != nil {
		s3.log.Error("Failed to sign put object",
			logger.String("key", key),
			logger.Error(err))
		return "", err
	}
	s3.log.Success("File upload url signed successfully",
		logger.String("key", res.RequestURI()),
		logger.String("size", res.Path))
	return res.String(), nil
}
func (s3 *S3Store) Get(ctx context.Context, key string) (io.Reader, int64, error) {
	obj, err := s3.client.GetObject(ctx, s3.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		s3.log.Error("Failed to get object",
			logger.String("key", key),
			logger.Error(err))
		return nil, 0, err
	}

	st, err := obj.Stat()
	if err != nil {
		s3.log.Error("Failed to stat object",
			logger.String("key", key),
			logger.Error(err))
		return nil, 0, err
	}

	s3.log.Success("Object retrieved successfully",
		logger.String("key", key),
		logger.Int64("size", st.Size))
	return obj, st.Size, nil
}

func (s3 *S3Store) Delete(ctx context.Context, key string) error {
	err := s3.client.RemoveObject(ctx, s3.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		s3.log.Error("Failed to delete object",
			logger.String("key", key),
			logger.Error(err))
		return err
	}
	s3.log.Success("Object deleted successfully",
		logger.String("key", key))
	return nil
}

func (s3 *S3Store) GetUrl(ctx context.Context, key string) (string, error) {
	url, err := s3.client.PresignedGetObject(ctx, s3.bucket, key, 12*time.Hour, nil)
	if err != nil {
		s3.log.Error("Failed to generate presigned URL",
			logger.String("key", key),
			logger.Error(err))
		return "", err
	}
	return url.String(), nil
}

func (s3 *S3Store) SaveLocally(ctx context.Context, key string, path string) error {
	err := s3.client.FGetObject(ctx, s3.bucket, key, path, minio.GetObjectOptions{})
	if err != nil {
		s3.log.Error("Failed to save object locally",
			logger.String("key", key),
			logger.String("path", path),
			logger.Error(err))
		return err
	}
	s3.log.Success("Object saved locally",
		logger.String("key", key),
		logger.String("path", path))
	return nil
}

func (s3 *S3Store) UploadLocalFile(ctx context.Context, key string, path string, contentType string) (string, error) {
	options := minio.PutObjectOptions{}
	if contentType != "" {
		options.ContentType = contentType
	}

	i, err := s3.client.FPutObject(ctx, s3.bucket, key, path, options)
	if err != nil {
		s3.log.Error("Failed to upload local file",
			logger.String("key", key),
			logger.String("path", path),
			logger.Error(err))
		return "", err
	}
	s3.log.Success("Local file uploaded successfully",
		logger.String("key", i.Key),
		logger.Int64("size", i.Size))
	return i.Key, nil
}
func (s3 *S3Store) ParseBucketEvent(rawData string) (*BucketEvent, error) {
	var payload EventRecord
	if err := json.Unmarshal([]byte(rawData), &payload); err != nil {
		s3.log.Error("Failed to unmarshal event payload", logger.Error(err))
		return nil, err
	}
	key, err := url.QueryUnescape(payload.Records[0].S3.Object.Key)
	if err != nil {
		return nil, err
	}
	event := s3.GetEventName(payload.Records[0].EventName)
	return &BucketEvent{
		Event:       event,
		ObjectKey:   key,
		Etag:        payload.Records[0].S3.Object.ETag,
		Size:        payload.Records[0].S3.Object.Size,
		ContentType: payload.Records[0].S3.Object.ContentType,
		Bucket:      payload.Records[0].S3.Bucket.Name,
	}, nil
}
func (s3 *S3Store) GetEventName(minioEvent string) BucketEventType {
	if strings.HasPrefix(minioEvent, "s3:ObjectCreated") {
		return BucketEventCreated
	} else if strings.HasPrefix(minioEvent, "s3:ObjectRemoved") {
		return BucketEventDeleted
	}
	return ""
}
