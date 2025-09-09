package workers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/util"
)

type Transcode struct {
	service *services.VideoService
	store   storage.ObjectStore
	ffm     *media.FFM
}

func NewTranscoder(service *services.VideoService, store storage.ObjectStore, ffm *media.FFM) *Transcode {
	return &Transcode{
		service: service,
		store:   store,
		ffm:     ffm,
	}
}

func (c *Transcode) Handle(ctx context.Context, payload jobs.JobPayload) error {
	fmt.Printf("transcoding started [videoId: %s] \n", payload.VideoID)
	v, err := c.service.GetVideo(ctx, payload.VideoID)
	if err != nil {
		return err
	}
	fmt.Printf("file info retrieved [videoId: %s] \n", payload.VideoID)
	tempDir := filepath.Join(os.TempDir(), payload.VideoID)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return err
	}
	url, err := c.service.GetDownloadUrl(ctx, v.OriginalObjectKey)
	if err != nil {
		return err
	}
	defer os.Remove(tempDir)
	fmt.Printf("file downloaded for transcoding [videoId: %s] \n", payload.VideoID)

	availableQualities := []string{}
	ext := filepath.Ext(v.Filename)
	for _, s := range util.Sizes {
		fmt.Println("transcoding for " + s.Label)
		outPath := filepath.Join(tempDir, fmt.Sprintf("%s%s", s.Label, ext))
		if err := c.ffm.TranscodeH264(ctx, url, outPath, s.Width, s.Height); err != nil {
			return err
		}
		fmt.Printf("transcoding finished [videoId: %s] [quality: %s] \n", payload.VideoID, s.Label)
		key := c.service.GetTranscodingPath(payload.VideoID, s.Label, ext)
		fmt.Printf("uploading transcoded file to [remote address]:%s \n", key)
		if _, err := c.store.UploadLocalFile(ctx, key, outPath, fmt.Sprintf("video/%s", strings.TrimPrefix(ext, "."))); err != nil {
			return err
		}
		fmt.Printf("uploaded transcoded file to [remote address]:%s \n", key)
		availableQualities = append(availableQualities, s.Label)
	}
	if err := c.service.UpdateQualities(ctx, payload.VideoID, availableQualities, models.StatusProcessing); err != nil {
		return err
	}
	fmt.Printf("transcoding finished [videoId: %s] \n", payload.VideoID)
	return nil
}
