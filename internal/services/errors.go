package services

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrBadRequest = errors.New("bad request")
)

// notFound maps pgx's no-rows error to ErrNotFound so handlers can return 404.
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
