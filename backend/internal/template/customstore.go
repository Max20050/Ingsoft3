package template

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CustomStore persists and retrieves a user's own custom templates from
// PostgreSQL. It's the CustomSource a CompositeSource layers over the
// built-in Registry.
type CustomStore struct {
	pool *pgxpool.Pool
}

func NewCustomStore(pool *pgxpool.Pool) *CustomStore {
	return &CustomStore{pool: pool}
}

// SavedCustomTemplate is a custom template as stored, with the bookkeeping a
// report pipeline doesn't need but a management UI does: who owns it and
// when it was created or last changed.
type SavedCustomTemplate struct {
	CustomTemplate
	CreatedAt time.Time
	UpdatedAt time.Time
}

func scanCustomTemplate(row pgx.Row) (SavedCustomTemplate, error) {
	var t SavedCustomTemplate
	var blocks []byte
	if err := row.Scan(&t.ID, &t.UserID, &t.Name, &t.Description, &blocks, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return SavedCustomTemplate{}, err
	}
	if len(blocks) > 0 {
		if err := json.Unmarshal(blocks, &t.Blocks); err != nil {
			return SavedCustomTemplate{}, fmt.Errorf("decode blocks: %w", err)
		}
	}
	return t, nil
}

const selectCustomTemplates = `
	SELECT id, user_id, name, description, blocks, created_at, updated_at
	FROM custom_templates`

// Create inserts a new custom template owned by userID, returning the saved
// row. Block IDs are expected to already be prepared (see PrepareBlocks) —
// the store persists exactly the blocks it's given.
func (s *CustomStore) Create(ctx context.Context, userID string, t CustomTemplate) (SavedCustomTemplate, error) {
	blocks, err := json.Marshal(t.Blocks)
	if err != nil {
		return SavedCustomTemplate{}, fmt.Errorf("encode blocks: %w", err)
	}

	var id string
	err = s.pool.QueryRow(ctx,
		`INSERT INTO custom_templates (user_id, name, description, blocks)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, t.Name, t.Description, blocks,
	).Scan(&id)
	if err != nil {
		return SavedCustomTemplate{}, err
	}
	return s.GetSaved(ctx, userID, id)
}

// Update replaces a custom template's name, description, and blocks. It
// returns ErrUnknownTemplate if no template with that ID is owned by userID —
// editing someone else's template isn't a permission error worth
// distinguishing from it not existing at all.
func (s *CustomStore) Update(ctx context.Context, userID, id string, t CustomTemplate) (SavedCustomTemplate, error) {
	blocks, err := json.Marshal(t.Blocks)
	if err != nil {
		return SavedCustomTemplate{}, fmt.Errorf("encode blocks: %w", err)
	}

	tag, err := s.pool.Exec(ctx,
		`UPDATE custom_templates SET name = $1, description = $2, blocks = $3, updated_at = now()
		 WHERE id = $4 AND user_id = $5`,
		t.Name, t.Description, blocks, id, userID,
	)
	if err != nil {
		return SavedCustomTemplate{}, err
	}
	if tag.RowsAffected() == 0 {
		return SavedCustomTemplate{}, ErrUnknownTemplate
	}
	return s.GetSaved(ctx, userID, id)
}

// Get returns a custom template owned by userID. It returns
// ErrUnknownTemplate if no row matched, whether the ID doesn't exist or
// belongs to a different user — a CompositeSource resolving a template it
// doesn't recognize shouldn't distinguish the two.
func (s *CustomStore) Get(ctx context.Context, userID, id string) (CustomTemplate, error) {
	t, err := scanCustomTemplate(s.pool.QueryRow(ctx,
		selectCustomTemplates+` WHERE id = $1 AND user_id = $2`, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CustomTemplate{}, ErrUnknownTemplate
		}
		return CustomTemplate{}, err
	}
	return t.CustomTemplate, nil
}

// List returns every custom template owned by userID, oldest first — the
// order they were added to the picker.
func (s *CustomStore) List(ctx context.Context, userID string) ([]CustomTemplate, error) {
	rows, err := s.pool.Query(ctx,
		selectCustomTemplates+` WHERE user_id = $1 ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]CustomTemplate, 0)
	for rows.Next() {
		t, err := scanCustomTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t.CustomTemplate)
	}
	return templates, rows.Err()
}

// GetSaved returns a custom template owned by userID along with its
// timestamps, for the HTTP layer to report back after a create or update.
func (s *CustomStore) GetSaved(ctx context.Context, userID, id string) (SavedCustomTemplate, error) {
	t, err := scanCustomTemplate(s.pool.QueryRow(ctx,
		selectCustomTemplates+` WHERE id = $1 AND user_id = $2`, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SavedCustomTemplate{}, ErrUnknownTemplate
		}
		return SavedCustomTemplate{}, err
	}
	return t, nil
}
