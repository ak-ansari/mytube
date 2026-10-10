package workers

import (
	"context"
	"strconv"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/observability"
)

// stageNames labels stage handler metrics (keys match NewHandlerRegistry).
var stageNames = map[int]string{2: "validate", 3: "thumbnail", 4: "transcode", 5: "segment", 6: "checksum", 7: "publish"}

// Instrument wraps every registered handler so each run is timed and counted in Prometheus.
func (hr *HandlerRegistry) Instrument() *HandlerRegistry {
	for stage, h := range hr.registry {
		name, ok := stageNames[stage]
		if !ok {
			name = strconv.Itoa(stage)
		}
		hr.registry[stage] = func(ctx context.Context, p *jobs.JobPayload) error {
			return observability.ObserveStage(name, func() error { return h(ctx, p) })
		}
	}
	return hr
}
