package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
)

type Validate struct {
	service *services.VideoService
	store   storage.ObjectStore
	ffm     *media.FFM
	log     logger.Logger
}

func NewValidate(service *services.VideoService, store storage.ObjectStore, ffm *media.FFM, log logger.Logger) *Validate {
	return &Validate{
		service: service,
		store:   store,
		ffm:     ffm,
		log:     log,
	}
}

// Handle validates a video file
func (c *Validate) Handle(ctx context.Context, p jobs.JobPayload) error {
	c.log.Info("Validation started",
		logger.String("videoId", p.VideoID))

	key, err := c.service.GetVideoKey(ctx, p.VideoID)
	if err != nil {
		c.log.Error("Failed to get video key",
			logger.String("videoId", p.VideoID),
			logger.Error(err))
		return err
	}

	f, _, err := c.store.Get(ctx, key)
	if err != nil {
		c.log.Error("Failed to get file from store",
			logger.String("videoId", p.VideoID),
			logger.String("key", key),
			logger.Error(err))
		return err
	}

	temp, err := os.CreateTemp("", "video-*")
	if err != nil {
		c.log.Error("Failed to create temp file",
			logger.String("videoId", p.VideoID),
			logger.Error(err))
		return err
	}
	defer temp.Close()
	defer os.Remove(temp.Name())

	c.log.Success("Temp file created",
		logger.String("videoId", p.VideoID),
		logger.String("path", temp.Name()))

	h := sha256.New()
	reader := io.TeeReader(f, h)
	if _, err := io.Copy(temp, reader); err != nil {
		c.log.Error("Failed to copy to temp file",
			logger.String("videoId", p.VideoID),
			logger.Error(err))
		return err
	}
	if err := temp.Sync(); err != nil {
		c.log.Error("Failed to sync temp file",
			logger.String("videoId", p.VideoID),
			logger.Error(err))
		return err
	}

	pr, err := c.ffm.Probe(ctx, temp.Name())
	if err != nil {
		c.log.Error("Failed to probe video",
			logger.String("videoId", p.VideoID),
			logger.Error(err))
		return err
	}

	sum := hex.EncodeToString(h.Sum(nil))

	var acodec, vcodec string
	var wpx, hpx, dur int

	for _, s := range pr.Streams {
		if s.CodecType == "video" {
			vcodec = s.CodecName
			wpx = s.Width
			hpx = s.Height
		} else if s.CodecType == "audio" {
			acodec = s.CodecName
		}
	}

	if pr.Format.Duration != "" {
		if d, err := parseDur(pr.Format.Duration); err == nil && d > 0 {
			dur = d
		}
	}

	if err := c.service.UpdateMeta(ctx, p.VideoID, sum, dur, vcodec, acodec, wpx, hpx, models.StatusValid); err != nil {
		c.log.Error("Failed to update video metadata",
			logger.String("videoId", p.VideoID),
			logger.Error(err))
		return err
	}

	c.log.Success("Validation finished",
		logger.String("videoId", p.VideoID),
		logger.String("checksum", sum),
		logger.Int("duration", dur),
		logger.String("vcodec", vcodec),
		logger.String("acodec", acodec),
		logger.Int("width", wpx),
		logger.Int("height", hpx))

	return nil
}

// parseDur converts ffmpeg duration string to int seconds
func parseDur(s string) (int, error) {
	var sec float64
	_, err := fmt.Sscanf(s, "%f", &sec)
	return int(sec + 0.5), err
}
