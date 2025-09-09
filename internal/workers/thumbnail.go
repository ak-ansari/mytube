package workers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
)

type Thumbnail struct {
	service *services.VideoService
	ffm     *media.FFM
	store   storage.ObjectStore
}

func NewThumbnail(s *services.VideoService, ffm *media.FFM, store storage.ObjectStore) *Thumbnail {
	return &Thumbnail{
		service: s,
		ffm:     ffm,
		store:   store,
	}
}

func (t *Thumbnail) Handle(ctx context.Context, payload jobs.JobPayload) error {
	fmt.Println("Creating Thumbnail for ", payload.VideoID)
	key, err := t.service.GetVideoKey(ctx, payload.VideoID)
	if err != nil {
		return err
	}
	url, err := t.service.GetDownloadUrl(ctx, key)
	if err != nil {
		return err
	}
	outDir := filepath.Join(os.TempDir(), payload.VideoID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	filename := fmt.Sprintf("%s.jpeg", payload.VideoID)
	outPath := filepath.Join(outDir, filename)

	if err := t.ffm.CreateThumbnail(ctx, url, outPath, 3); err != nil {
		return err
	}
	fmt.Printf("Thumbnail is created for [videoId]: %s\n", payload.VideoID)
	remotePath := filepath.Join("thumbnails", payload.VideoID, filename)
	fmt.Printf("uploading Thumbnail to [Remote Path]: %s\n", remotePath)
	if _, err := t.store.UploadLocalFile(ctx, remotePath, outPath, "image/jpeg"); err != nil {
		return err
	}
	fmt.Printf("uploading Thumbnail path in db: %s\n", remotePath)
	if err := t.service.UpdateThumbnail(ctx, payload.VideoID, remotePath); err != nil {
		return err
	}
	fmt.Println("Thumbnail creation step finished ", payload.VideoID)
	return nil
}
