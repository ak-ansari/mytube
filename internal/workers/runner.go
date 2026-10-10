package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ak-ansari/mytube/internal/cache"
	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/queue"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/pkg/logger"
)

// errorPause is how long a worker waits after an infrastructure error (e.g. Redis down) to avoid a hot loop.
const errorPause = time.Second

type Runner struct {
	qName           string
	bucketEventName string
	q               queue.Queue
	cache           cache.Cache
	log             logger.Logger
	hr              *HandlerRegistry
	pc              *services.PipelineCoordinator
	vs              *services.VideoService
	wg              sync.WaitGroup
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

// Start launches the worker goroutines; Wait blocks until they return after ctx is cancelled.
func (r *Runner) Start(ctx context.Context) {
	for i := 0; i < 2; i++ {
		r.wg.Add(1)
		go func(workerID int) {
			defer r.wg.Done()
			for {
				select {
				case <-ctx.Done():
					r.log.Info("Worker stopped",
						logger.Int("workerID", workerID))
					return
				default:
				}

				if failed := r.iterate(ctx, workerID); failed {
					select {
					case <-ctx.Done():
					case <-time.After(errorPause):
					}
				}
			}
		}(i)
	}
}

// Wait blocks until every worker goroutine has finished its current iteration and stopped.
func (r *Runner) Wait() {
	r.wg.Wait()
}

// iterate runs one poll of bucket events and the job queue. It reports whether an error occurred;
// a panic is recovered and logged so one bad message can't kill the worker.
func (r *Runner) iterate(ctx context.Context, workerID int) (failed bool) {
	defer func() {
		if rec := recover(); rec != nil {
			r.log.Error("Worker recovered from panic",
				logger.Int("workerID", workerID),
				logger.Any("panic", rec))
			failed = true
		}
	}()

	if err := r.ProcessBucketEvents(ctx); err != nil {
		r.log.Error("Error while processing bucket events", logger.Error(err))
		failed = true
	}
	if err := r.ProcessQueueTasks(ctx, workerID); err != nil {
		r.log.Error("Error while processing task queue", logger.Error(err))
		failed = true
	}
	return failed
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
		r.processBucketEvent(ctx, key, uploaded[key])
	}
	return nil
}

// processBucketEvent handles one event: failures are logged and the event is dropped (no retry).
// The event is always removed from the hash and the lock always released.
func (r *Runner) processBucketEvent(ctx context.Context, key, data string) {
	defer r.releaseLock(ctx, key)
	defer func() {
		if err := r.cache.DeleteFromHash(ctx, r.bucketEventName, key); err != nil {
			r.log.Error(fmt.Sprintf("Failed to delete task %s, from cache", key), logger.Error(err))
		}
	}()

	id, shouldProcess, err := r.vs.ParseBucketEvent(ctx, data)
	if err != nil {
		r.log.Error(fmt.Sprintf("failed to get id for the key %s, dropping event", key), logger.Error(err))
		return
	}
	if shouldProcess {
		fmt.Printf(">>>> task picked from Bucket events key:%s >>> \n\n", key)
		if err := r.pc.Advance(ctx, id); err != nil {
			r.log.Error(fmt.Sprintf("failed to advance video from bucket event key %s, dropping event", key),
				logger.String("videoId", id),
				logger.Error(err))
			return
		}
	}
	r.log.Success(fmt.Sprintf("task deleted from queue key: %s, id: %s", key, id))
}

// dispatch runs the stage handler. A failing handler marks the video failed; nothing is retried.
func (r *Runner) dispatch(ctx context.Context, payload *jobs.JobPayload) error {
	handler, err := r.hr.GetHandler(payload.Stage)
	if err != nil {
		r.fail(ctx, payload, err)
		return nil
	}
	if err := handler(ctx, payload); err != nil {
		r.fail(ctx, payload, err)
		return nil
	}
	return r.pc.Advance(ctx, payload.VideoID)
}

func (r *Runner) fail(ctx context.Context, payload *jobs.JobPayload, cause error) {
	r.log.Error("Stage failed, marking video as failed",
		logger.String("videoId", payload.VideoID),
		logger.Int("stage", payload.Stage),
		logger.Error(cause))
	// still record the failure when the handler was interrupted by shutdown
	if err := r.pc.Fail(context.WithoutCancel(ctx), payload.VideoID, cause.Error()); err != nil {
		r.log.Error("Failed to mark video as failed",
			logger.String("videoId", payload.VideoID),
			logger.Int("stage", payload.Stage),
			logger.Error(err))
	}
}
