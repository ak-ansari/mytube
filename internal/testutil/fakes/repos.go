package fakes

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ---------------------------------------------------------------- VideoRepository

// VideoRepo is an in-memory repository.VideoRepository (the pipeline's videos table).
type VideoRepo struct {
	Errs   Errs
	Calls  Calls
	mu     sync.Mutex
	Videos map[string]*models.Video
}

var _ repository.VideoRepository = (*VideoRepo)(nil)

func NewVideoRepo() *VideoRepo { return &VideoRepo{Videos: map[string]*models.Video{}} }

// Put seeds (or replaces) a video.
func (r *VideoRepo) Put(v *models.Video) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *v
	r.Videos[v.ID.String()] = &cp
}

func (r *VideoRepo) InsertBasic(_ context.Context, v *models.Video) error {
	r.Calls.add("InsertBasic")
	if err := r.Errs.get("InsertBasic"); err != nil {
		return err
	}
	r.Put(v)
	return nil
}

func (r *VideoRepo) UpdateVideo(_ context.Context, v *models.Video) error {
	r.Calls.add("UpdateVideo")
	if err := r.Errs.get("UpdateVideo"); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.Videos[v.ID.String()]
	if !ok {
		return pgx.ErrNoRows
	}
	cur.Title, cur.Description, cur.Visibility, cur.Thumbnail = v.Title, v.Description, v.Visibility, v.Thumbnail
	return nil
}

