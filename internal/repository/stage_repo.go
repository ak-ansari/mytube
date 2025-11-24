package repository

import "context"

type StageRepo interface {
	GetAll(ctx context.Context)
}
