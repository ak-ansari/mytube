package storage

import (
	"context"
	"io"
)

type BucketEventType = string

const (
	DirectoryOriginals  string          = "originals"
	DirectoryThumbnails string          = "thumbnails"
	DirectoryTranscoded string          = "transcoded"
	DirectorySegments   string          = "segments"
	BucketEventCreated  BucketEventType = "created"
	BucketEventDeleted  BucketEventType = "deleted"
)

type BucketEvent struct {
	Event       BucketEventType
	ObjectKey   string
	Etag        string
	Size        int64
	ContentType string
	Bucket      string
}

type ObjectStore interface {
	Put(ctx context.Context, fileId string, key string, file io.Reader, size int64) (string, error)
	GerPreSignedPutUrl(ctx context.Context, key string, size int64) (string, error)
	Get(ctx context.Context, key string) (io.Reader, int64, error)
	Delete(ctx context.Context, key string) error
	GetUrl(ctx context.Context, key string) (string, error)
	SaveLocally(ctx context.Context, key string, path string) error
	UploadLocalFile(ctx context.Context, key string, path string, mediaType string) (string, error)
	ParseBucketEvent(rawData string) (*BucketEvent, error)
}
