// Package recipient manages the people (or other systems) a report can be
// sent to: their contact details, free-form attributes a report's inputs can
// filter by, and the groups they're organized into.
package recipient

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a recipient doesn't exist or isn't owned by the requesting user.
var ErrNotFound = errors.New("recipient not found")

// Recipient is a row from the recipients table.
type Recipient struct {
	ID     string
	UserID string
	Email  string
	Name   string
	// Attributes are free-form values a report's inputs can be filled from at
	// send time (e.g. "region"), keyed by whatever name a report's input asks for.
	Attributes map[string]any
	CreatedAt  time.Time
}

// Store persists and retrieves recipients from PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func scanRecipient(row pgx.Row) (Recipient, error) {
	var rec Recipient
	var attributes []byte
	if err := row.Scan(&rec.ID, &rec.UserID, &rec.Email, &rec.Name, &attributes, &rec.CreatedAt); err != nil {
		return Recipient{}, err
	}
	if len(attributes) > 0 {
		if err := json.Unmarshal(attributes, &rec.Attributes); err != nil {
			return Recipient{}, err
		}
	}
	return rec, nil
}

// Create inserts a new recipient, returning the saved row.
func (s *Store) Create(ctx context.Context, rec Recipient) (Recipient, error) {
	if rec.Attributes == nil {
		rec.Attributes = map[string]any{}
	}
	attributes, err := json.Marshal(rec.Attributes)
	if err != nil {
		return Recipient{}, err
	}

	created, err := scanRecipient(s.pool.QueryRow(ctx,
		`INSERT INTO recipients (user_id, email, name, attributes)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, email, name, attributes, created_at`,
		rec.UserID, rec.Email, rec.Name, attributes,
	))
	if err != nil {
		return Recipient{}, err
	}
	return created, nil
}

// List returns all recipients owned by the given user, most recently created first.
func (s *Store) List(ctx context.Context, userID string) ([]Recipient, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, email, name, attributes, created_at
		 FROM recipients WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	recipients := make([]Recipient, 0)
	for rows.Next() {
		rec, err := scanRecipient(rows)
		if err != nil {
			return nil, err
		}
		recipients = append(recipients, rec)
	}
	return recipients, rows.Err()
}

// Delete removes a recipient owned by the given user. It returns ErrNotFound if no row matched.
func (s *Store) Delete(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM recipients WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
