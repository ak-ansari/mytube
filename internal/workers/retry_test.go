package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/minio/minio-go/v7"
)

// runExit runs a shell snippet that must fail and returns its *exec.ExitError, plus the error
// wrapped the way internal/media wraps ffmpeg failures (stderr appended to the message).
func runExit(t *testing.T, script string) (bare error, wrapped error) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	cmd := exec.Command("sh", "-c", script)
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected an *exec.ExitError, got %T %v", err, err)
	}
	return err, fmt.Errorf("ffmpeg transcode failed: %w\nstderr: %s", err, out)
}

func TestIsRetryable(t *testing.T) {
	falseBare, _ := runExit(t, "false")
	netBare, netWrapped := runExit(t, "echo connection refused >&2; exit 1")
	_, http5xx := runExit(t, "echo 'Server returned 503 Service Unavailable' >&2; exit 1")
	_, http429 := runExit(t, "echo 'Server returned 429 Too Many Requests' >&2; exit 1")
	_, http404 := runExit(t, "echo 'Server returned 404 Not Found' >&2; exit 1")
	_, badInput := runExit(t, "echo 'Invalid data found when processing input' >&2; exit 1")
	_, upperTimeout := runExit(t, "echo 'Connection TIMED OUT' >&2; exit 1")

	var syntaxErr error = json.Unmarshal([]byte("{nope"), &struct{}{})
	var typeErr error = json.Unmarshal([]byte(`{"stage":"x"}`), &jobs.JobPayload{})
	if _, ok := syntaxErr.(*json.SyntaxError); !ok {
		t.Fatalf("syntaxErr is %T", syntaxErr)
	}
	if _, ok := typeErr.(*json.UnmarshalTypeError); !ok {
		t.Fatalf("typeErr is %T", typeErr)
	}

	pg := func(code string) error { return &pgconn.PgError{Code: code, Message: "pg " + code} }
	s3 := func(status int) error { return minio.ErrorResponse{StatusCode: status, Code: "X", Message: "s3"} }
	wrap := func(err error) error { return fmt.Errorf("stage: %w", fmt.Errorf("inner: %w", err)) }
	dnsTimeout := &net.DNSError{Err: "i/o timeout", Name: "minio", IsTimeout: true}
	dialErr := &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		// explicit permanent marks win over everything
		{"permanent", jobs.Permanent(errors.New("bad input")), false},
		{"wrapped permanent", wrap(jobs.Permanent(errors.New("bad input"))), false},
		{"permanent around a retryable cause", jobs.Permanent(context.DeadlineExceeded), false},
		{"permanent around a network error", wrap(jobs.Permanent(dialErr)), false},
		// the video is gone
		{"pgx no rows", pgx.ErrNoRows, false},
		{"wrapped pgx no rows", wrap(pgx.ErrNoRows), false},
		{"not found", services.ErrNotFound, false},
		{"wrapped not found", wrap(services.ErrNotFound), false},
		// context / io / syscall
		{"deadline", context.DeadlineExceeded, true},
		{"wrapped deadline", wrap(context.DeadlineExceeded), true},
		{"canceled", context.Canceled, true},
		{"unexpected EOF", wrap(io.ErrUnexpectedEOF), true},
		{"ECONNREFUSED", syscall.ECONNREFUSED, true},
		{"wrapped ECONNRESET", wrap(os.NewSyscallError("read", syscall.ECONNRESET)), true},
		// net.Error
		{"dns timeout", dnsTimeout, true},
		{"dial error", wrap(dialErr), true},
		{"plain net.OpError", &net.OpError{Op: "read", Err: errors.New("weird")}, true},
		// postgres classes
		{"pg 08006 connection failure", pg("08006"), true},
		{"pg 08001 wrapped", wrap(pg("08001")), true},
		{"pg 40001 serialization", pg("40001"), true},
		{"pg 40P01 deadlock", pg("40P01"), true},
		{"pg 53300 too many connections", pg("53300"), true},
		{"pg 57P01 admin shutdown", pg("57P01"), true},
		{"pg 57P03 cannot connect now", wrap(pg("57P03")), true},
		{"pg 57014 query canceled (class 57, not 57P)", pg("57014"), false},
		{"pg 23505 unique violation", pg("23505"), false},
		{"pg 42P01 undefined table", wrap(pg("42P01")), false},
		{"pg 22P02 invalid text", pg("22P02"), false},
		// minio
		{"s3 500", s3(500), true},
		{"s3 503", s3(503), true},
		{"s3 429", s3(429), true},
		{"s3 404", s3(404), false},
		{"s3 403", s3(403), false},
		{"s3 400", s3(400), false},
		// ffmpeg / ffprobe exit errors
		{"exit error from false", falseBare, false},
		{"wrapped exit error from false", wrap(falseBare), false},
		{"bare exit error: stderr is not in Error()", netBare, false},
		{"exit error with connection refused output", netWrapped, true},
		{"exit error with 5xx output", http5xx, true},
		{"exit error with 429 output", wrap(http429), true},
		{"exit error with 404 output", http404, false},
		{"exit error with bad input", badInput, false},
		{"exit error, upper-case network text", upperTimeout, true},
		// json
		{"json syntax", syntaxErr, false},
		{"wrapped json syntax", wrap(syntaxErr), false},
		{"json type", typeErr, false},
		{"wrapped json type", wrap(typeErr), false},
		// anything else is retried; MaxAttempts bounds it
		{"unknown", errors.New("boom"), true},
		{"wrapped unknown", wrap(errors.New("boom")), true},
		{"bad request is unknown here", services.ErrBadRequest, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 10, BaseDelay: 10 * time.Second, MaxDelay: 5 * time.Minute}
	tests := []struct {
		retry int
		base  time.Duration
	}{
		{-3, 10 * time.Second}, // clamped to the first retry
		{0, 10 * time.Second},
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{3, 40 * time.Second},
		{4, 80 * time.Second},
		{5, 160 * time.Second},
		{6, 5 * time.Minute}, // 320s capped
		{7, 5 * time.Minute},
		{100, 5 * time.Minute}, // no overflow
	}
	for _, tt := range tests {
		lo, hi := tt.base, tt.base+tt.base/5
		minSeen, maxSeen := time.Duration(1<<62), time.Duration(0)
		for i := 0; i < 2000; i++ {
			d := p.Backoff(tt.retry)
			if d < lo || d > hi {
				t.Fatalf("Backoff(%d) = %v, want within [%v, %v]", tt.retry, d, lo, hi)
			}
			minSeen, maxSeen = min(minSeen, d), max(maxSeen, d)
		}
		// jitter actually varies (20% of >= 10s is a wide range)
		if maxSeen-minSeen < tt.base/20 {
			t.Errorf("Backoff(%d) jitter range %v..%v looks constant", tt.retry, minSeen, maxSeen)
		}
	}

	// growth: the jitter-free floor doubles
	for r := 1; r < 5; r++ {
		if p.Backoff(r+1) < 2*p.BaseDelay<<(r-1) {
			t.Errorf("Backoff(%d) did not double", r+1)
		}
	}

	// tiny delays: jitter bound is d/5+1, so a 1ns base stays in [1ns, 1ns]
	tiny := RetryPolicy{MaxAttempts: 2, BaseDelay: time.Nanosecond, MaxDelay: time.Nanosecond}
	for i := 0; i < 100; i++ {
		if d := tiny.Backoff(1); d != time.Nanosecond {
			t.Fatalf("tiny backoff = %v", d)
		}
	}
	// base above max is capped
	odd := RetryPolicy{MaxAttempts: 2, BaseDelay: time.Minute, MaxDelay: time.Second}
	if d := odd.Backoff(1); d < time.Second || d > time.Second+time.Second/5 {
		t.Errorf("base > max: %v", d)
	}
}

