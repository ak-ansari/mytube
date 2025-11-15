package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ak-ansari/mytube/internal/cache"
	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
	"github.com/ak-ansari/mytube/internal/queue"
)

type Runner struct {
	qName                string
	bucketEventName      string
	q                    queue.Queue
	cache                cache.Cache
	validate             *Validate
	transcode            *Transcode
	segment              *Segment
	checksum             *Checksum
	publish              *Publish
	thumbnail            *Thumbnail
	log                  logger.Logger
	bucketEventProcessor *bucketEventProcessor
}

func NewRunner(
	q queue.Queue,
	cache cache.Cache,
	qName string,
	bucketEventName string,
	bucketEventProcessor *bucketEventProcessor,
	validate *Validate,
	transcode *Transcode,
	segment *Segment,
	checksum *Checksum,
	publish *Publish,
	thumbnail *Thumbnail,
	log logger.Logger,
) *Runner {
	return &Runner{
		q:                    q,
		qName:                qName,
		bucketEventName:      bucketEventName,
		validate:             validate,
		transcode:            transcode,
		segment:              segment,
		checksum:             checksum,
		publish:              publish,
		thumbnail:            thumbnail,
		log:                  log,
		cache:                cache,
		bucketEventProcessor: bucketEventProcessor,
	}
}

func (r *Runner) Start(ctx context.Context) {
	for i := 0; i < 2; i++ {
		go func(workerID int) error {
			for {
				select {
				case <-ctx.Done():
					r.log.Info("Worker stopped",
						logger.Int("workerID", workerID))
					return ctx.Err()
				default:
				}

				err := r.ProcessBucketEvents(ctx)
				if err != nil {
					r.log.Error("Error while processing bucket events", logger.Error(err))
				}
				err = r.ProcessQueueTasks(ctx, workerID)
				if err != nil {
					r.log.Error("Error while processing task queue", logger.Error(err))
				}
			}
		}(i)
	}
}
func (r *Runner) ProcessQueueTasks(ctx context.Context, workerID int) error {
	var payload jobs.JobPayload
	j, err := r.q.Dequeue(ctx, r.qName)
	if err != nil {
		r.log.Error("Failed to dequeue job",
			logger.Int("workerID", workerID),
			logger.Error(err))
		return err
	}
	if j == nil {
		return nil
	}
	fmt.Println(">>>> task picked")

	if err := json.Unmarshal(j, &payload); err != nil {
		r.log.Error("Failed to unmarshal job payload",
			logger.Int("workerID", workerID),
			logger.Error(err))
		return err
	}
	if err := r.dispatch(ctx, &payload); err != nil {
		r.log.Error("Job handler failed",
			logger.String("step", string(payload.Step)),
			logger.String("videoId", payload.VideoID),
			logger.Error(err))
		return err
	}
	return nil
}

func (r *Runner) acquireLock(ctx context.Context, key string) bool {
	lockKey := fmt.Sprintf("bucket_event_lock:%s", key)

	ok, _ := r.cache.SetNX(ctx, lockKey, "1", 5*time.Minute)
	return ok
}

func (r *Runner) releaseLock(ctx context.Context, key string) {
	lockKey := fmt.Sprintf("bucket_event_lock:%s", key)
	r.cache.Delete(ctx, lockKey)
}

func (r *Runner) ProcessBucketEvents(ctx context.Context) error {
	uploaded, err := r.cache.GetAllFromHash(ctx, r.bucketEventName)
	if err != nil {
		r.log.Error(fmt.Sprintf("Failed to read events from bucket event hash %s", err.Error()))
		return err
	}
	for key := range uploaded {
		if !r.acquireLock(ctx, key) {
			continue // another worker is already processing this key
		}
		id, shouldProcess, err := r.bucketEventProcessor.Process(ctx, uploaded[key])
		if err != nil {
			r.log.Error(fmt.Sprintf("failed to get id for the key %s", key), logger.Error(err))
			continue
		}
		if shouldProcess {
			fmt.Printf(">>>> task picked from Bucket events key:%s >>> \n\n", key)
			step := jobs.StepValidate
			if err := r.enqueueNext(ctx, id, step); err != nil {
				r.log.Error(fmt.Sprintf("Failed to enqueue task step %s,for video id %s", step, id))
				continue
			}
		}
		if err := r.cache.DeleteFromHash(ctx, r.bucketEventName, key); err != nil {
			r.log.Error(fmt.Sprintf("Failed to delete task %s, from cache", key))
		}
		r.log.Success(fmt.Sprintf("task deleted from queue key: %s, id: %s", key, id))
		r.releaseLock(ctx, key)
	}
	return nil
}

func (r *Runner) dispatch(ctx context.Context, payload *jobs.JobPayload) error {
	handler, nextStep := r.getHandler(payload.Step)
	if handler == nil {
		return fmt.Errorf("no handler for step %s", payload.Step)
	}
	if err := handler(ctx, payload); err != nil {
		return err
	}
	if nextStep != "" {
		return r.enqueueNext(ctx, payload.VideoID, nextStep)
	}
	return nil
}

func (r *Runner) getHandler(step jobs.Step) (func(ctx context.Context, p *jobs.JobPayload) error, jobs.Step) {
	switch step {
	//phase 1 in draft state
	case jobs.StepValidate:
		return r.validate.Handle, jobs.StepThumbs
	case jobs.StepThumbs:
		return r.thumbnail.Handle, "" // background process end for phase 1

		// phase 2 submit state
	case jobs.StepTranscode:
		return r.transcode.Handle, jobs.StepSegment
	case jobs.StepSegment:
		return r.segment.Handle, jobs.StepChecksum
	case jobs.StepChecksum:
		return r.checksum.Handle, jobs.StepPublish
	case jobs.StepPublish:
		return r.publish.Handle, "" // background processing finished successfully
	}
	return nil, ""
}

func (r *Runner) enqueueNext(ctx context.Context, id string, step jobs.Step) error {
	p, err := json.Marshal(jobs.JobPayload{VideoID: id, Step: step})
	if err != nil {
		return err
	}
	r.log.Info("Enqueuing next step",
		logger.String("videoId", id),
		logger.String("step", string(step)))
	return r.q.Enqueue(ctx, r.qName, p)
}
