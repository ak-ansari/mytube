package jobs

import "errors"

// PermanentError marks a failure that retrying cannot fix (bad input, missing video, ...).
// The runner sends it straight to the dead-letter table and fails the video.
type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// Permanent wraps err so the runner does not retry it. Permanent(nil) is nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}

// IsPermanent reports whether err (or anything it wraps) was marked with Permanent.
func IsPermanent(err error) bool {
	var p *PermanentError
	return errors.As(err, &p)
}
