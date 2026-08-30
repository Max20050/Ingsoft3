package recipient

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrGroupNotFound is returned when a recipient group doesn't exist or isn't
// owned by the requesting user.
var ErrGroupNotFound = errors.New("recipient group not found")

// Group is a row from the recipient_groups table.
type Group struct {
	ID        string
	UserID    string
	Name      string
	CreatedAt time.Time
}

// GroupStore persists and retrieves recipient groups and their membership from PostgreSQL.
type GroupStore struct {
	pool *pgxpool.Pool
}

func NewGroupStore(pool *pgxpool.Pool) *GroupStore {
	return &GroupStore{pool: pool}
}

// Create inserts a new recipient group, returning the saved row.
func (s *GroupStore) Create(ctx context.Context, userID, name string) (Group, error) {
	var created Group
	err := s.pool.QueryRow(ctx,
		`INSERT INTO recipient_groups (user_id, name) VALUES ($1, $2)
		 RETURNING id, user_id, name, created_at`,
		userID, name,
	).Scan(&created.ID, &created.UserID, &created.Name, &created.CreatedAt)
	if err != nil {
		return Group{}, err
	}
	return created, nil
}

// List returns all recipient groups owned by the given user, most recently created first.
func (s *GroupStore) List(ctx context.Context, userID string) ([]Group, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, created_at FROM recipient_groups
		 WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]Group, 0)
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.UserID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// owns confirms a group belongs to the given user, so membership operations
// can't be pointed at another user's group or recipient.
func (s *GroupStore) owns(ctx context.Context, userID, groupID string) error {
	var id string
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM recipient_groups WHERE id = $1 AND user_id = $2`, groupID, userID,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrGroupNotFound
		}
		return err
	}
	return nil
}

// Delete removes a recipient group owned by the given user. Its membership
// rows cascade with it. It returns ErrGroupNotFound if no row matched.
func (s *GroupStore) Delete(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM recipient_groups WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrGroupNotFound
	}
	return nil
}

// Members returns the recipients belonging to a group owned by the given user.
func (s *GroupStore) Members(ctx context.Context, userID, groupID string) ([]Recipient, error) {
	if err := s.owns(ctx, userID, groupID); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT r.id, r.user_id, r.email, r.name, r.attributes, r.created_at
		 FROM recipients r
		 JOIN recipient_group_members m ON m.recipient_id = r.id
		 WHERE m.group_id = $1
		 ORDER BY r.created_at DESC`, groupID)
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

// AddMember adds a recipient to a group, both owned by the given user. Adding
// a recipient already in the group is a no-op.
func (s *GroupStore) AddMember(ctx context.Context, userID, groupID, recipientID string) error {
	if err := s.owns(ctx, userID, groupID); err != nil {
		return err
	}

	tag, err := s.pool.Exec(ctx,
		`INSERT INTO recipient_group_members (group_id, recipient_id)
		 SELECT $1, id FROM recipients WHERE id = $2 AND user_id = $3
		 ON CONFLICT DO NOTHING`,
		groupID, recipientID, userID)
	if err != nil {
		return err
	}
	// A no-op conflict still affects zero rows, so only an actual missing
	// recipient (not already a member) is reported as not found.
	if tag.RowsAffected() == 0 {
		var exists bool
		err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM recipient_group_members WHERE group_id = $1 AND recipient_id = $2)`,
			groupID, recipientID,
		).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
	}
	return nil
}

// RemoveMember removes a recipient from a group, both owned by the given user.
func (s *GroupStore) RemoveMember(ctx context.Context, userID, groupID, recipientID string) error {
	if err := s.owns(ctx, userID, groupID); err != nil {
		return err
	}

	tag, err := s.pool.Exec(ctx,
		`DELETE FROM recipient_group_members WHERE group_id = $1 AND recipient_id = $2`,
		groupID, recipientID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
