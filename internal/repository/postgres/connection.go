package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dtroode/gophkeeper-server/database"
)

type Connection struct {
	*pgxpool.Pool
}

func NewConection(ctx context.Context, dsn string) (*Connection, error) {
	conf, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres dsn: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection pool: %w", err)
	}

	if err := database.Migrate(ctx, dsn); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return &Connection{
		Pool: pool,
	}, nil
}

func (s *Connection) Close() error {
	if s.Pool != nil {
		s.Pool.Close()
	}
	return nil
}

func (s *Connection) Ping(ctx context.Context) error {
	if s.Pool == nil {
		return fmt.Errorf("connection pool is nil")
	}
	return s.Pool.Ping(ctx)
}
