package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open creates a PostgreSQL connection pool and verifies connectivity.
func Open(ctx context.Context, dsn string) (*Store, error) {
	if dsn == "" {
		return nil, errors.New("postgres dsn is empty")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close releases the PostgreSQL connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// Migrate applies the built-in schema needed by the monitor.
func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, migrationSQL)
	return err
}
