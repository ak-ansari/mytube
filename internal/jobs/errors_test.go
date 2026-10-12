package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestPermanent(t *testing.T) {
	if Permanent(nil) != nil {
		t.Error("Permanent(nil) should be nil")
	}
	if IsPermanent(nil) {
		t.Error("IsPermanent(nil)")
	}

	cause := errors.New("bad input")
	p := Permanent(cause)
	if p.Error() != "bad input" {
		t.Errorf("message = %q, want the cause's message", p.Error())
	}
	if !IsPermanent(p) {
		t.Error("IsPermanent(Permanent(err)) = false")
	}
	if !errors.Is(p, cause) || errors.Unwrap(p) != cause {
		t.Error("Permanent must unwrap to its cause")
	}
	var pe *PermanentError
	if !errors.As(p, &pe) || pe.Err != cause {
		t.Errorf("errors.As = %+v", pe)
	}

	wrapped := fmt.Errorf("stage 4: %w", fmt.Errorf("transcode: %w", p))
	if !IsPermanent(wrapped) || !errors.Is(wrapped, cause) {
		t.Error("a wrapped Permanent is still permanent and still matches its cause")
	}
	if IsPermanent(cause) || IsPermanent(fmt.Errorf("x: %w", cause)) || IsPermanent(errors.New("other")) {
		t.Error("unmarked errors are not permanent")
	}
	// %v formatting does not keep the mark
	if IsPermanent(fmt.Errorf("x: %v", p)) {
		t.Errorf("formatting with %%v breaks the chain, so it is not permanent")
	}
	// double marking is harmless
	if pp := Permanent(p); !IsPermanent(pp) || !errors.Is(pp, cause) {
		t.Error("Permanent(Permanent(err))")
	}
	// joined errors: permanent if any part is
	if !IsPermanent(errors.Join(errors.New("a"), p)) {
		t.Error("errors.Join with a permanent part")
	}
}

func TestJobPayloadAttempt(t *testing.T) {
	// attempt 0 is omitted, so payloads queued before retries existed decode the same
	b, _ := json.Marshal(JobPayload{VideoID: "v", Stage: 4, Status: "thumbnail_generated"})
	if string(b) != `{"video_id":"v","stage":4,"status":"thumbnail_generated"}` {
		t.Errorf("first attempt payload = %s", b)
	}
	b, _ = json.Marshal(JobPayload{VideoID: "v", Stage: 4, Attempt: 2})
	var p JobPayload
	if err := json.Unmarshal(b, &p); err != nil || p.Attempt != 2 {
		t.Errorf("round trip = %+v, %v (%s)", p, err, b)
	}
	p = JobPayload{}
	if err := json.Unmarshal([]byte(`{"video_id":"v","stage":3}`), &p); err != nil || p.Attempt != 0 {
		t.Errorf("legacy payload = %+v, %v", p, err)
	}
}
