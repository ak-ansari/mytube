package workers

import (
	"context"
	"strings"

	"github.com/ak-ansari/mytube/internal/pkg/logger"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
)

type bucketEventProcessor struct {
	service *services.VideoService
	log     logger.Logger
	store   storage.ObjectStore
}

func NewBucketEventProcessor(s *services.VideoService, log logger.Logger, store storage.ObjectStore) *bucketEventProcessor {
	return &bucketEventProcessor{service: s, log: log, store: store}
}
func (bp *bucketEventProcessor) Process(ctx context.Context, data string) (string, bool, error) {
	bucketEvent, err := bp.store.ParseBucketEvent(data)
	if err != nil {
		return "", false, err
	}
	// process only if the file is uploaded in originals directory and event is created
	if !strings.HasPrefix(bucketEvent.ObjectKey, storage.DirectoryOriginals) || bucketEvent.Event != storage.BucketEventCreated {
		return "", false, nil
	}
	id, err := bp.service.GetVideoByKey(ctx, bucketEvent.ObjectKey)
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}
