package services

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/ak-ansari/mytube/internal/cache"
	"github.com/ak-ansari/mytube/internal/index"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/stream"
	"github.com/google/uuid"
)

const viewDedupWindow = 30 * time.Minute

// VideoCatalogService serves the public catalog (feed, search, watch, engagement) and the creator studio.
type VideoCatalogService struct {
	repo        repository.VideoCatalogRepository
	commentRepo repository.CommentRepository
	videoIndex  index.VideoIndex
	objStore    storage.ObjectStore
	cache       cache.Cache
	streams     *stream.Signer
}

func NewVideoCatalogService(repo repository.VideoCatalogRepository, commentRepo repository.CommentRepository, vi index.VideoIndex, objStore storage.ObjectStore, c cache.Cache, streams *stream.Signer) *VideoCatalogService {
	return &VideoCatalogService{repo: repo, commentRepo: commentRepo, videoIndex: vi, objStore: objStore, cache: c, streams: streams}
}

// Paging is a 1-based page with a bounded size.
type Paging struct {
	Page int
	Size int
}

func (p Paging) Offset() int { return (p.Page - 1) * p.Size }

func (s *VideoCatalogService) signCards(ctx context.Context, cards []*models.VideoCard) {
	for _, c := range cards {
		c.ThumbnailURL = signURL(ctx, s.objStore, c.Thumbnail)
		signChannel(ctx, s.objStore, &c.Channel)
	}
}

func (s *VideoCatalogService) listPublished(ctx context.Context, f repository.VideoFilter, p Paging) (models.Page[*models.VideoCard], error) {
	// fetch one extra row to know whether another page exists
	cards, err := s.repo.ListPublished(ctx, f, p.Offset(), p.Size+1)
	if err != nil {
		return models.Page[*models.VideoCard]{}, err
	}
	page := models.NewPage(cards, p.Page, p.Size)
	s.signCards(ctx, page.Items)
	return page, nil
}

// Browse returns the latest public videos, or search results when query is set.
func (s *VideoCatalogService) Browse(ctx context.Context, query string, p Paging) (models.Page[*models.VideoCard], error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return s.listPublished(ctx, repository.VideoFilter{}, p)
	}
	ids, err := s.videoIndex.SearchVideo(ctx, query, []string{"title", "description"}, p.Offset(), p.Size+1)
	if err != nil {
		return models.Page[*models.VideoCard]{}, err
	}
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if u, err := uuid.Parse(id); err == nil {
			parsed = append(parsed, u)
		}
	}
	// paging already happened in ES; Postgres only filters out unpublished/private hits
	cards, err := s.repo.ListPublished(ctx, repository.VideoFilter{IDs: parsed}, 0, len(parsed))
	if err != nil {
		return models.Page[*models.VideoCard]{}, err
	}
	hasMore := len(ids) > p.Size
	if len(cards) > p.Size {
		cards = cards[:p.Size]
	}
	s.signCards(ctx, cards)
	return models.Page[*models.VideoCard]{Items: cards, Page: p.Page, Size: p.Size, HasMore: hasMore}, nil
}

func (s *VideoCatalogService) ChannelVideos(ctx context.Context, channelID uuid.UUID, p Paging) (models.Page[*models.VideoCard], error) {
	return s.listPublished(ctx, repository.VideoFilter{ChannelID: &channelID}, p)
}

func (s *VideoCatalogService) SubscriptionFeed(ctx context.Context, userID uuid.UUID, p Paging) (models.Page[*models.VideoCard], error) {
	return s.listPublished(ctx, repository.VideoFilter{SubscriberID: &userID}, p)
}

func (s *VideoCatalogService) LikedVideos(ctx context.Context, userID uuid.UUID, p Paging) (models.Page[*models.VideoCard], error) {
	return s.listPublished(ctx, repository.VideoFilter{LikedBy: &userID}, p)
}

func (s *VideoCatalogService) Related(ctx context.Context, id uuid.UUID, p Paging) (models.Page[*models.VideoCard], error) {
	return s.listPublished(ctx, repository.VideoFilter{ExcludeID: &id}, p)
}

