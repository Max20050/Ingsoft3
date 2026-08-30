package template

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ArchiveStore is the per-user overlay of which template IDs — built-in or
// custom — a user has archived. A built-in template isn't a row a user owns,
// so archiving it can't be a column on the template itself; this table is
// what makes "hidden from my picker" a fact about the user rather than the
// template, without affecting any other account or any report that already
// references it.
type ArchiveStore struct {
	pool *pgxpool.Pool
}

func NewArchiveStore(pool *pgxpool.Pool) *ArchiveStore {
	return &ArchiveStore{pool: pool}
}

// Archive marks a template ID archived for userID. It's idempotent: archiving
// an already-archived template is not an error.
func (s *ArchiveStore) Archive(ctx context.Context, userID, templateID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO template_archive_state (user_id, template_id) VALUES ($1, $2)
		 ON CONFLICT (user_id, template_id) DO NOTHING`,
		userID, templateID)
	return err
}

// Restore un-archives a template ID for userID. It's idempotent: restoring a
// template that isn't archived is not an error.
func (s *ArchiveStore) Restore(ctx context.Context, userID, templateID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM template_archive_state WHERE user_id = $1 AND template_id = $2`,
		userID, templateID)
	return err
}

// Archived returns the set of template IDs userID has archived.
func (s *ArchiveStore) Archived(ctx context.Context, userID string) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT template_id FROM template_archive_state WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	archived := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		archived[id] = true
	}
	return archived, rows.Err()
}
