package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/DimaOshchepkov/url_shortener/internal/config"
	"github.com/DimaOshchepkov/url_shortener/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	db *pgxpool.Pool
}

func NewStorage(cfg *config.Config) (*Storage, error) {
	const op = "storage.postgres.NewStorage"

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		cfg.Storage.Host, cfg.Storage.Port, cfg.Storage.User, cfg.Storage.Password, cfg.Storage.Dbname)

	poolCfg, err := pgxpool.ParseConfig(psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	poolCfg.MaxConns = cfg.Storage.PoolMaxConns
	poolCfg.MinConns = cfg.Storage.PoolMinConns

	db, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &Storage{db: db}, nil
}

func (s *Storage) CloseStorage() {
	s.db.Close()
}

func (s *Storage) SaveURL(ctx context.Context, urlToSave string, alias string) error {
	const op = "storage.postgres.SaveURL"

	stmt := `INSERT INTO urls (url, alias) VALUES($1, $2)`
	_, err := s.db.Exec(ctx, stmt, urlToSave, alias)
	if err != nil {
		if IsDuplicatedKeyError(err) {
			return fmt.Errorf("%s: %w", op, storage.ErrURLExists)
		}
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *Storage) GetURL(ctx context.Context, alias string) (string, error) {
	const op = "storage.postgres.GetURL"

	stmt := `SELECT url FROM urls WHERE alias = $1`
	var resURL string
	err := s.db.QueryRow(ctx, stmt, alias).Scan(&resURL)
	if err != nil {
		if IsNotFoundError(err) {
			return "", fmt.Errorf("%s: %w", op, storage.ErrURLNotFound)
		}
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return resURL, nil
}

// IncrementClicks atomically increments the click counter for the given alias.
// This is called after a successful redirect to track per-link analytics.
func (s *Storage) IncrementClicks(ctx context.Context, alias string) error {
	return s.IncrementClicksBy(ctx, alias, 1)
}

// IncrementClicksBy atomically increments the click counter by the given delta.
// Used by ClickBatcher to flush batched counts.
func (s *Storage) IncrementClicksBy(ctx context.Context, alias string, delta int64) error {
	const op = "storage.postgres.IncrementClicksBy"

	stmt := `UPDATE urls SET clicks = clicks + $2 WHERE alias = $1`
	_, err := s.db.Exec(ctx, stmt, alias, delta)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *Storage) DeleteURL(ctx context.Context, alias string) error {
	const op = "storage.postgres.DeleteURL"

	stmt := `DELETE FROM urls WHERE alias = $1`
	res, err := s.db.Exec(ctx, stmt, alias)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	affect := res.RowsAffected()
	if affect == 0 {
		return fmt.Errorf("%s: %w", op, storage.ErrAliasNotFound)
	}
	return nil
}

func IsDuplicatedKeyError(err error) bool {
	var perr *pgconn.PgError
	if errors.As(err, &perr) {
		return perr.Code == "23505" // error code of duplicate
	}
	return false
}

func IsNotFoundError(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
