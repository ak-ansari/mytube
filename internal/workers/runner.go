package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ak-ansari/mytube/internal/cache"
	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/queue"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/pkg/logger"
)

type Runner struct {
	qName           string
	bucketEventName string
	q               queue.Queue
	cache           cache.Cache
	log             logger.Logger
	hr              *HandlerRegistry
	pc              *services.PipelineCoordinator
	vs              *services.VideoService
}

func NewRunner(
	q queue.Queue,
	cache cache.Cache,
	qName string,
	bucketEventName string,
	log logger.Logger,
	hr *HandlerRegistry,
	pc *services.PipelineCoordinator,
	vs *services.VideoService,
) *Runner {
	return &Runner{
		q:               q,
		qName:           qName,
		bucketEventName: bucketEventName,
		log:             log,
		cache:           cache,
		hr:              hr,
		pc:              pc,
		vs:              vs,
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
		return fmt.Errorf("failed to dequeue the job. workerId: %d,queueName:%s, error: %w", workerID, r.qName, err)
	}
	if j == nil {
		return nil
	}

	if err := json.Unmarshal(j, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal job payload workerID:%d, error: %w", workerID, err)
	}
	return r.dispatch(ctx, &payload)
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
		id, shouldProcess, err := r.vs.ParseBucketEvent(ctx, uploaded[key])
		if err != nil {
			r.log.Error(fmt.Sprintf("failed to get id for the key %s", key), logger.Error(err))
			continue
		}
		if shouldProcess {
			fmt.Printf(">>>> task picked from Bucket events key:%s >>> \n\n", key)
			if err := r.pc.Advance(ctx, id); err != nil {
				return err
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
	handler, err := r.hr.GetHandler(payload.Stage)
	if err != nil {
		return err
	}
	if err := handler(ctx, payload); err != nil {
		return err
	}
	return r.pc.Advance(ctx, payload.VideoID)
}