func TestRetryPolicyFromConfig(t *testing.T) {
	def := DefaultRetryPolicy
	if def != (RetryPolicy{MaxAttempts: 4, BaseDelay: 10 * time.Second, MaxDelay: 5 * time.Minute}) {
		t.Fatalf("DefaultRetryPolicy = %+v", def)
	}
	tests := []struct {
		name string
		in   config.Worker
		want RetryPolicy
	}{
		{"empty section", config.Worker{}, def},
		{"all overridden", config.Worker{MaxAttempts: 6, RetryBaseDelay: "2s", RetryMaxDelay: "1m"}, RetryPolicy{6, 2 * time.Second, time.Minute}},
		{"only attempts", config.Worker{MaxAttempts: 1}, RetryPolicy{1, def.BaseDelay, def.MaxDelay}},
		{"only base", config.Worker{RetryBaseDelay: "500ms"}, RetryPolicy{def.MaxAttempts, 500 * time.Millisecond, def.MaxDelay}},
		{"only max", config.Worker{RetryMaxDelay: "10m"}, RetryPolicy{def.MaxAttempts, def.BaseDelay, 10 * time.Minute}},
		{"zero attempts -> default", config.Worker{MaxAttempts: 0}, def},
		{"negative attempts -> default", config.Worker{MaxAttempts: -2}, def},
		{"unparsable durations -> default", config.Worker{RetryBaseDelay: "soon", RetryMaxDelay: "10"}, def},
		{"negative durations -> default", config.Worker{RetryBaseDelay: "-5s", RetryMaxDelay: "-1m"}, def},
		{"zero durations -> default", config.Worker{RetryBaseDelay: "0s", RetryMaxDelay: "0"}, def},
		{"max below base is raised to base", config.Worker{RetryBaseDelay: "2m", RetryMaxDelay: "30s"}, RetryPolicy{def.MaxAttempts, 2 * time.Minute, 2 * time.Minute}},
		{"max below default base", config.Worker{RetryMaxDelay: "5s"}, RetryPolicy{def.MaxAttempts, def.BaseDelay, def.BaseDelay}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RetryPolicyFromConfig(tt.in); got != tt.want {
				t.Errorf("RetryPolicyFromConfig(%+v) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
	// the package default is not mutated by a call
	_ = RetryPolicyFromConfig(config.Worker{MaxAttempts: 9})
	if DefaultRetryPolicy != def {
		t.Error("RetryPolicyFromConfig mutated DefaultRetryPolicy")
	}
}

func TestThumbnailSeek(t *testing.T) {
	tests := []struct{ dur, want int }{
		{-5, 0},
		{0, 0},
		{1, 0},
		{2, 1},
		{3, 1},
		{4, 2},
		{5, 2},
		{6, 3},
		{7, 3},
		{60, 3},
		{36000, 3},
	}
	for _, tt := range tests {
		got := thumbnailSeek(tt.dur)
		if got != tt.want {
			t.Errorf("thumbnailSeek(%d) = %d, want %d", tt.dur, got, tt.want)
		}
		// the seek always lands inside the clip
		if tt.dur > 0 && got >= tt.dur {
			t.Errorf("thumbnailSeek(%d) = %d is past the end", tt.dur, got)
		}
	}
}
