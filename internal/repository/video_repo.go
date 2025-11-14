package repository

import (
	"context"

	"github.com/ak-ansari/mytube/internal/models"
)

type VideoRepository interface {
	InsertBasic(ctx context.Context, vm *models.Video) error
}
