package workers

import (
	"context"
	"fmt"

	"github.com/ak-ansari/mytube/internal/jobs"
)

type Handler func(ctx context.Context, p *jobs.JobPayload) error
type HandlerRegistry struct {
	registry map[int]Handler
}

func NewHandlerRegistry(v *Validate, thumb *Thumbnail, trans *Transcode, seg *Segment, pub *Publish, checksum *Checksum) *HandlerRegistry {
	registry := map[int]Handler{
		2: v.Handle,
		3: thumb.Handle,
		4: trans.Handle,
		5: seg.Handle,
		6: checksum.Handle,
		7: pub.Handle,
	}

	return &HandlerRegistry{
		registry: registry,
	}
}
func (hr *HandlerRegistry) GetHandler(stage int) (Handler, error) {
	handler, ok := hr.registry[stage]
	if !ok {
		return nil, fmt.Errorf("no handler is registered for stage %d", stage)
	}
	return handler, nil
}
