package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ak-ansari/mytube/internal/cache"
	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/queue"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/pkg/logger"
	"github.com/google/uuid"
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

	// set by WithRetry; nil means a failure fails the video at once (no retries, no dead-letter log)
	retry *RetryPolicy
	dq    queue.DelayedQueue
	dlq   repository.DeadLetterRepository
	now   func() time.Time
}

// promoteBatch is how many due retries one iteration moves back onto the job queue.
const promoteBatch = 100

// WithRetry turns on retries: a retryable failure is re-enqueued with exponential backoff (on dq, which must
// be the job queue's store) until policy.MaxAttempts runs; a permanent failure, or one out of attempts,
// is written to the dead-letter table and fails the video.
func (r *Runner) WithRetry(dq queue.DelayedQueue, dlq repository.DeadLetterRepository, policy RetryPolicy) *Runner {
	r.retry = &policy
	r.dq = dq
	r.dlq = dlq
	return r
}

func (r *Runner) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
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
	if err := r.PromoteDueRetries(ctx); err != nil {
		r.log.Error("Error while promoting due retries", logger.Error(err))
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
		if r.retry != nil {
			r.deadLetter(ctx, models.DeadLetterSourceJob, "", nil, 1, false, fmt.Errorf("unmarshal job payload: %w", err), string(j))
		}
		return fmt.Errorf("failed to unmarshal job payload workerID:%d, error: %w", workerID, err)
	}
	return r.dispatch(ctx, &payload)
}

// PromoteDueRetries moves retries whose backoff has elapsed back onto the job queue.
func (r *Runner) PromoteDueRetries(ctx context.Context) error {
	if r.retry == nil {
		return nil
	}
	n, err := r.dq.PromoteDue(ctx, r.qName, r.clock(), promoteBatch)
	if err != nil {
		return err
	}
	if n > 0 {
		r.log.Info("Promoted due retries", logger.Int("count", n))
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
		r.processBucketEvent(ctx, key, uploaded[key])
	}
	return nil
}

// bucketEventRetry is the retry state of one bucket event, kept in the cache while the event stays in the hash.
type bucketEventRetry struct {
	Attempts int   `json:"attempts"` // failed runs so far
	NextAt   int64 `json:"next_at"`  // unix ms before which the event is skipped
}

func bucketRetryKey(key string) string { return "bucket_event_retry:" + key }

// processBucketEvent handles one event. Without WithRetry a failure is logged and the event dropped.
// With it, a retryable failure keeps the event in the hash for a later poll (with backoff) until
// MaxAttempts; a permanent one, or one out of attempts, is dead-lettered and dropped.
// The lock is always released.
func (r *Runner) processBucketEvent(ctx context.Context, key, data string) {
	defer r.releaseLock(ctx, key)

	var state bucketEventRetry
	if r.retry != nil {
		// a missing key leaves state zeroed (the Redis cache returns nil, fakes return a miss error)
		_ = r.cache.Get(ctx, bucketRetryKey(key), &state)
		if state.NextAt > r.clock().UnixMilli() {
			return // backing off; leave it in the hash
		}
	}

	id, shouldProcess, err := r.vs.ParseBucketEvent(ctx, data)
	if err != nil {
		err = fmt.Errorf("resolve video for bucket event: %w", err)
	} else if shouldProcess {
		fmt.Printf(">>>> task picked from Bucket events key:%s >>> \n\n", key)
		if aerr := r.pc.Advance(ctx, id); aerr != nil {
			err = fmt.Errorf("advance video from bucket event: %w", aerr)
		}
	}

	if err != nil && r.retry != nil {
		retryable := IsRetryable(err)
		attempts := state.Attempts + 1
		if retryable && attempts < r.retry.MaxAttempts {
			next := bucketEventRetry{Attempts: attempts, NextAt: r.clock().Add(r.retry.Backoff(attempts)).UnixMilli()}
			if serr := r.cache.Set(ctx, bucketRetryKey(key), next, 24*time.Hour); serr == nil {
				r.log.Warn("Bucket event failed, will retry",
					logger.String("key", key),
					logger.String("videoId", id),
					logger.Int("attempt", attempts),
					logger.Error(err))
				return
			}
		}
		r.deadLetter(ctx, models.DeadLetterSourceBucketEvent, id, nil, attempts, retryable, err, data)
	} else if err != nil {
		r.log.Error(fmt.Sprintf("failed to process bucket event key %s, dropping event", key),
			logger.String("videoId", id),
			logger.Error(err))
	}

	if err := r.cache.DeleteFromHash(ctx, r.bucketEventName, key); err != nil {
		r.log.Error(fmt.Sprintf("Failed to delete task %s, from cache", key), logger.Error(err))
	}
	if r.retry != nil {
		_ = r.cache.Delete(ctx, bucketRetryKey(key))
	}
	if err == nil {
		r.log.Success(fmt.Sprintf("task deleted from queue key: %s, id: %s", key, id))
	}
}

