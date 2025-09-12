package workers

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
)

type Thumbnail struct {
	service *services.VideoService
	ffm     *media.FFM
	store   storage.ObjectStore
	log     logger.Logger
}

func NewThumbnail(s *services.VideoService, ffm *media.FFM, store storage.ObjectStore, log logger.Logger) *Thumbnail {
	return &Thumbnail{
		service: s,
		ffm:     ffm,
		store:   store,
		log:     log,
	}
}

func (t *Thumbnail) Handle(ctx context.Context, payload jobs.JobPayload) error {
	t.log.Info("Creating thumbnail",
		logger.String("videoId", payload.VideoID))

	key, err := t.service.GetVideoKey(ctx, payload.VideoID)
	if err != nil {
		t.log.Error("Failed to get video key",
			logger.String("videoId", payload.VideoID),
			logger.Error(err))
		return err
	}

	url, err := t.service.GetDownloadUrl(ctx, key)
	if err != nil {
		t.log.Error("Failed to get video download URL",
			logger.String("videoId", payload.VideoID),
			logger.Error(err))
		return err
	}

	outDir := filepath.Join(os.TempDir(), payload.VideoID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.log.Error("Failed to create temp directory",
			logger.String("path", outDir),
			logger.Error(err))
		return err
	}

	filename := payload.VideoID + ".jpeg"
	outPath := filepath.Join(outDir, filename)

	if err := t.ffm.CreateThumbnail(ctx, url, outPath, 3); err != nil {
		t.log.Error("Failed to create thumbnail",
			logger.String("videoId", payload.VideoID),
			logger.String("file", outPath),
			logger.Error(err))
		return err
	}

	t.log.Success("Thumbnail created",
		logger.String("videoId", payload.VideoID),
		logger.String("file", outPath))

	remotePath := filepath.Join("thumbnails", payload.VideoID, filename)
	t.log.Info("Uploading thumbnail",
		logger.String("videoId", payload.VideoID),
		logger.String("remotePath", remotePath))

	if _, err := t.store.UploadLocalFile(ctx, remotePath, outPath, "image/jpeg"); err != nil {
		t.log.Error("Failed to upload thumbnail",
			logger.String("videoId", payload.VideoID),
			logger.String("remotePath", remotePath),
			logger.Error(err))
		return err
	}

	if err := t.service.UpdateThumbnail(ctx, payload.VideoID, remotePath); err != nil {
		t.log.Error("Failed to update thumbnail in DB",
			logger.String("videoId", payload.VideoID),
			logger.String("remotePath", remotePath),
			logger.Error(err))
		return err
	}

	t.log.Success("Thumbnail creation step finished",
		logger.String("videoId", payload.VideoID),
		logger.String("remotePath", remotePath))

	return nil
}
