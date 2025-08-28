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
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
)

type Validate struct {
	service *services.VideoService
	store   storage.ObjectStore
	ffm     *media.FFM
}

func NewValidate(service *services.VideoService, store storage.ObjectStore, ffm *media.FFM) *Validate {
	return &Validate{
		service: service,
		store:   store,
		ffm:     ffm,
	}
}

// Handle remains exported since workers will call it
func (c *Validate) Handle(ctx context.Context, payload jobs.JobPayload) error {
	videoId := payload.VideoID
	fmt.Printf("validation started [videoId: %s] \n", videoId)

	v, err := c.service.GetVideo(ctx, videoId)
	if err != nil {
		return err
	}

	f, _, err := c.store.Get(ctx, v.OriginalObjectKey)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp("", "video-*")
	if err != nil {
		return err
	}
	defer temp.Close()
	defer os.Remove(temp.Name())
	h := sha256.New()
	reader := io.TeeReader(f, h)
	if _, err := io.Copy(temp, reader); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	pr, err := c.ffm.Probe(ctx, temp.Name())
	if err != nil {
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

	if err := c.service.UpdateMeta(ctx, videoId, sum, dur, vcodec, acodec, wpx, hpx); err != nil {
		return err
	}

	if err := c.service.UpdateStatus(ctx, videoId, models.StatusValid, jobs.StepTranscode); err != nil {
		return err
	}

	fmt.Printf("validation finished [videoId: %s] \n", payload.VideoID)
	return nil
}

// helper stays private
func parseDur(s string) (int, error) {
	var sec float64
	_, err := fmt.Sscanf(s, "%f", &sec)
	return int(sec + 0.5), err
}
