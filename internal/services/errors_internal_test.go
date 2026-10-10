package services

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/jackc/pgx/v5"
)

func TestNotFound(t *testing.T) {
	other := errors.New("db down")
	tests := []struct {
		name string
		in   error
		want error
	}{
		{"no rows", pgx.ErrNoRows, ErrNotFound},
		{"wrapped no rows", fmt.Errorf("query: %w", pgx.ErrNoRows), ErrNotFound},
		{"other error", other, other},
		{"nil", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notFound(tt.in); !errors.Is(got, tt.want) || (tt.want == nil && got != nil) {
				t.Fatalf("notFound(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	errs := []error{ErrNotFound, ErrForbidden, ErrBadRequest}
	for i, a := range errs {
		for j, b := range errs {
			if (i == j) != errors.Is(a, b) {
				t.Errorf("errors.Is(%v, %v) = %v", a, b, errors.Is(a, b))
			}
		}
	}
	wrapped := fmt.Errorf("%w: title must be 3-100 characters", ErrBadRequest)
	if !errors.Is(wrapped, ErrBadRequest) || wrapped.Error() != "bad request: title must be 3-100 characters" {
		t.Errorf("wrapped = %q", wrapped)
	}
}

func TestValidThumbnailKey(t *testing.T) {
	const id = "65ae26a1-c831-4010-8a2e-82a2e35b0292"
	tests := []struct {
		key  string
		want bool
	}{
		{"thumbnails/" + id + "/" + id + ".jpeg", true},
		{"thumbnails/" + id + "/0b0f5e0e-1111-2222-3333-444444444444.png", true},
		{"thumbnails/" + id + "/", false},
		{"thumbnails/" + id, false},
		{"thumbnails/other/" + id + ".jpeg", false},
		{"thumbnails/" + id + "/../other/a.png", false},
		{"thumbnails/" + id + "/a..png", false}, // any ".." is rejected
		{"avatars/" + id + "/a.png", false},
		{"/thumbnails/" + id + "/a.png", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := validThumbnailKey(id, tt.key); got != tt.want {
			t.Errorf("validThumbnailKey(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestIsFailed(t *testing.T) {
	for status, want := range map[models.VideoStatus]bool{
		models.StatusPhase1Failed: true,
		models.StatusPhase2Failed: true,
		models.StatusValid:        false,
		models.StatusPublished:    false,
		"":                        false,
	} {
		if got := isFailed(status); got != want {
			t.Errorf("isFailed(%q) = %v", status, got)
		}
	}
}
