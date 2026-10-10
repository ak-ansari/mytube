package storage

import (
	"os"
	"strings"
	"testing"

	"github.com/ak-ansari/mytube/pkg/logger"
)

type nopLogger struct{ errors int }

func (*nopLogger) Debug(string, ...logger.Field)   {}
func (*nopLogger) Info(string, ...logger.Field)    {}
func (*nopLogger) Success(string, ...logger.Field) {}
func (*nopLogger) Warn(string, ...logger.Field)    {}
func (l *nopLogger) Error(string, ...logger.Field) { l.errors++ }
func (*nopLogger) Fatal(string, ...logger.Field)   {}
func (*nopLogger) Flush()                          {}

// ParseBucketEvent and GetEventName never touch the MinIO client, so an S3Store without one is enough.
func newParser() (*S3Store, *nopLogger) {
	l := &nopLogger{}
	return &S3Store{log: l}, l
}

func TestParseBucketEventSample(t *testing.T) {
	raw, err := os.ReadFile("../workers/testdata/sample_event.json")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := newParser()
	ev, err := s.ParseBucketEvent(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := BucketEvent{
		Event:       BucketEventCreated,
		ObjectKey:   "originals/60b45786-76b6-4983-905d-926f97197367/original.mkv",
		Etag:        "7cece7b174b13e45b9f381d2c0402cde",
		Size:        21308703,
		ContentType: "video/mp4",
		Bucket:      "mytube",
	}
	if *ev != want {
		t.Errorf("event = %+v\nwant    %+v", *ev, want)
	}
}

func TestParseBucketEvent(t *testing.T) {
	event := func(name, key string) string {
		return `{"Records":[{"eventName":"` + name + `","s3":{"bucket":{"name":"b"},"object":{"key":"` + key + `","size":5}}}]}`
	}
	tests := []struct {
		name      string
		raw       string
		wantErr   bool
		wantEvent string
		wantKey   string
	}{
		{"put", event("s3:ObjectCreated:Put", "originals%2Fa%2Foriginal.mp4"), false, BucketEventCreated, "originals/a/original.mp4"},
		{"multipart", event("s3:ObjectCreated:CompleteMultipartUpload", "originals/a/b.mp4"), false, BucketEventCreated, "originals/a/b.mp4"},
		{"delete", event("s3:ObjectRemoved:Delete", "originals%2Fa"), false, BucketEventDeleted, "originals/a"},
		{"other event", event("s3:ObjectAccessed:Get", "k"), false, "", "k"},
		{"plus in key decodes to space", event("s3:ObjectCreated:Put", "originals%2Fmy+file.mp4"), false, BucketEventCreated, "originals/my file.mp4"},
		{"bad escape", event("s3:ObjectCreated:Put", "originals%2"), true, "", ""},
		{"no records", `{"Records":[]}`, true, "", ""},
		{"empty object", `{}`, true, "", ""},
		{"not json", `{"Records":`, true, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newParser()
			ev, err := s.ParseBucketEvent(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if tt.wantErr {
				return
			}
			if ev.Event != tt.wantEvent || ev.ObjectKey != tt.wantKey || ev.Bucket != "b" || ev.Size != 5 {
				t.Errorf("event = %+v", ev)
			}
		})
	}
	s, l := newParser()
	_, _ = s.ParseBucketEvent("garbage")
	if l.errors != 1 {
		t.Errorf("unmarshal failure should be logged once, got %d", l.errors)
	}
}

func TestGetEventName(t *testing.T) {
	s, _ := newParser()
	for in, want := range map[string]string{
		"s3:ObjectCreated:Put":                 BucketEventCreated,
		"s3:ObjectCreated:Copy":                BucketEventCreated,
		"s3:ObjectRemoved:Delete":              BucketEventDeleted,
		"s3:ObjectRemoved:DeleteMarkerCreated": BucketEventDeleted,
		"":                                     "",
		"ObjectCreated:Put":                    "",
		"s3:objectcreated:put":                 "",
	} {
		if got := s.GetEventName(in); got != want {
			t.Errorf("GetEventName(%q) = %q, want %q", in, got, want)
		}
	}
	if !strings.HasPrefix(DirectoryOriginals, "originals") {
		t.Error("directory constant changed")
	}
}
