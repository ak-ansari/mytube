package services

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ak-ansari/mytube/internal/auth"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/google/uuid"
)

type UserService struct {
	repo     repository.UserRepository
	objStore storage.ObjectStore
}

func NewUserService(repo repository.UserRepository, objStore storage.ObjectStore) *UserService {
	return &UserService{repo: repo, objStore: objStore}
}

// SyncFromToken makes sure the Keycloak user has a row in users and returns it.
func (s *UserService) SyncFromToken(ctx context.Context, claims *auth.Claims) (*models.User, error) {
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("keycloak sub is not a uuid: %w", err)
	}
	userName := claims.PreferredUsername
	if userName == "" {
		userName = claims.Email
	}
	channelName := claims.Name
	if channelName == "" {
		channelName = userName
	}
	return s.repo.Upsert(ctx, &models.User{
		ID:          id,
		UserName:    userName,
		ChannelName: channelName,
		Email:       claims.Email,
	})
}

func (s *UserService) GetProfile(ctx context.Context, id uuid.UUID) (*models.Profile, error) {
	p, err := s.repo.GetProfile(ctx, id)
	if err != nil {
		return nil, notFound(err)
	}
	signChannel(ctx, s.objStore, &p.ChannelSummary)
	return p, nil
}

type UpdateProfileInput struct {
	ChannelName *string `json:"channel_name"`
	Description *string `json:"description"`
	Avatar      *string `json:"avatar"` // object key from /api/files/presign
}

func (s *UserService) UpdateProfile(ctx context.Context, id uuid.UUID, in UpdateProfileInput) (*models.Profile, error) {
	if in.ChannelName != nil {
		name := strings.TrimSpace(*in.ChannelName)
		if n := utf8.RuneCountInString(name); n < 3 || n > 60 {
			return nil, fmt.Errorf("%w: channel name must be 3-60 characters", ErrBadRequest)
		}
		in.ChannelName = &name
	}
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > 1000 {
		return nil, fmt.Errorf("%w: description must be at most 1000 characters", ErrBadRequest)
	}
	// "" clears the avatar; anything else must be an image the user uploaded under their own folder
	if in.Avatar != nil && *in.Avatar != "" && !validImageKey(AvatarPrefix(id), *in.Avatar) {
		return nil, fmt.Errorf("%w: avatar must be an image uploaded to your own avatar folder", ErrBadRequest)
	}
	if err := s.repo.UpdateProfile(ctx, id, in.ChannelName, in.Description, in.Avatar); err != nil {
		return nil, err
	}
	return s.GetProfile(ctx, id)
}

func (s *UserService) GetChannel(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (*models.Channel, error) {
	c, err := s.repo.GetChannel(ctx, id, viewerID)
	if err != nil {
		return nil, notFound(err)
	}
	signChannel(ctx, s.objStore, &c.ChannelSummary)
	return c, nil
}

func (s *UserService) Subscribe(ctx context.Context, subscriberID, channelID uuid.UUID) (*models.Channel, error) {
	if subscriberID == channelID {
		return nil, fmt.Errorf("%w: you cannot subscribe to your own channel", ErrBadRequest)
	}
	if _, err := s.GetChannel(ctx, channelID, nil); err != nil {
		return nil, err
	}
	if err := s.repo.Subscribe(ctx, subscriberID, channelID); err != nil {
		return nil, err
	}
	return s.GetChannel(ctx, channelID, &subscriberID)
}

func (s *UserService) Unsubscribe(ctx context.Context, subscriberID, channelID uuid.UUID) (*models.Channel, error) {
	if err := s.repo.Unsubscribe(ctx, subscriberID, channelID); err != nil {
		return nil, err
	}
	return s.GetChannel(ctx, channelID, &subscriberID)
}

func (s *UserService) ListSubscriptions(ctx context.Context, subscriberID uuid.UUID) ([]*models.Channel, error) {
	channels, err := s.repo.ListSubscriptions(ctx, subscriberID)
	if err != nil {
		return nil, err
	}
	for _, c := range channels {
		signChannel(ctx, s.objStore, &c.ChannelSummary)
	}
	return channels, nil
}

// signURL turns an object key into a presigned GET url ("" when there is no key).
func signURL(ctx context.Context, store storage.ObjectStore, key *string) string {
	if key == nil || *key == "" {
		return ""
	}
	url, err := store.GetUrl(ctx, *key)
	if err != nil {
		return ""
	}
	return url
}

func signChannel(ctx context.Context, store storage.ObjectStore, c *models.ChannelSummary) {
	c.AvatarURL = signURL(ctx, store, c.Avatar)
}
