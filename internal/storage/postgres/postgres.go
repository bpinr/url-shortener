package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bpinr/url-shortener/internal/storage"
)

const queryTimeout = 2 * time.Second

const uniqueViolation = "23505"

const schema = `
CREATE TABLE IF NOT EXISTS url (
	id         BIGSERIAL PRIMARY KEY,
	alias      TEXT        NOT NULL UNIQUE,
	url        TEXT        NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`

type Storage struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Storage, error) {
	const fn = "storage.postgres.New"

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn, err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("%s: ping: %w", fn, err)
	}

	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("%s: create schema: %w", fn, err)
	}

	return &Storage{pool: pool}, nil
}

func (s *Storage) Close() {
	s.pool.Close()
}

func (s *Storage) SaveURL(ctx context.Context, urlToSave, alias string) (int64, error) {
	const fn = "storage.postgres.SaveURL"

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO url (url, alias) VALUES ($1, $2) RETURNING id`,
		urlToSave, alias,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return 0, fmt.Errorf("%s: %w", fn, storage.ErrAliasExists)
		}

		return 0, fmt.Errorf("%s: %w", fn, err)
	}

	return id, nil
}

func (s *Storage) GetURL(ctx context.Context, alias string) (string, error) {
	const fn = "storage.postgres.GetURL"

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var url string
	err := s.pool.QueryRow(ctx, `SELECT url FROM url WHERE alias = $1`, alias).Scan(&url)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("%s: %w", fn, storage.ErrURLNotFound)
		}

		return "", fmt.Errorf("%s: %w", fn, err)
	}

	return url, nil
}

func (s *Storage) UpdateURL(ctx context.Context, alias, newURL string) error {
	const fn = "storage.postgres.UpdateURL"

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	tag, err := s.pool.Exec(ctx, `UPDATE url SET url = $1 WHERE alias = $2`, newURL, alias)
	if err != nil {
		return fmt.Errorf("%s: %w", fn, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", fn, storage.ErrURLNotFound)
	}

	return nil
}

func (s *Storage) DeleteURL(ctx context.Context, alias string) error {
	const fn = "storage.postgres.DeleteURL"

	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	tag, err := s.pool.Exec(ctx, `DELETE FROM url WHERE alias = $1`, alias)
	if err != nil {
		return fmt.Errorf("%s: %w", fn, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", fn, storage.ErrURLNotFound)
	}

	return nil
}
