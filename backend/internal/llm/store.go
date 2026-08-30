package llm

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a user has no LLM configuration saved.
var ErrNotFound = errors.New("LLM configuration not found")

// Config is a row from the llm_configs table (never carries the decrypted API key).
type Config struct {
	ID        string
	UserID    string
	Provider  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store persists and retrieves LLM configurations from PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Upsert creates or replaces the calling user's LLM configuration with an already-encrypted API key.
func (s *Store) Upsert(ctx context.Context, userID string, provider string, encryptedAPIKey []byte) (Config, error) {
	var saved Config
	err := s.pool.QueryRow(ctx,
		`INSERT INTO llm_configs (user_id, provider, encrypted_api_key)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE
		 SET provider = EXCLUDED.provider, encrypted_api_key = EXCLUDED.encrypted_api_key, updated_at = now()
		 RETURNING id, user_id, provider, created_at, updated_at`,
		userID, provider, encryptedAPIKey,
	).Scan(&saved.ID, &saved.UserID, &saved.Provider, &saved.CreatedAt, &saved.UpdatedAt)
	if err != nil {
		return Config{}, err
	}
	return saved, nil
}

// Get returns the calling user's LLM configuration. It returns ErrNotFound if none is saved.
func (s *Store) Get(ctx context.Context, userID string) (Config, error) {
	var cfg Config
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, provider, created_at, updated_at FROM llm_configs WHERE user_id = $1`, userID,
	).Scan(&cfg.ID, &cfg.UserID, &cfg.Provider, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Config{}, ErrNotFound
		}
		return Config{}, err
	}
	return cfg, nil
}

// GetWithKey returns the calling user's LLM configuration along with its
// still-encrypted API key, for the code paths that actually have to call the
// provider. It returns ErrNotFound if none is saved.
func (s *Store) GetWithKey(ctx context.Context, userID string) (Config, []byte, error) {
	var cfg Config
	var encryptedAPIKey []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, provider, created_at, updated_at, encrypted_api_key
		 FROM llm_configs WHERE user_id = $1`, userID,
	).Scan(&cfg.ID, &cfg.UserID, &cfg.Provider, &cfg.CreatedAt, &cfg.UpdatedAt, &encryptedAPIKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Config{}, nil, ErrNotFound
		}
		return Config{}, nil, err
	}
	return cfg, encryptedAPIKey, nil
}

// Delete removes the calling user's LLM configuration. It returns ErrNotFound if none was saved.
func (s *Store) Delete(ctx context.Context, userID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM llm_configs WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