// dispatch runs the stage handler and advances the video. Failures go to handleFailure.
func (r *Runner) dispatch(ctx context.Context, payload *jobs.JobPayload) error {
	handler, err := r.hr.GetHandler(payload.Stage)
	if err != nil {
		r.handleFailure(ctx, payload, jobs.Permanent(err))
		return nil
	}
	if err := handler(ctx, payload); err != nil {
		r.handleFailure(ctx, payload, err)
		return nil
	}
	if err := r.pc.Advance(ctx, payload.VideoID); err != nil {
		if r.retry == nil {
			return err
		}
		// the stage's work is done but moving on failed; handlers are idempotent, so rerunning the stage is safe
		r.handleFailure(ctx, payload, fmt.Errorf("advance after stage %d: %w", payload.Stage, err))
	}
	return nil
}

// handleFailure retries a retryable failure with backoff, and otherwise dead-letters the job and fails the video.
func (r *Runner) handleFailure(ctx context.Context, payload *jobs.JobPayload, cause error) {
	if r.retry == nil {
		r.fail(ctx, payload, cause)
		return
	}
	// enqueue/dead-letter/fail must still happen when the handler was interrupted by shutdown
	bg := context.WithoutCancel(ctx)

	// shutdown interrupted the stage: put it back unchanged (not a failed attempt) so the next start resumes it
	if ctx.Err() != nil && errors.Is(cause, context.Canceled) {
		if err := r.enqueueJob(bg, payload, 0); err == nil {
			r.log.Info("Stage interrupted by shutdown, re-queued",
				logger.String("videoId", payload.VideoID),
				logger.Int("stage", payload.Stage))
			return
		}
	}

	retryable := IsRetryable(cause)
	attempts := payload.Attempt + 1
	if retryable && attempts < r.retry.MaxAttempts {
		next := *payload
		next.Attempt = attempts
		delay := r.retry.Backoff(attempts)
		err := r.enqueueJob(bg, &next, delay)
		if err == nil {
			r.log.Warn("Stage failed, will retry",
				logger.String("videoId", payload.VideoID),
				logger.Int("stage", payload.Stage),
				logger.Int("attempt", attempts),
				logger.Int("maxAttempts", r.retry.MaxAttempts),
				logger.String("retryIn", delay.Round(time.Second).String()),
				logger.Error(cause))
			return
		}
		r.log.Error("Failed to schedule retry",
			logger.String("videoId", payload.VideoID),
			logger.Int("stage", payload.Stage),
			logger.Error(err))
	}

	raw, _ := json.Marshal(payload)
	stage := payload.Stage
	r.deadLetter(bg, models.DeadLetterSourceJob, payload.VideoID, &stage, attempts, retryable, cause, string(raw))
	r.fail(bg, payload, cause)
}

func (r *Runner) enqueueJob(ctx context.Context, payload *jobs.JobPayload, delay time.Duration) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return r.dq.EnqueueAfter(ctx, r.qName, raw, delay)
}

// deadLetter records work that failed for good; a failure to record it is only logged.
func (r *Runner) deadLetter(ctx context.Context, source models.DeadLetterSource, videoID string, stage *int, attempts int, retryable bool, cause error, payload string) {
	reason := cause.Error()
	if retryable {
		reason = fmt.Sprintf("retries exhausted after %d attempts: %s", attempts, reason)
	}
	j := &models.DeadLetterJob{Source: source, Stage: stage, Attempts: attempts, Retryable: retryable, Error: reason, Payload: payload}
	if id, err := uuid.Parse(videoID); err == nil {
		j.VideoID = &id
	}
	r.log.Error("Dead-lettering pipeline work",
		logger.String("source", string(source)),
		logger.String("videoId", videoID),
		logger.Int("attempts", attempts),
		logger.Bool("retryable", retryable),
		logger.Error(cause))
	if err := r.dlq.Insert(context.WithoutCancel(ctx), j); err != nil {
		r.log.Error("Failed to write dead letter",
			logger.String("videoId", videoID),
			logger.Error(err))
	}
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
