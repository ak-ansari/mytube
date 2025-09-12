package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func NewPool(conf *config.Config, l logger.Logger) (*pgxpool.Pool, error) {
	dbConf := conf.DB
	pgDns := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbConf.PgUser, dbConf.PgPASS, dbConf.PgHost, dbConf.PgPort, dbConf.PgDBName)
	db, err := sql.Open("pgx", pgDns)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return nil, err
	}
	dir := "./migrations"

	if err := goose.RunContext(context.Background(), "up", db, dir); err != nil {
		return nil, err
	}
	l.Success("migration command executed successfully")
	return pgxpool.New(context.Background(), pgDns)
}
