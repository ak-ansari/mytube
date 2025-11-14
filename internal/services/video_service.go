package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"crypto/sha256"

	"github.com/ak-ansari/mytube/internal/api/dto"
	"github.com/ak-ansari/mytube/internal/cache"
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
	Url     string
}
type VideoService struct {
	objStore          storage.ObjectStore
	videoMetadataRepo repository.VideoMetadataRepository
	videoRepo         repository.VideoRepository
	queue             queue.Queue
	cache             cache.Cache
	queueName         string
}

func NewVideoService(objStore storage.ObjectStore, videoMetadataRepo repository.VideoMetadataRepository, videoRepo repository.VideoRepository, queue queue.Queue, cache cache.Cache, queueName string) *VideoService {
	return &VideoService{
		objStore:          objStore,
		queueName:         queueName,
		queue:             queue,
		cache:             cache,
		videoMetadataRepo: videoMetadataRepo,
		videoRepo:         videoRepo,
	}
}
func (v *VideoService) GetVideoKey(ctx context.Context, id string) (string, error) {
	cacheKey := cache.GetKey(cache.KEY, id)
	var cached string
	if err := v.cache.Get(ctx, cacheKey, &cached); err == nil && cached != "" {
		return cached, nil
	}
	video, err := v.GetVideo(ctx, id)
	if err != nil {
		return "", err
	}
	return video.OriginalObjectKey, v.cache.Set(ctx, cacheKey, video.OriginalObjectKey, 24*time.Hour)
}

// func (v *VideoService) UploadVideo(ctx context.Context, file *multipart.FileHeader) (*UploadResult, error) {
// 	f, err := file.Open()
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer f.Close()
// 	buf := &bytes.Buffer{}
// 	if _, err := io.Copy(buf, f); err != nil {
// 		return nil, err
// 	}
// 	sum, err := v.CalculateChecksum(f)
// 	if err != nil {
// 		return nil, err
// 	}
// 	id := uuid.New()
// 	ext := filepath.Ext(file.Filename)
// 	key := filepath.Join("originals", id.String(), "original"+ext)

// 	path, err := v.objStore.Put(ctx, id.String(), key, buf, int64(buf.Len()))
// 	if err != nil {
// 		return nil, err
// 	}

//		// save meta in db
//		vm := models.Video{
//			ID:                id,
//			Filename:          file.Filename,
//			OriginalObjectKey: path,
//			Status:            models.StatusUploaded,
//		}
//		if err := v.videoMetadataRepo.InsertBasic(ctx, vm); err != nil {
//			return nil, err
//		}
//		payload, err := json.Marshal(jobs.JobPayload{VideoID: id.String(), Step: jobs.StepValidate})
//		if err != nil {
//			return nil, err
//		}
//		if err := v.queue.Enqueue(ctx, v.queueName, payload); err != nil {
//			return nil, err
//		}
//		cacheKey := cache.GetKey(cache.KEY, path)
//		if err := v.cache.Set(ctx, cacheKey, path, 24*time.Hour); err != nil {
//			return nil, err
//		}
//		return &UploadResult{VideoId: id.String(), Key: path, Sha256: sum}, nil
//	}
func (v *VideoService) UploadPreSign(ctx context.Context, videoDto *dto.UploadVideoDto, user *models.User) (*UploadResult, error) {
	id := uuid.New()
	ext := filepath.Ext(videoDto.Filename)
	key := filepath.Join("originals", id.String(), "original"+ext)

	// save meta in db
	vm := &models.VideoMetadata{
		ID:                id, // unique id of the entry
		Filename:          videoDto.Filename,
		Size:              videoDto.Size,
		OriginalObjectKey: key,
	}
	if err := v.videoMetadataRepo.InsertBasic(ctx, vm); err != nil {
		return nil, err
	}
	video := &models.Video{
		ID:      uuid.New(),
		VideoId: vm.ID,
		UserID:  user.ID,
		Status:  models.StatusUploading,
		FileKey: key,
	}
	if err := v.videoRepo.InsertBasic(ctx, video); err != nil {
		return nil, err
	}

	url, err := v.objStore.GerPreSignedPutUrl(ctx, key, videoDto.Size)
	if err != nil {
		return nil, err
	}
	cacheKey := cache.GetKey(cache.KEY, key)
	if err := v.cache.Set(ctx, cacheKey, key, 24*time.Hour); err != nil {
		return nil, err
	}
	return &UploadResult{VideoId: id.String(), Key: key, Url: url}, nil
}
func (v *VideoService) GetVideo(ctx context.Context, id string) (*models.VideoMetadata, error) {
	return v.videoMetadataRepo.Get(ctx, id)
}
func (v *VideoService) GetDownloadUrl(ctx context.Context, key string) (string, error) {
	cacheKey := cache.GetKey(cache.URL, key)
	var cached string
	if err := v.cache.Get(ctx, cacheKey, &cached); err == nil && cached != "" {
		return cached, err
	}
	u, err := v.objStore.GetUrl(ctx, key)
	if err != nil {
		return "", err
	}
	return u, v.cache.Set(ctx, cacheKey, u, 24*time.Hour)
}
func (v *VideoService) DownloadVideo() string {
	return "video is downloaded"
}
func (v *VideoService) UpdateMeta(ctx context.Context, videoId string, sha string, dur int, vcodec, acodec string, w, h int) error {
	return v.videoMetadataRepo.UpdateMeta(ctx, videoId, sha, dur, vcodec, acodec, w, h)
}
func (v *VideoService) UpdateQualities(ctx context.Context, videoId string, qualities []string) error {
	return v.videoMetadataRepo.UpdateQualities(ctx, videoId, qualities)
}
func (v *VideoService) UpdateManifest(ctx context.Context, videoId string, manifest string) error {
	return v.videoMetadataRepo.UpdateManifest(ctx, videoId, manifest)
}
func (v *VideoService) UpdateThumbnail(ctx context.Context, videoId string, thumbnailKey string) error {
	return v.videoMetadataRepo.UpdateThumbnail(ctx, videoId, thumbnailKey)
}

//	func (v *VideoService) UpdateStatus(ctx context.Context, videoId string, status models.VideoStatus) error {
//		return v.videoMetadataRepo.UpdateStatus(ctx, videoId)
//	}
func (v *VideoService) GetTranscodingPath(id string, quality string, ext string) string {
	return filepath.Join("transcoded", id, fmt.Sprintf("%s%s", quality, ext))
}
func (v *VideoService) GetHlsDir(id string) string {
	return filepath.Join("segments", id)
}
func (v *VideoService) CalculateChecksum(f io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
