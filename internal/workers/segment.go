package workers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/util"
)

type Segment struct {
	service *services.VideoService
	store   storage.ObjectStore
	ffm     *media.FFM
}

func NewSegment(service *services.VideoService, store storage.ObjectStore, ffm *media.FFM) *Segment {
	return &Segment{
		service: service,
		store:   store,
		ffm:     ffm,
	}
}

func (s *Segment) Handle(ctx context.Context, payload jobs.JobPayload) error {
	fmt.Printf("segment processing started [videoId: %s] \n\n", payload.VideoID)

	// get file info from db
	v, err := s.service.GetVideo(ctx, payload.VideoID)
	if err != nil {
		return err
	}
	// get quality to metadata mapping
	qualityMap := util.GetQualityMap()

	// directories to work with
	remoteDir := s.service.GetHlsDir(payload.VideoID)
	tempDir := filepath.Join(os.TempDir(), payload.VideoID)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	// manifesfile init
	manifest := "#EXTM3U\n"

	// loop over each available quality and process the segment generation
	for _, quality := range v.AvailableQualities {
		fmt.Printf("creating segments for [video]: %s, quality: %s\n\n", payload.VideoID, quality)
		q := qualityMap[quality]
		if err := s.createSegments(ctx, tempDir, v.Filename, payload.VideoID, quality); err != nil {
			return err
		}

		fmt.Printf("segments created for [video]: %s, quality: %s\n\n", payload.VideoID, quality)
		if err := s.uploadHlsFiles(ctx, quality, tempDir, remoteDir); err != nil {
			return err
		}
		fmt.Printf("all segments uploaded [video]: %s, quality: %s\n\n", payload.VideoID, quality)
		manifest += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n%s.m3u8\n",
			q.Bandwidth, q.Width, q.Height, q.Label)

	}
	manifestPath, err := s.uploadMasterPlaylist(ctx, tempDir, remoteDir, manifest)
	if err != nil {
		return err
	}
	fmt.Printf("manifest file uploaded at [path] %s \n\n", manifestPath)
	if err := s.service.UpdateStatus(ctx, payload.VideoID, models.StatusProcessing, jobs.StepThumbs, []repository.ExtraFields{{Val: manifestPath, Field: "manifest_path"}}); err != nil {
		return err
	}
	fmt.Printf("segment processing finished [videoId: %s] \n\n", payload.VideoID)
	return nil
}
func (s *Segment) createSegments(ctx context.Context, tempDir, originalName, id, quality string) error {
	ext := filepath.Ext(originalName)
	key := s.service.GetTranscodingPath(id, quality, ext)
	localFilePath := filepath.Join(tempDir, fmt.Sprintf("temp%s", ext))
	if err := s.store.SaveLocally(ctx, key, localFilePath); err != nil {
		return err
	}

	if err := s.ffm.SegmentHLS(ctx, localFilePath, tempDir, quality, 4); err != nil {
		return err
	}
	return os.Remove(localFilePath)

}

func (s *Segment) uploadHlsFiles(ctx context.Context, quality, localDir, remoteDir string) error {
	pattern := filepath.Join(localDir, fmt.Sprintf("%s*", quality))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, f := range files {
		remotePath := filepath.Join(remoteDir, filepath.Base(f))
		fmt.Printf("uploading segment for [quality]: %s, [file]: %s\n\n", quality, f)
		if _, err := s.store.UploadLocalFile(ctx, remotePath, f, ""); err != nil {
			return err
		}
		fmt.Printf("segment uploaded for [quality]: %s, [file]: %s\n\n", quality, f)
	}
	return nil

}

func (s *Segment) uploadMasterPlaylist(ctx context.Context, localDir, remoteDir, master string) (string, error) {
	manifestName := "master.m3u8"

	localPath := filepath.Join(localDir, manifestName)
	if err := os.WriteFile(localPath, []byte(master), 0644); err != nil {
		return "", err
	}
	remotePath := filepath.Join(remoteDir, manifestName)
	fmt.Printf("uploading manifest file \n")
	key, err := s.store.UploadLocalFile(ctx, remotePath, localPath, "")
	if err != nil {
		return "", err
	}
	return key, nil
}
func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true // file exists
	}
	if os.IsNotExist(err) {
		return false // file does not exist
	}
	// some other error (e.g., permission denied)
	return false
}
