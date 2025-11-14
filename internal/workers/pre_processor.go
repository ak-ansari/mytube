package workers

import (
	"context"
	"encoding/json"

	"github.com/ak-ansari/mytube/internal/pkg/logger"
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

type preProcessor struct {
	log logger.Logger
}

func NewPreProcessor(log logger.Logger) *preProcessor {
	return &preProcessor{
		log: log,
	}
}

func (p *preProcessor) Process(ctx context.Context, events map[string]string) {
	for key := range events {
		data := events[key]
		var payload EventRecord
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			p.log.Error("Failed to unmarshal event payload",
				logger.Error(err))
			continue
		}
		go p.processData(&payload)
	}
}

func (p *preProcessor) processData(data *EventRecord) {

}
