package workers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/queue"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
)

type Transcode struct {
	service *services.VideoService
	store   storage.ObjectStore
	queue   queue.Queue
	ffm     *media.FFM
}

func NewTranscoder(service *services.VideoService, store storage.ObjectStore, queue queue.Queue, ffm *media.FFM) *Transcode {
	return &Transcode{
		service: service,
		store:   store,
		queue:   queue,
		ffm:     ffm,
	}
}

var sizes = []struct {
	height int
	width  int
	label  string
}{
	{640, 360, "360p"},
	{426, 240, "240p"},
	{854, 480, "480p"},
	{1280, 720, "720p"},
	{1920, 1080, "1080p"},
}

func (c *Transcode) Handle(ctx context.Context, payload jobs.JobPayload) error {
	fmt.Printf("transcoding started [videoId: %s] \n", payload.VideoID)
	v, err := c.service.GetVideo(ctx, payload.VideoID)
	if err != nil {
		return err
	}
	inPath := filepath.Join(os.TempDir(), filepath.Base(v.OriginalObjectKey))
	if err := c.store.SaveLocally(ctx, v.OriginalObjectKey, inPath); err != nil {
		return err
	}
	defer os.Remove(inPath)
	outDir := filepath.Join(os.TempDir(), "transcode", payload.VideoID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(outDir)
	availableQualities := []string{}
	for _, s := range sizes {
		fmt.Println("transcoding for " + s.label)
		outPath := filepath.Join(outDir, fmt.Sprintf("%s.mp4", s.label))
		if err := c.ffm.TranscodeH264(ctx, inPath, outPath, s.width, s.height); err != nil {
			return err
		}
		key := filepath.Join("transcoded", payload.VideoID, fmt.Sprintf("%s.mp4", s.label))
		if _, err := c.store.UploadLocalFile(ctx, key, outPath, "video/mp4"); err != nil {
			return err
		}
		availableQualities = append(availableQualities, s.label)
	}
	if err := c.service.UpdateStatus(ctx, payload.VideoID, models.StatusProcessing, jobs.StepSegment, []repository.ExtraFields{{Val: availableQualities, Field: "available_qualities"}}); err != nil {
		return err
	}
	fmt.Printf("transcoding finished [videoId: %s] \n", payload.VideoID)
	return nil
}
