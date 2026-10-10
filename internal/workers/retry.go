package workers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/minio/minio-go/v7"
)

// RetryPolicy bounds how often and how fast a failing stage is retried.
type RetryPolicy struct {
	MaxAttempts int           // total runs of a stage, including the first
	BaseDelay   time.Duration // delay before the first retry; doubles per retry
	MaxDelay    time.Duration // cap on a single delay
}

// DefaultRetryPolicy is used for any WORKER setting that is missing from the config.
var DefaultRetryPolicy = RetryPolicy{MaxAttempts: 4, BaseDelay: 10 * time.Second, MaxDelay: 5 * time.Minute}

// RetryPolicyFromConfig fills unset or invalid WORKER values with DefaultRetryPolicy.
func RetryPolicyFromConfig(c config.Worker) RetryPolicy {
	p := DefaultRetryPolicy
	if c.MaxAttempts > 0 {
		p.MaxAttempts = c.MaxAttempts
	}
	if d, err := time.ParseDuration(c.RetryBaseDelay); err == nil && d > 0 {
		p.BaseDelay = d
	}
	if d, err := time.ParseDuration(c.RetryMaxDelay); err == nil && d > 0 {
		p.MaxDelay = d
	}
	if p.MaxDelay < p.BaseDelay {
		p.MaxDelay = p.BaseDelay
	}
	return p
}

// Backoff is the delay before retry number `retry` (1 for the first retry): BaseDelay·2^(retry-1),
// capped at MaxDelay, plus up to 20% jitter so retries of many videos don't line up.
func (p RetryPolicy) Backoff(retry int) time.Duration {
	if retry < 1 {
		retry = 1
	}
	d := p.BaseDelay
	for i := 1; i < retry && d < p.MaxDelay; i++ {
		d *= 2
	}
	if d > p.MaxDelay {
		d = p.MaxDelay
	}
	return d + time.Duration(rand.Int64N(int64(d)/5+1))
}

// ffmpeg/ffprobe exit codes are input errors, unless the input was a URL that failed to load.
var transientFFmpegOutput = []string{
	"connection refused", "connection reset", "connection timed out", "timed out",
	"server returned 5", "server returned 429", "i/o error", "network is unreachable",
}

// IsRetryable reports whether running the stage again may succeed.
// Unknown errors are retried: MaxAttempts bounds the cost of guessing wrong.
func IsRetryable(err error) bool {
	if err == nil || jobs.IsPermanent(err) {
		return false
	}
	// the video row (or metadata) is gone, so the video was deleted mid-pipeline
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, services.ErrNotFound) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 08 connection, 40 serialization/deadlock, 53 out of resources, 57P operator intervention (e.g. restart)
		c := pgErr.Code
		return strings.HasPrefix(c, "08") || strings.HasPrefix(c, "40") || strings.HasPrefix(c, "53") || strings.HasPrefix(c, "57P")
	}
	if resp := minio.ToErrorResponse(err); resp.StatusCode != 0 {
		return resp.StatusCode >= 500 || resp.StatusCode == 429
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		msg := strings.ToLower(err.Error())
		for _, s := range transientFFmpegOutput {
			if strings.Contains(msg, s) {
				return true
			}
		}
		return false
	}
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		return false
	}
	return true
}
