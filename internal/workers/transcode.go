package workers

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/util"
)

type Transcode struct {
	service *services.VideoService
	store   storage.ObjectStore
	ffm     *media.FFM
	log     logger.Logger
}

func NewTranscoder(service *services.VideoService, store storage.ObjectStore, ffm *media.FFM, log logger.Logger) *Transcode {
	return &Transcode{
		service: service,
		store:   store,
		ffm:     ffm,
		log:     log,
	}
}

func (c *Transcode) Handle(ctx context.Context, payload jobs.JobPayload) error {
	c.log.Info("Transcoding started",
		logger.String("videoId", payload.VideoID))

	v, err := c.service.GetVideo(ctx, payload.VideoID)
	if err != nil {
		c.log.Error("Failed to get video info",
			logger.String("videoId", payload.VideoID),
			logger.Error(err))
		return err
	}
	c.log.Success("Video info retrieved",
		logger.String("videoId", payload.VideoID),
		logger.String("filename", v.Filename))

	tempDir := filepath.Join(os.TempDir(), payload.VideoID)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		c.log.Error("Failed to create temp directory",
			logger.String("path", tempDir),
			logger.Error(err))
		return err
	}
	defer os.RemoveAll(tempDir)

	url, err := c.service.GetDownloadUrl(ctx, v.OriginalObjectKey)
	if err != nil {
		c.log.Error("Failed to get download URL",
			logger.String("videoId", payload.VideoID),
			logger.String("key", v.OriginalObjectKey),
			logger.Error(err))
		return err
	}

	availableQualities := []string{}
	ext := filepath.Ext(v.Filename)

	for _, s := range util.Sizes {
		c.log.Info("Transcoding quality started",
			logger.String("videoId", payload.VideoID),
			logger.String("quality", s.Label))

		outPath := filepath.Join(tempDir, s.Label+ext)
		if err := c.ffm.TranscodeH264(ctx, url, outPath, s.Width, s.Height); err != nil {
			c.log.Error("Failed transcoding",
				logger.String("videoId", payload.VideoID),
				logger.String("quality", s.Label),
				logger.Error(err))
			return err
		}

		key := c.service.GetTranscodingPath(payload.VideoID, s.Label, ext)
		c.log.Info("Uploading transcoded file",
			logger.String("videoId", payload.VideoID),
			logger.String("quality", s.Label),
			logger.String("remotePath", key))

		if _, err := c.store.UploadLocalFile(ctx, key, outPath, "video/"+strings.TrimPrefix(ext, ".")); err != nil {
			c.log.Error("Failed to upload transcoded file",
				logger.String("videoId", payload.VideoID),
				logger.String("quality", s.Label),
				logger.String("remotePath", key),
				logger.Error(err))
			return err
		}

		c.log.Success("Transcoded file uploaded",
			logger.String("videoId", payload.VideoID),
			logger.String("quality", s.Label),
			logger.String("remotePath", key))

		availableQualities = append(availableQualities, s.Label)
	}

	if err := c.service.UpdateQualities(ctx, payload.VideoID, availableQualities, models.StatusProcessing); err != nil {
		c.log.Error("Failed to update qualities in DB",
			logger.String("videoId", payload.VideoID),
			logger.Error(err))
		return err
	}

	c.log.Success("Transcoding finished",
		logger.String("videoId", payload.VideoID),
		logger.Any("availableQualities", availableQualities))

	return nil
}