func (r *VideoRepo) Get(_ context.Context, id string) (*models.Video, error) {
	r.Calls.add("Get")
	if err := r.Errs.get("Get"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.Videos[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	cp := *v
	return &cp, nil
}

func (r *VideoRepo) GetByIds(_ context.Context, ids []string) ([]*models.Video, error) {
	r.Calls.add("GetByIds")
	if err := r.Errs.get("GetByIds"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*models.Video
	for _, id := range ids {
		if v, ok := r.Videos[id]; ok {
			cp := *v
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *VideoRepo) UpdateState(_ context.Context, id string, stage int, status models.VideoStatus) error {
	r.Calls.add("UpdateState")
	if err := r.Errs.get("UpdateState"); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.Videos[id]
	if !ok {
		return pgx.ErrNoRows
	}
	v.Stage, v.Status = stage, status
	return nil
}

func (r *VideoRepo) AdvanceState(_ context.Context, id string, from, to int, status models.VideoStatus) (bool, error) {
	r.Calls.add("AdvanceState")
	if err := r.Errs.get("AdvanceState"); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.Videos[id]
	if !ok || v.Stage != from || v.Status == models.StatusPhase1Failed || v.Status == models.StatusPhase2Failed {
		return false, nil
	}
	v.Stage, v.Status = to, status
	return true, nil
}

// ---------------------------------------------------------------- VideoMetadataRepository

// VideoMetadataRepo is an in-memory repository.VideoMetadataRepository keyed by video id.
// GetByKey returns the video id, like the Postgres impl.
type VideoMetadataRepo struct {
	Errs  Errs
	Calls Calls
	mu    sync.Mutex
	Metas map[string]*models.VideoMetadata
}

var _ repository.VideoMetadataRepository = (*VideoMetadataRepo)(nil)

func NewVideoMetadataRepo() *VideoMetadataRepo {
	return &VideoMetadataRepo{Metas: map[string]*models.VideoMetadata{}}
}

func (r *VideoMetadataRepo) Put(m *models.VideoMetadata) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *m
	r.Metas[m.VideoId.String()] = &cp
}

func (r *VideoMetadataRepo) with(method, id string, fn func(m *models.VideoMetadata)) error {
	r.Calls.add(method)
	if err := r.Errs.get(method); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.Metas[id]
	if !ok {
		return pgx.ErrNoRows
	}
	fn(m)
	return nil
}

func (r *VideoMetadataRepo) InsertBasic(_ context.Context, m *models.VideoMetadata) error {
	r.Calls.add("InsertBasic")
	if err := r.Errs.get("InsertBasic"); err != nil {
		return err
	}
	r.Put(m)
	return nil
}

func (r *VideoMetadataRepo) UpdateMeta(_ context.Context, id, sha string, dur int, vc, ac string, w, h int) error {
	return r.with("UpdateMeta", id, func(m *models.VideoMetadata) {
		m.SHA256, m.DurationSeconds, m.CodecVideo, m.CodecAudio, m.Width, m.Height = &sha, &dur, &vc, &ac, &w, &h
	})
}

func (r *VideoMetadataRepo) UpdateQualities(_ context.Context, id string, q []string) error {
	return r.with("UpdateQualities", id, func(m *models.VideoMetadata) { m.AvailableQualities = q })
}

func (r *VideoMetadataRepo) UpdateManifest(_ context.Context, id, manifest string) error {
	return r.with("UpdateManifest", id, func(m *models.VideoMetadata) { m.ManifestPath = &manifest })
}

func (r *VideoMetadataRepo) UpdateThumbnail(_ context.Context, id, key string) error {
	return r.with("UpdateThumbnail", id, func(m *models.VideoMetadata) { m.Thumbnail = &key })
}

func (r *VideoMetadataRepo) Get(_ context.Context, id string) (*models.VideoMetadata, error) {
	var out *models.VideoMetadata
	err := r.with("Get", id, func(m *models.VideoMetadata) { cp := *m; out = &cp })
	return out, err
}

func (r *VideoMetadataRepo) GetByKey(_ context.Context, key string) (string, error) {
	r.Calls.add("GetByKey")
	if err := r.Errs.get("GetByKey"); err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, m := range r.Metas {
		if m.OriginalObjectKey == key {
			return id, nil
		}
	}
	return "", pgx.ErrNoRows
}

// ---------------------------------------------------------------- UserRepository

type userRow struct {
	user      models.User
	createdAt time.Time
}

// UserRepo is an in-memory repository.UserRepository. Subscriptions are shared with CatalogRepo
// through Subs (subscriber -> channel -> true).
type UserRepo struct {
	Errs  Errs
	Calls Calls
	mu    sync.Mutex
	users map[uuid.UUID]*userRow
	Subs  map[uuid.UUID]map[uuid.UUID]bool
}

var _ repository.UserRepository = (*UserRepo)(nil)

func NewUserRepo() *UserRepo {
	return &UserRepo{users: map[uuid.UUID]*userRow{}, Subs: map[uuid.UUID]map[uuid.UUID]bool{}}
}

// Put seeds (or replaces) a user.
func (r *UserRepo) Put(u models.User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[u.ID] = &userRow{user: u, createdAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

// User returns a copy of a stored user.
func (r *UserRepo) User(id uuid.UUID) (models.User, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.users[id]
	if !ok {
		return models.User{}, false
	}
	return row.user, true
}

func (r *UserRepo) Upsert(_ context.Context, u *models.User) (*models.User, error) {
	r.Calls.add("Upsert")
	if err := r.Errs.get("Upsert"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.users[u.ID]
	if !ok {
		row = &userRow{user: *u, createdAt: time.Now()}
		if row.user.Status == "" {
			row.user.Status = models.Active
		}
		r.users[u.ID] = row
	} else {
		// identity fields refresh, profile fields (channel name, description, avatar) are kept
		row.user.UserName, row.user.Email = u.UserName, u.Email
	}
	cp := row.user
	return &cp, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (r *UserRepo) channelLocked(id uuid.UUID, viewer *uuid.UUID) (*models.Channel, bool) {
	row, ok := r.users[id]
	if !ok {
		return nil, false
	}
	var subs int64
	for _, chans := range r.Subs {
		if chans[id] {
			subs++
		}
	}
	return &models.Channel{
		ChannelSummary: models.ChannelSummary{
			ID: id, UserName: row.user.UserName, ChannelName: strPtr(row.user.ChannelName), Avatar: strPtr(row.user.Avatar),
		},
		Description:  strPtr(row.user.Description),
		Subscribers:  subs,
		IsSubscribed: viewer != nil && r.Subs[*viewer][id],
		CreatedAt:    row.createdAt,
	}, true
}

func (r *UserRepo) GetProfile(_ context.Context, id uuid.UUID) (*models.Profile, error) {
	r.Calls.add("GetProfile")
	if err := r.Errs.get("GetProfile"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.channelLocked(id, nil)
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return &models.Profile{Channel: *ch, Email: strPtr(r.users[id].user.Email)}, nil
}

func (r *UserRepo) UpdateProfile(_ context.Context, id uuid.UUID, channelName, description, avatar *string) error {
	r.Calls.add("UpdateProfile")
	if err := r.Errs.get("UpdateProfile"); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.users[id]
	if !ok {
		return pgx.ErrNoRows
	}
	if channelName != nil {
		row.user.ChannelName = *channelName
	}
	if description != nil {
		row.user.Description = *description
	}
	if avatar != nil {
		row.user.Avatar = *avatar
	}
	return nil
}

func (r *UserRepo) GetChannel(_ context.Context, id uuid.UUID, viewer *uuid.UUID) (*models.Channel, error) {
	r.Calls.add("GetChannel")
	if err := r.Errs.get("GetChannel"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.channelLocked(id, viewer)
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return ch, nil
}

func (r *UserRepo) Subscribe(_ context.Context, sub, ch uuid.UUID) error {
	r.Calls.add("Subscribe")
	if err := r.Errs.get("Subscribe"); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Subs[sub] == nil {
		r.Subs[sub] = map[uuid.UUID]bool{}
	}
	r.Subs[sub][ch] = true
	return nil
}

func (r *UserRepo) Unsubscribe(_ context.Context, sub, ch uuid.UUID) error {
	r.Calls.add("Unsubscribe")
	if err := r.Errs.get("Unsubscribe"); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Subs[sub], ch)
	return nil
}

func (r *UserRepo) ListSubscriptions(_ context.Context, sub uuid.UUID) ([]*models.Channel, error) {
	r.Calls.add("ListSubscriptions")
	if err := r.Errs.get("ListSubscriptions"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]uuid.UUID, 0, len(r.Subs[sub]))
	for id := range r.Subs[sub] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	out := []*models.Channel{}
	for _, id := range ids {
		if ch, ok := r.channelLocked(id, &sub); ok {
			out = append(out, ch)
		}
	}
	return out, nil
}

// IsSubscribed reports whether sub follows ch.
func (r *UserRepo) IsSubscribed(sub, ch uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Subs[sub][ch]
}

// ---------------------------------------------------------------- VideoCatalogRepository

type reactionKey struct{ video, user uuid.UUID }

// CatalogRepo is an in-memory repository.VideoCatalogRepository. Seed it with Put; lists are
// newest-first by CreatedAt. "Published" means status published and visibility public, like
// publishedVideoCond. Users (optional) supplies subscriptions for the SubscriberID filter.
type CatalogRepo struct {
	Errs      Errs
	Calls     Calls
	Users     *UserRepo
	mu        sync.Mutex
	videos    map[uuid.UUID]*models.VideoDetails
	reactions map[reactionKey]int
	// LastFilter/LastOffset/LastLimit record the last ListPublished/ListByOwner call.
	LastFilter            repository.VideoFilter
	LastOffset, LastLimit int
}

var _ repository.VideoCatalogRepository = (*CatalogRepo)(nil)

func NewCatalogRepo(users *UserRepo) *CatalogRepo {
	return &CatalogRepo{Users: users, videos: map[uuid.UUID]*models.VideoDetails{}, reactions: map[reactionKey]int{}}
}

// Put seeds (or replaces) a video.
func (r *CatalogRepo) Put(d *models.VideoDetails) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *d
	r.videos[d.ID] = &cp
}

// Video returns a copy of a stored video.
func (r *CatalogRepo) Video(id uuid.UUID) (*models.VideoDetails, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.videos[id]
	if !ok {
		return nil, false
	}
	cp := *d
	return &cp, true
}

// Reaction returns the stored reaction (0 when none).
func (r *CatalogRepo) Reaction(video, user uuid.UUID) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reactions[reactionKey{video, user}]
}

func isPublic(d *models.VideoDetails) bool {
	return d.Status == models.StatusPublished && d.Visibility != nil && *d.Visibility == models.VisibilityPublic
}

func (r *CatalogRepo) sortedLocked() []*models.VideoDetails {
	all := make([]*models.VideoDetails, 0, len(r.videos))
	for _, d := range r.videos {
		all = append(all, d)
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].CreatedAt.After(all[j].CreatedAt)
		}
		return all[i].ID.String() < all[j].ID.String()
	})
	return all
}

func window[T any](items []T, offset, limit int) []T {
	if offset >= len(items) {
		return []T{}
	}
	items = items[offset:]
	if limit < len(items) {
		items = items[:limit]
	}
	return items
}

func card(d *models.VideoDetails) *models.VideoCard {
	c := d.VideoCard
	return &c
}

func (r *CatalogRepo) ListPublished(_ context.Context, f repository.VideoFilter, offset, limit int) ([]*models.VideoCard, error) {
	r.Calls.add("ListPublished")
	if err := r.Errs.get("ListPublished"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.LastFilter, r.LastOffset, r.LastLimit = f, offset, limit
	var src []*models.VideoDetails
	if f.IDs != nil {
		for _, id := range f.IDs {
			if d, ok := r.videos[id]; ok {
				src = append(src, d)
			}
		}
	} else {
		src = r.sortedLocked()
	}
	out := []*models.VideoCard{}
	for _, d := range src {
		if !isPublic(d) {
			continue
		}
		if f.ChannelID != nil && d.Channel.ID != *f.ChannelID {
			continue
		}
		if f.ExcludeID != nil && d.ID == *f.ExcludeID {
			continue
		}
		if f.LikedBy != nil && r.reactions[reactionKey{d.ID, *f.LikedBy}] != 1 {
			continue
		}
		if f.SubscriberID != nil && (r.Users == nil || !r.Users.IsSubscribed(*f.SubscriberID, d.Channel.ID)) {
			continue
		}
		out = append(out, card(d))
	}
	return window(out, offset, limit), nil
}

func (r *CatalogRepo) ListByOwner(_ context.Context, owner uuid.UUID, offset, limit int) ([]*models.VideoCard, error) {
	r.Calls.add("ListByOwner")
	if err := r.Errs.get("ListByOwner"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.LastOffset, r.LastLimit = offset, limit
	out := []*models.VideoCard{}
	for _, d := range r.sortedLocked() {
		if d.Channel.ID == owner {
			out = append(out, card(d))
		}
	}
	return window(out, offset, limit), nil
}

func (r *CatalogRepo) GetDetails(_ context.Context, id uuid.UUID, viewer *uuid.UUID) (*models.VideoDetails, error) {
	r.Calls.add("GetDetails")
	if err := r.Errs.get("GetDetails"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.videos[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	cp := *d
	cp.Likes, cp.Dislikes, cp.MyReaction = 0, 0, 0
	for k, v := range r.reactions {
		if k.video != id {
			continue
		}
		if v == 1 {
			cp.Likes++
		} else if v == -1 {
			cp.Dislikes++
		}
		if viewer != nil && k.user == *viewer {
			cp.MyReaction = v
		}
	}
	if viewer != nil && r.Users != nil {
		cp.IsSubscribed = r.Users.IsSubscribed(*viewer, d.Channel.ID)
	}
	return &cp, nil
}

func (r *CatalogRepo) GetOwnerID(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	r.Calls.add("GetOwnerID")
	if err := r.Errs.get("GetOwnerID"); err != nil {
		return uuid.Nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.videos[id]
	if !ok {
		return uuid.Nil, pgx.ErrNoRows
	}
	return d.Channel.ID, nil
}

func (r *CatalogRepo) UpdateDetails(_ context.Context, v *models.Video) error {
	r.Calls.add("UpdateDetails")
	if err := r.Errs.get("UpdateDetails"); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.videos[v.ID]
	if !ok {
		return pgx.ErrNoRows
	}
	if v.Title != nil {
		d.Title = v.Title
	}
	if v.Description != nil {
		d.Description = v.Description
	}
	if v.Visibility != nil {
		d.Visibility = v.Visibility
	}
	if v.Thumbnail != nil {
		d.Thumbnail = v.Thumbnail
	}
	return nil
}

func (r *CatalogRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.Calls.add("Delete")
	if err := r.Errs.get("Delete"); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.videos, id)
	return nil
}

func (r *CatalogRepo) IncrementViews(_ context.Context, id uuid.UUID) error {
	r.Calls.add("IncrementViews")
	if err := r.Errs.get("IncrementViews"); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if d, ok := r.videos[id]; ok {
		d.Views++
	}
	return nil
}

func (r *CatalogRepo) SetReaction(_ context.Context, video, user uuid.UUID, value int) error {
	r.Calls.add("SetReaction")
	if err := r.Errs.get("SetReaction"); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if value == 0 {
		delete(r.reactions, reactionKey{video, user})
	} else {
		r.reactions[reactionKey{video, user}] = value
	}
	return nil
}

// ---------------------------------------------------------------- CommentRepository

// CommentRepo is an in-memory repository.CommentRepository; List is newest-first.
// Users (optional) fills the author summary on Insert.
type CommentRepo struct {
	Errs     Errs
	Calls    Calls
	Users    *UserRepo
	mu       sync.Mutex
	comments map[uuid.UUID]*models.Comment
	seq      int
}

var _ repository.CommentRepository = (*CommentRepo)(nil)

func NewCommentRepo(users *UserRepo) *CommentRepo {
	return &CommentRepo{Users: users, comments: map[uuid.UUID]*models.Comment{}}
}

// Put seeds a comment.
func (r *CommentRepo) Put(c *models.Comment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	r.comments[c.ID] = &cp
}

// Has reports whether a comment exists.
func (r *CommentRepo) Has(id uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.comments[id]
	return ok
}

func (r *CommentRepo) List(_ context.Context, video uuid.UUID, offset, limit int) ([]*models.Comment, error) {
	r.Calls.add("List")
	if err := r.Errs.get("List"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*models.Comment{}
	for _, c := range r.comments {
		if c.VideoID == video {
			cp := *c
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID.String() < out[j].ID.String()
	})
	return window(out, offset, limit), nil
}

func (r *CommentRepo) Get(_ context.Context, id uuid.UUID) (*models.Comment, error) {
	r.Calls.add("Get")
	if err := r.Errs.get("Get"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.comments[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	cp := *c
	return &cp, nil
}

func (r *CommentRepo) Insert(_ context.Context, id, video, user uuid.UUID, content string) error {
	r.Calls.add("Insert")
	if err := r.Errs.get("Insert"); err != nil {
		return err
	}
	author := models.ChannelSummary{ID: user}
	if r.Users != nil {
		if u, ok := r.Users.User(user); ok {
			author.UserName, author.ChannelName, author.Avatar = u.UserName, strPtr(u.ChannelName), strPtr(u.Avatar)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	now := time.Date(2026, 1, 1, 0, 0, r.seq, 0, time.UTC)
	r.comments[id] = &models.Comment{ID: id, VideoID: video, Content: content, CreatedAt: now, UpdatedAt: now, Author: author}
	return nil
}

func (r *CommentRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.Calls.add("Delete")
	if err := r.Errs.get("Delete"); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.comments, id)
	return nil
}
