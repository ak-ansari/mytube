package workers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/util"
)

type Segment struct {
	service *services.VideoService
	store   storage.ObjectStore
	ffm     *media.FFM
	log     logger.Logger
}

func NewSegment(service *services.VideoService, store storage.ObjectStore, ffm *media.FFM, log logger.Logger) *Segment {
	return &Segment{
		service: service,
		store:   store,
		ffm:     ffm,
		log:     log,
	}
}

func (s *Segment) Handle(ctx context.Context, payload jobs.JobPayload) error {
	s.log.Info("Segment processing started",
		logger.String("videoId", payload.VideoID))

	// get file info from db
	v, err := s.service.GetVideo(ctx, payload.VideoID)
	if err != nil {
		s.log.Error("Failed to get video info",
			logger.String("videoId", payload.VideoID),
			logger.Error(err))
		return err
	}

	// get quality to metadata mapping
	qualityMap := util.GetQualityMap()

	// directories to work with
	remoteDir := s.service.GetHlsDir(payload.VideoID)
	tempDir := filepath.Join(os.TempDir(), payload.VideoID)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		s.log.Error("Failed to create temp dir",
			logger.String("path", tempDir),
			logger.Error(err))
		return err
	}
	defer os.RemoveAll(tempDir)

	// manifest file init
	manifest := "#EXTM3U\n"

	// loop over each available quality and process segment generation
	for _, quality := range v.AvailableQualities {
		s.log.Info("Creating segments",
			logger.String("videoId", payload.VideoID),
			logger.String("quality", quality))

		q := qualityMap[quality]
		if err := s.createSegments(ctx, tempDir, v.Filename, payload.VideoID, quality); err != nil {
			s.log.Error("Failed to create segments",
				logger.String("videoId", payload.VideoID),
				logger.String("quality", quality),
				logger.Error(err))
			return err
		}

		if err := s.uploadHlsFiles(ctx, quality, tempDir, remoteDir); err != nil {
			s.log.Error("Failed to upload HLS files",
				logger.String("videoId", payload.VideoID),
				logger.String("quality", quality),
				logger.Error(err))
			return err
		}

		manifest += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n%s.m3u8\n",
			q.Bandwidth, q.Width, q.Height, q.Label)
	}

	manifestPath, err := s.uploadMasterPlaylist(ctx, tempDir, remoteDir, manifest)
	if err != nil {
		s.log.Error("Failed to upload master playlist",
			logger.String("videoId", payload.VideoID),
			logger.Error(err))
		return err
	}

	if err := s.service.UpdateManifest(ctx, payload.VideoID, manifestPath); err != nil {
		s.log.Error("Failed to update manifest in DB",
			logger.String("videoId", payload.VideoID),
			logger.String("path", manifestPath),
			logger.Error(err))
		return err
	}

	s.log.Success("Segment processing finished",
		logger.String("videoId", payload.VideoID),
		logger.String("manifestPath", manifestPath))

	return nil
}

func (s *Segment) createSegments(ctx context.Context, tempDir, originalName, id, quality string) error {
	ext := filepath.Ext(originalName)
	key := s.service.GetTranscodingPath(id, quality, ext)
	url, err := s.service.GetDownloadUrl(ctx, key)
	if err != nil {
		return err
	}

	if err := s.ffm.SegmentHLS(ctx, url, tempDir, quality, 4); err != nil {
		return err
	}
	return nil
}

func (s *Segment) uploadHlsFiles(ctx context.Context, quality, localDir, remoteDir string) error {
	pattern := filepath.Join(localDir, fmt.Sprintf("%s*", quality))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	for _, f := range files {
		filename := filepath.Base(f)
		ext := filepath.Ext(filename)
		remotePath := filepath.Join(remoteDir, filename)
		if ext != ".ts" && ext != ".m3u8" {
			continue
		}
		s.log.Info("Uploading segment",
			logger.String("quality", quality),
			logger.String("file", f))

		if _, err := s.store.UploadLocalFile(ctx, remotePath, f, ""); err != nil {
			return err
		}

		s.log.Success("Segment uploaded",
			logger.String("quality", quality),
			logger.String("file", f))
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
	s.log.Info("Uploading manifest file",
		logger.String("path", remotePath))

	key, err := s.store.UploadLocalFile(ctx, remotePath, localPath, "")
	if err != nil {
		s.log.Error("Failed to upload manifest file",
			logger.String("path", remotePath),
			logger.Error(err))
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
		return false
	}
	return false
}
