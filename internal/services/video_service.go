package services

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"path/filepath"

	"crypto/sha256"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/queue"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/google/uuid"
)

type UploadResult struct {
	VideoId string `json:"videoId"`
	Key     string `json:"key"`
	Sha256  string `json:"sha256"`
}
type VideoService struct {
	objStore  storage.ObjectStore
	repo      repository.VideoRepository
	queue     queue.Queue
	queueName string
}

func NewVideoService(objStore storage.ObjectStore, repo repository.VideoRepository, queue queue.Queue, cnf *config.Config) *VideoService {
	return &VideoService{
		objStore:  objStore,
		queueName: cnf.Queue.RedisQueueName,
		queue:     queue,
		repo:      repo,
	}
}

func (vs *VideoService) UploadVideo(ctx context.Context, file *multipart.FileHeader) (*UploadResult, error) {
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := &bytes.Buffer{}
	hash := sha256.New()
	mw := io.MultiWriter(buf, hash)
	if _, err := io.Copy(mw, f); err != nil {
		return nil, err
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	id := uuid.New()
	ext := filepath.Ext(file.Filename)
	key := filepath.Join("originals", id.String(), "original"+ext)

	path, err := vs.objStore.Put(ctx, key, buf, int64(buf.Len()))
	if err != nil {
		return nil, err
	}

	// save meta in db
	v := models.Video{
		ID:                id,
		Filename:          file.Filename,
		OriginalObjectKey: path,
		Status:            models.StatusUploaded,
	}
	if err := vs.repo.InsertBasic(ctx, v); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(jobs.JobPayload{VideoID: id.String(), Step: jobs.StepValidate})
	if err != nil {
		return nil, err
	}
	if err := vs.queue.Enqueue(ctx, vs.queueName, payload); err != nil {
		return nil, err
	}
	return &UploadResult{VideoId: id.String(), Key: path, Sha256: sum}, nil
}
func (vs *VideoService) GetVideo(ctx context.Context, id string) (*models.Video, error) {
	return vs.repo.Get(ctx, id)
}
func (vs *VideoService) DownloadVideo() string {
	return "video is downloaded"
}
func (vs *VideoService) UpdateMeta(ctx context.Context, videoId string, sha string, dur int, vcodec, acodec string, w, h int) error {
	return vs.repo.UpdateMeta(ctx, videoId, sha, dur, vcodec, acodec, w, h)
}
func (vs *VideoService) UpdateStatus(ctx context.Context, videoId string, status models.VideoStatus, nextStep jobs.Step, extraProperties []repository.ExtraFields) error {
	if err := vs.repo.UpdateStatus(ctx, videoId, status, extraProperties); err != nil {
		return err
	}
	j, err := json.Marshal(jobs.JobPayload{VideoID: videoId, Step: nextStep})
	if err != nil {
		return err
	}
	return vs.queue.Enqueue(ctx, vs.queueName, j)
}
