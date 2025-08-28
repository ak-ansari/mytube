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
	for i := range 2 {
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
					fmt.Print(err)
				}

			}
		}(i)
	}
}
func (r *Runner) dispatch(ctx context.Context, payload jobs.JobPayload) error {
	var err error
	switch payload.Step {
	case jobs.StepValidate:
		err = r.validate.Handle(ctx, payload)
	case jobs.StepTranscode:
		err = r.transcode.Handle(ctx, payload)
	case jobs.StepSegment:
		err = r.segment.Handle(ctx, payload)
	case jobs.StepChecksum:
		err = r.checksum.Handle(ctx, payload)
	case jobs.StepThumbs:
		err = r.thumbnail.Handle(ctx, payload)
	case jobs.StepPublish:
		err = r.publish.Handle(ctx, payload)
	}
	return err
}
