package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/google/uuid"
)

const directoryAvatars = "avatars"

// MaxImageUploadSize bounds presigned image uploads (thumbnails, avatars).
const MaxImageUploadSize = 10 << 20

// AvatarPrefix is the folder a user's avatar uploads live in.
func AvatarPrefix(userID uuid.UUID) string { return directoryAvatars + "/" + userID.String() + "/" }

// FileService hands out presigned URLs for the small images clients upload and display directly.
// Video files never go through it: originals use the upload presign, playback the HLS gateway.
type FileService struct {
	objStore storage.ObjectStore
	catalog  *VideoCatalogService
}

func NewFileService(objStore storage.ObjectStore, catalog *VideoCatalogService) *FileService {
	return &FileService{objStore: objStore, catalog: catalog}
}

// PresignImageUpload signs a PUT for an image under the caller's own avatar folder or under the
// thumbnail folder of a video the caller owns: `avatars/{userId}/{file}` or `thumbnails/{videoId}/{file}`.
func (s *FileService) PresignImageUpload(ctx context.Context, userID uuid.UUID, key string, size int64) (string, error) {
	if size <= 0 || size > MaxImageUploadSize {
		return "", fmt.Errorf("%w: size must be between 1 byte and %d MB", ErrBadRequest, MaxImageUploadSize>>20)
	}
	switch {
	case strings.HasPrefix(key, directoryAvatars+"/"):
		if !validImageKey(AvatarPrefix(userID), key) {
			return "", fmt.Errorf("%w: avatars must be images under %s", ErrBadRequest, AvatarPrefix(userID))
		}
	case strings.HasPrefix(key, storage.DirectoryThumbnails+"/"):
		videoID, ok := folderID(key)
		if !ok || !validThumbnailKey(videoID.String(), key) {
			return "", fmt.Errorf("%w: thumbnails must be images under thumbnails/{videoId}/", ErrBadRequest)
		}
		if err := s.catalog.AssertOwner(ctx, videoID, userID); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("%w: only thumbnail and avatar uploads can be presigned", ErrBadRequest)
	}
	return s.objStore.GerPreSignedPutUrl(ctx, key, size)
}

// DownloadURL signs a GET for a thumbnail or avatar image. Originals, renditions and segments are refused.
func (s *FileService) DownloadURL(ctx context.Context, key string) (string, error) {
	if !IsPublicImageKey(key) {
		return "", fmt.Errorf("%w: only thumbnail and avatar images can be fetched", ErrBadRequest)
	}
	return s.objStore.GetUrl(ctx, key)
}

// IsPublicImageKey reports whether key is a thumbnail or avatar image (`{thumbnails|avatars}/{uuid}/{file}`).
func IsPublicImageKey(key string) bool {
	id, ok := folderID(key)
	if !ok {
		return false
	}
	for _, dir := range []string{storage.DirectoryThumbnails, directoryAvatars} {
		if validImageKey(dir+"/"+id.String()+"/", key) {
			return true
		}
	}
	return false
}

// folderID parses the uuid folder of a `{dir}/{uuid}/{file}` key.
func folderID(key string) (uuid.UUID, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(parts[1])
	// uuid.Parse accepts braces/urn forms; require the canonical form the prefix checks compare against
	return id, err == nil && id.String() == parts[1]
}
