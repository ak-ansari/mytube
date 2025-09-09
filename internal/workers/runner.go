package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/queue"
)

type Runner struct {
	qName     string
	q         queue.Queue
	validate  *Validate
	transcode *Transcode
	segment   *Segment
	checksum  *Checksum
	publish   *Publish
	thumbnail *Thumbnail
}

func NewRunner(q queue.Queue,
	qName string,
	validate *Validate,
	transcode *Transcode,
	segment *Segment,
	checksum *Checksum,
	publish *Publish,
	thumbnail *Thumbnail) *Runner {
	return &Runner{
		q:         q,
		qName:     qName,
		validate:  validate,
		transcode: transcode,
		segment:   segment,
		checksum:  checksum,
		publish:   publish,
		thumbnail: thumbnail,
	}
}

func (r *Runner) Start(ctx context.Context) {
	for i := 0; i < 2; i++ {
		go func(id int) error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				j, err := r.q.Dequeue(ctx, r.qName)
				if err != nil {
					continue
				}
				if j == nil {
					continue
				}

				var payload jobs.JobPayload
				if err := json.Unmarshal(j, &payload); err != nil {
					fmt.Println("Error while unmarshaling the job")
					continue
				}
				if err := r.dispatch(ctx, payload); err != nil {
					fmt.Printf("error in step %s for video %s \n\n\n", payload.Step, payload.VideoID)
					fmt.Print(err)
				}

			}
		}(i)
	}
}
func (r *Runner) dispatch(ctx context.Context, payload jobs.JobPayload) error {
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

func (r *Runner) getHandler(step jobs.Step) (func(ctx context.Context, p jobs.JobPayload) error, jobs.Step) {
	switch step {
	case jobs.StepValidate:
		return r.validate.Handle, jobs.StepTranscode
	case jobs.StepTranscode:
		return r.transcode.Handle, jobs.StepSegment
	case jobs.StepSegment:
		return r.segment.Handle, jobs.StepChecksum
	case jobs.StepChecksum:
		return r.checksum.Handle, jobs.StepThumbs
	case jobs.StepThumbs:
		return r.thumbnail.Handle, jobs.StepPublish
	case jobs.StepPublish:
		return r.publish.Handle, ""
	}
	return nil, ""
}

func (r *Runner) enqueueNext(ctx context.Context, id string, step jobs.Step) error {
	p, err := json.Marshal(jobs.JobPayload{VideoID: id, Step: step})
	if err != nil {
		return err
	}
	return r.q.Enqueue(ctx, r.qName, p)

}
