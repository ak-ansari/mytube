package repository

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/google/uuid"
)

type UserRepository interface {
	// Upsert creates the user on first login and refreshes identity fields from Keycloak; profile fields are kept.
	Upsert(ctx context.Context, u *models.User) (*models.User, error)
	GetProfile(ctx context.Context, id uuid.UUID) (*models.Profile, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, channelName, description, avatar *string) error
	GetChannel(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (*models.Channel, error)
	Subscribe(ctx context.Context, subscriberID, channelID uuid.UUID) error
	Unsubscribe(ctx context.Context, subscriberID, channelID uuid.UUID) error
	ListSubscriptions(ctx context.Context, subscriberID uuid.UUID) ([]*models.Channel, error)
}
