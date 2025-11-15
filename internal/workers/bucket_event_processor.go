package workers

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/ak-ansari/mytube/internal/pkg/logger"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
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
type bucketEventProcessor struct {
	service *services.VideoService
	log     logger.Logger
}

func NewBucketEventProcessor(s *services.VideoService, log logger.Logger) *bucketEventProcessor {
	return &bucketEventProcessor{service: s, log: log}
}
func (bp *bucketEventProcessor) getParsed(ctx context.Context, rawData string) (*EventRecord, error) {
	var payload EventRecord
	if err := json.Unmarshal([]byte(rawData), &payload); err != nil {
		bp.log.Error("Failed to unmarshal event payload", logger.Error(err))
		return nil, err
	}
	return &payload, nil
}
func (bp *bucketEventProcessor) Process(ctx context.Context, data string) (string, bool, error) {
	result, err := bp.getParsed(ctx, data)
	if err != nil {
		return "", false, err
	}
	key, err := url.QueryUnescape(result.Records[0].S3.Object.Key)
	if !strings.HasPrefix(key, storage.DirectoryOriginals) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	id, err := bp.service.GetVideoByKey(ctx, key)
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}
