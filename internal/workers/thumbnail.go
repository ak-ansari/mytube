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
	"github.com/ak-ansari/mytube/pkg/logger"
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

// thumbnailSeek picks the frame time: 3s in, or the middle of clips shorter than 6s.
func thumbnailSeek(durationSeconds int) int {
	if durationSeconds <= 0 {
		return 0
	}
	if half := durationSeconds / 2; half < 3 {
		return half
	}
	return 3
}

func (t *Thumbnail) Handle(ctx context.Context, payload *jobs.JobPayload) error {
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

	outDir, err := os.MkdirTemp("", "mytube-thumbnail-"+payload.VideoID+"-*")
	if err != nil {
		t.log.Error("Failed to create temp directory",
			logger.String("videoId", payload.VideoID),
			logger.Error(err))
		return err
	}
	defer os.RemoveAll(outDir)

	filename := payload.VideoID + ".jpeg"
	outPath := filepath.Join(outDir, filename)

	seek := 3
	if meta, err := t.service.GetMeta(ctx, payload.VideoID); err == nil && meta.DurationSeconds != nil {
		seek = thumbnailSeek(*meta.DurationSeconds)
	}
	if err := t.ffm.CreateThumbnail(ctx, url, outPath, seek); err != nil {
		t.log.Error("Failed to create thumbnail",
			logger.String("videoId", payload.VideoID),
			logger.String("file", outPath),
			logger.Error(err))
		return err
	}

	// ffmpeg exits 0 without writing a frame when the seek lands past the end of the video
	if !FileExists(outPath) {
		return jobs.Permanent(fmt.Errorf("ffmpeg produced no thumbnail frame at %ds", seek))
	}

	t.log.Success("Thumbnail created",
		logger.String("videoId", payload.VideoID),
		logger.String("file", outPath))

	remotePath := filepath.Join(string(storage.DirectoryThumbnails), payload.VideoID, filename)
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
