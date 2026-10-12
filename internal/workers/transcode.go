package workers

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/media"

	// "github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/util"
	"github.com/ak-ansari/mytube/pkg/logger"
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

func (c *Transcode) Handle(ctx context.Context, payload *jobs.JobPayload) error {
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

	tempDir, err := os.MkdirTemp("", "mytube-transcode-"+payload.VideoID+"-*")
	if err != nil {
		c.log.Error("Failed to create temp directory",
			logger.String("videoId", payload.VideoID),
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

	sourceHeight := 0
	if v.Height != nil {
		sourceHeight = *v.Height
	}
	for _, s := range targetSizes(sourceHeight) {
		c.log.Info("Transcoding quality started",
			logger.String("videoId", payload.VideoID),
			logger.String("quality", s.Label))

		outPath := filepath.Join(tempDir, s.Label+".mp4")
		if err := c.ffm.TranscodeH264(ctx, url, outPath, s.Width, s.Height); err != nil {
			c.log.Error("Failed transcoding",
				logger.String("videoId", payload.VideoID),
				logger.String("quality", s.Label),
				logger.Error(err))
			return err
		}

		key := c.service.GetTranscodingPath(payload.VideoID, s.Label)
		c.log.Info("Uploading transcoded file",
			logger.String("videoId", payload.VideoID),
			logger.String("quality", s.Label),
			logger.String("remotePath", key))

		if _, err := c.store.UploadLocalFile(ctx, key, outPath, "video/mp4"); err != nil {
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

	if err := c.service.UpdateQualities(ctx, payload.VideoID, availableQualities); err != nil {
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

// targetSizes returns the renditions to produce: no upscaling past the source height,
// but always at least the smallest one (also when the source height is unknown).
func targetSizes(sourceHeight int) []util.Quality {
	sizes := []util.Quality{}
	for _, s := range util.Sizes {
		if s.Height <= sourceHeight {
			sizes = append(sizes, s)
		}
	}
	if len(sizes) == 0 {
		sizes = append(sizes, util.Sizes[0])
	}
	return sizes
}