// MyVideos lists all of a creator's videos, including ones still processing or private.
func (s *VideoCatalogService) MyVideos(ctx context.Context, ownerID uuid.UUID, p Paging) (models.Page[*models.VideoCard], error) {
	cards, err := s.repo.ListByOwner(ctx, ownerID, p.Offset(), p.Size+1)
	if err != nil {
		return models.Page[*models.VideoCard]{}, err
	}
	page := models.NewPage(cards, p.Page, p.Size)
	s.signCards(ctx, page.Items)
	return page, nil
}

// GetDetails returns a video if the viewer may watch it: published public/unlisted videos for everyone, anything for the owner.
func (s *VideoCatalogService) GetDetails(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (*models.VideoDetails, error) {
	d, err := s.repo.GetDetails(ctx, id, viewerID)
	if err != nil {
		return nil, notFound(err)
	}
	isOwner := viewerID != nil && *viewerID == d.Channel.ID
	if !isOwner {
		if d.Status != models.StatusPublished || d.Visibility == nil || *d.Visibility == models.VisibilityPrivate {
			return nil, ErrNotFound
		}
	}
	d.ThumbnailURL = signURL(ctx, s.objStore, d.Thumbnail)
	if d.ManifestPath != nil && *d.ManifestPath != "" {
		// segments are private; playback goes through the token-checked nginx gateway
		d.ManifestURL = s.streams.PlaybackURL(d.ID.String(), path.Base(*d.ManifestPath))
	}
	signChannel(ctx, s.objStore, &d.Channel)
	return d, nil
}

// AssertOwner returns ErrNotFound / ErrForbidden unless userID uploaded the video.
func (s *VideoCatalogService) AssertOwner(ctx context.Context, videoID, userID uuid.UUID) error {
	owner, err := s.repo.GetOwnerID(ctx, videoID)
	if err != nil {
		return notFound(err)
	}
	if owner != userID {
		return ErrForbidden
	}
	return nil
}

type UpdateVideoInput struct {
	Title       *string                 `json:"title"`
	Description *string                 `json:"description"`
	Visibility  *models.VideoVisibility `json:"visibility"`
	Thumbnail   *string                 `json:"thumbnail"` // object key from /api/files/presign
}

func (s *VideoCatalogService) Update(ctx context.Context, id, userID uuid.UUID, in UpdateVideoInput) (*models.VideoDetails, error) {
	if err := s.AssertOwner(ctx, id, userID); err != nil {
		return nil, err
	}
	current, err := s.repo.GetDetails(ctx, id, &userID)
	if err != nil {
		return nil, notFound(err)
	}
	if current.Title == nil {
		return nil, fmt.Errorf("%w: finish the upload (confirm) before editing", ErrBadRequest)
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if len(t) < 3 || len(t) > 100 {
			return nil, fmt.Errorf("%w: title must be 3-100 characters", ErrBadRequest)
		}
		in.Title = &t
	}
	if in.Description != nil && len(*in.Description) > 5000 {
		return nil, fmt.Errorf("%w: description must be at most 5000 characters", ErrBadRequest)
	}
	if in.Visibility != nil {
		switch *in.Visibility {
		case models.VisibilityPublic, models.VisibilityPrivate, models.VisibilityUnlisted:
		default:
			return nil, fmt.Errorf("%w: visibility must be public, private or unlisted", ErrBadRequest)
		}
	}
	if err := s.repo.UpdateDetails(ctx, &models.Video{
		ID: id, Title: in.Title, Description: in.Description, Visibility: in.Visibility, Thumbnail: in.Thumbnail,
	}); err != nil {
		return nil, err
	}
	updated, err := s.GetDetails(ctx, id, &userID)
	if err != nil {
		return nil, err
	}
	description := ""
	if updated.Description != nil {
		description = *updated.Description
	}
	if err := s.videoIndex.UpdateVideo(ctx, &models.Video{ID: id, Title: updated.Title, Description: &description}); err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete removes the video row (metadata, reactions and comments cascade), its search doc and all stored files.
func (s *VideoCatalogService) Delete(ctx context.Context, id, userID uuid.UUID) error {
	if err := s.AssertOwner(ctx, id, userID); err != nil {
		return err
	}
	d, err := s.repo.GetDetails(ctx, id, &userID)
	if err != nil {
		return notFound(err)
	}
	if err := s.videoIndex.DeleteVideo(ctx, id.String()); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	for _, dir := range []string{storage.DirectoryOriginals, storage.DirectoryThumbnails, storage.DirectoryTranscoded, storage.DirectorySegments} {
		if err := s.objStore.DeletePrefix(ctx, path.Join(dir, id.String())+"/"); err != nil {
			return err
		}
	}
	// custom thumbnails uploaded outside the per-video folders
	if d.Thumbnail != nil && !strings.HasPrefix(*d.Thumbnail, storage.DirectoryThumbnails+"/"+id.String()) {
		_ = s.objStore.Delete(ctx, *d.Thumbnail)
	}
	return nil
}

// RecordView counts at most one view per viewer per video every viewDedupWindow.
func (s *VideoCatalogService) RecordView(ctx context.Context, id uuid.UUID, viewerKey string) error {
	key := cache.GetKey("view", id.String()+":"+viewerKey)
	first, err := s.cache.SetNX(ctx, key, "1", viewDedupWindow)
	if err != nil || !first {
		return err
	}
	return s.repo.IncrementViews(ctx, id)
}

// React sets the user's like (1), dislike (-1) or clears it (0).
func (s *VideoCatalogService) React(ctx context.Context, id, userID uuid.UUID, value int) (*models.VideoDetails, error) {
	if value < -1 || value > 1 {
		return nil, fmt.Errorf("%w: value must be 1, -1 or 0", ErrBadRequest)
	}
	if _, err := s.GetDetails(ctx, id, &userID); err != nil {
		return nil, err
	}
	if err := s.repo.SetReaction(ctx, id, userID, value); err != nil {
		return nil, err
	}
	return s.GetDetails(ctx, id, &userID)
}

func (s *VideoCatalogService) ListComments(ctx context.Context, videoID uuid.UUID, viewerID *uuid.UUID, p Paging) (models.Page[*models.Comment], error) {
	if _, err := s.GetDetails(ctx, videoID, viewerID); err != nil {
		return models.Page[*models.Comment]{}, err
	}
	comments, err := s.commentRepo.List(ctx, videoID, p.Offset(), p.Size+1)
	if err != nil {
		return models.Page[*models.Comment]{}, err
	}
	page := models.NewPage(comments, p.Page, p.Size)
	for _, c := range page.Items {
		signChannel(ctx, s.objStore, &c.Author)
	}
	return page, nil
}

func (s *VideoCatalogService) AddComment(ctx context.Context, videoID, userID uuid.UUID, content string) (*models.Comment, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 2000 {
		return nil, fmt.Errorf("%w: comment must be 1-2000 characters", ErrBadRequest)
	}
	if _, err := s.GetDetails(ctx, videoID, &userID); err != nil {
		return nil, err
	}
	id := uuid.New()
	if err := s.commentRepo.Insert(ctx, id, videoID, userID, content); err != nil {
		return nil, err
	}
	c, err := s.commentRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	signChannel(ctx, s.objStore, &c.Author)
	return c, nil
}

// DeleteComment is allowed for the comment author and the video owner.
func (s *VideoCatalogService) DeleteComment(ctx context.Context, videoID, commentID, userID uuid.UUID) error {
	c, err := s.commentRepo.Get(ctx, commentID)
	if err != nil {
		return notFound(err)
	}
	if c.VideoID != videoID {
		return ErrNotFound
	}
	if c.Author.ID != userID {
		if err := s.AssertOwner(ctx, videoID, userID); err != nil {
			return ErrForbidden
		}
	}
	return s.commentRepo.Delete(ctx, commentID)
}
