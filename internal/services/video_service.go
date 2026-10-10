package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"crypto/sha256"

	"github.com/ak-ansari/mytube/internal/api/dto"
	"github.com/ak-ansari/mytube/internal/cache"
	"github.com/ak-ansari/mytube/internal/index"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/queue"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/util"
	"github.com/google/uuid"
)

type UploadResult struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Sha256 string `json:"sha256"`
	Url    string `json:"url"`
	Stage  int    `json:"stage"`
	Status string `json:"status"`
}
type VideoService struct {
	objStore          storage.ObjectStore
	videoMetadataRepo repository.VideoMetadataRepository
	videoRepo         repository.VideoRepository
	queue             queue.Queue
	cache             cache.Cache
	queueName         string
	sm                *VideoStateMachine
	pc                *PipelineCoordinator
	videoIndex        index.VideoIndex
}

func NewVideoService(vi index.VideoIndex, objStore storage.ObjectStore, videoMetadataRepo repository.VideoMetadataRepository, videoRepo repository.VideoRepository, queue queue.Queue, cache cache.Cache, queueName string, sm *VideoStateMachine, pc *PipelineCoordinator) *VideoService {
	return &VideoService{
		objStore:          objStore,
		queueName:         queueName,
		queue:             queue,
		cache:             cache,
		videoMetadataRepo: videoMetadataRepo,
		videoRepo:         videoRepo,
		sm:                sm,
		pc:                pc,
		videoIndex:        vi,
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

func (v *VideoService) UploadPreSign(ctx context.Context, videoDto *dto.UploadVideoDto, user *models.User) (*UploadResult, error) {
	id := uuid.New()
	ext := filepath.Ext(videoDto.Filename)
	if ext == "" {
		return nil, fmt.Errorf("%w: filename should be provided with a valid extension. filename:%s", ErrBadRequest, videoDto.Filename)
	}
	key := filepath.Join(storage.DirectoryOriginals, id.String(), "original"+ext)
	// inserting available properties
	stage, err := v.sm.GetDefault()
	if err != nil {
		return nil, err
	}
	video := &models.Video{
		ID:      id,
		UserID:  user.ID,
		FileKey: key,
		Status:  stage.Name,
		Stage:   stage.Ordering,
	}
	if err := v.videoRepo.InsertBasic(ctx, video); err != nil {
		return nil, err
	}
	// save meta in db
	vm := &models.VideoMetadata{
		ID:                uuid.New(), // unique id of the entry
		Filename:          videoDto.Filename,
		Size:              videoDto.Size,
		OriginalObjectKey: key,
		VideoId:           id,
	}
	if err := v.videoMetadataRepo.InsertBasic(ctx, vm); err != nil {
		return nil, err
	}

	url, err := v.objStore.GerPreSignedPutUrl(ctx, key, videoDto.Size)
	if err != nil {
		return nil, err
	}
	cacheKey := cache.GetKey(cache.KEY, key)
	if err := v.cache.Set(ctx, cacheKey, id.String(), 24*time.Hour); err != nil {
		return nil, err
	}
	return &UploadResult{Key: key, Url: url, ID: video.ID.String()}, nil
}
func (v *VideoService) GetVideo(ctx context.Context, id string) (*models.VideoMetadata, error) {
	return v.videoMetadataRepo.Get(ctx, id)
}
func (v *VideoService) ConfirmVideo(ctx context.Context, id string, dto dto.VideoConfirmDto) (*models.Video, error) {
	parsedId, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid video id", ErrBadRequest)
	}
	title := strings.TrimSpace(dto.Title)
	if len(title) < 3 || len(title) > 100 {
		return nil, fmt.Errorf("%w: title must be 3-100 characters", ErrBadRequest)
	}
	if len(dto.Description) > 5000 {
		return nil, fmt.Errorf("%w: description must be at most 5000 characters", ErrBadRequest)
	}
	// "" keeps the auto-generated thumbnail (the card query falls back to video_metadata.thumbnail)
	if dto.Thumbnail != "" && !validThumbnailKey(id, dto.Thumbnail) {
		return nil, fmt.Errorf("%w: thumbnail must be uploaded for this video", ErrBadRequest)
	}
	current, err := v.videoRepo.Get(ctx, id)
	if err != nil {
		return nil, notFound(err)
	}
	if current.Stage > 3 || isFailed(current.Status) {
		return nil, fmt.Errorf("%w: video already confirmed", ErrBadRequest)
	}
	vm := &models.Video{
		ID:          parsedId,
		Thumbnail:   &dto.Thumbnail,
		Description: &dto.Description,
		Visibility:  &dto.Visibility,
		Title:       &title,
	}
	// Postgres first, so a failed write never leaves an orphan search doc
	if err := v.videoRepo.UpdateVideo(ctx, vm); err != nil {
		return nil, err
	}
	if err := v.videoIndex.InsertVideo(ctx, vm); err != nil {
		return nil, err
	}
	// Re-read after saving the title: if phase 1 already finished (stage 3 `valid` with the
	// thumbnail set) the worker is waiting on us; otherwise its own Advance at the phase
	// boundary will see the title. AdvanceState makes a simultaneous advance safe.
	current, err = v.videoRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	meta, err := v.videoMetadataRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Stage == 3 && current.Status == models.StatusValid && meta.Thumbnail != nil && *meta.Thumbnail != "" {
		if err := v.pc.Advance(ctx, id); err != nil {
			return nil, err
		}
	}
	vm, err = v.videoRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return vm, nil

}

func isFailed(status models.VideoStatus) bool {
	return status == models.StatusPhase1Failed || status == models.StatusPhase2Failed
}
func (v *VideoService) GetDownloadUrl(ctx context.Context, key string) (string, error) {
	return v.objStore.GetUrl(ctx, key)
}
func (v *VideoService) GetVideoByKey(ctx context.Context, key string) (string, error) {
	cacheKey := cache.GetKey(cache.KEY, key)
	var cached string
	if err := v.cache.Get(ctx, cacheKey, &cached); err == nil && cached != "" {
		return cached, nil
	}
	id, err := v.videoMetadataRepo.GetByKey(ctx, key)
	if err != nil {
		return "", err
	}
	return id, nil

}
func (v *VideoService) DownloadVideo() string {
	return "video is downloaded"
}
func (v *VideoService) UpdateMeta(ctx context.Context, videoId string, sha string, dur int, vcodec, acodec string, w, h int) error {
	return v.videoMetadataRepo.UpdateMeta(ctx, videoId, sha, dur, vcodec, acodec, w, h)
}
func (v *VideoService) GetMeta(ctx context.Context, videoId string) (*models.VideoMetadata, error) {
	return v.videoMetadataRepo.Get(ctx, videoId)
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

// GetTranscodingPath is where a transcoded rendition lives; renditions are always MP4.
func (v *VideoService) GetTranscodingPath(id string, quality string) string {
	return filepath.Join(storage.DirectoryTranscoded, id, quality+".mp4")
}
func (v *VideoService) GetHlsDir(id string) string {
	return filepath.Join(storage.DirectorySegments, id)
}
func (v *VideoService) CalculateChecksum(f io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func (v *VideoService) ParseBucketEvent(ctx context.Context, data string) (string, bool, error) {
	bucketEvent, err := v.objStore.ParseBucketEvent(data)
	if err != nil {
		return "", false, err
	}
	// process only if the file is uploaded in originals directory and event is created
	if !strings.HasPrefix(bucketEvent.ObjectKey, storage.DirectoryOriginals) || bucketEvent.Event != storage.BucketEventCreated {
		return "", false, nil
	}
	id, err := v.GetVideoByKey(ctx, bucketEvent.ObjectKey)
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}
func (v *VideoService) SearchVideo(ctx context.Context, query, pageStr, sizeStr string) ([]*models.Video, error) {
	page := util.ParseIntWithDefault(pageStr, 1)
	size := util.ParseIntWithDefault(sizeStr, 20)
	offset := (page - 1) * size
	ids, err := v.videoIndex.SearchVideo(ctx, query, []string{"title", "description"}, offset, size)
	if err != nil {
		return nil, err
	}
	public, err := v.publicVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	// keep the relevance order from ES
	videos := make([]*models.Video, 0, len(ids))
	for _, id := range ids {
		if vid, ok := public[id]; ok {
			videos = append(videos, vid)
		}
	}
	return videos, nil
}

// SuggestVideo returns completion texts, only from published public videos.
func (v *VideoService) SuggestVideo(ctx context.Context, query string) ([]string, error) {
	suggestions, err := v.videoIndex.SuggestVideos(ctx, query)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(suggestions))
	for _, sg := range suggestions {
		ids = append(ids, sg.ID)
	}
	public, err := v.publicVideos(ctx, ids)
	if err != nil {
		return nil, err
	}
	texts := []string{}
	for _, sg := range suggestions {
		if _, ok := public[sg.ID]; ok {
			texts = append(texts, sg.Text)
		}
	}
	return texts, nil
}

// publicVideos loads the given ids and keeps only published public videos, keyed by id.
func (v *VideoService) publicVideos(ctx context.Context, ids []string) (map[string]*models.Video, error) {
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, err := uuid.Parse(id); err == nil {
			valid = append(valid, id)
		}
	}
	videos, err := v.videoRepo.GetByIds(ctx, valid)
	if err != nil {
		return nil, err
	}
	public := make(map[string]*models.Video, len(videos))
	for _, vid := range videos {
		if vid.Status == models.StatusPublished && vid.Visibility != nil && *vid.Visibility == models.VisibilityPublic {
			public[vid.ID.String()] = vid
		}
	}
	return public, nil
}
