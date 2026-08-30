package datasource

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrConnectionNotFound is returned when a Google Sheets connection doesn't exist or isn't owned by the requesting user.
var ErrConnectionNotFound = errors.New("google sheets connection not found")

// SheetsConnection is a row from the google_sheets_connections table: a
// user's authorized link to their Google account, holding the encrypted
// OAuth token used to list and read their spreadsheets.
type SheetsConnection struct {
	ID        string
	UserID    string
	CreatedAt time.Time
}

// SheetsStore persists Google Sheets OAuth connections in PostgreSQL.
type SheetsStore struct {
	pool *pgxpool.Pool
}

func NewSheetsStore(pool *pgxpool.Pool) *SheetsStore {
	return &SheetsStore{pool: pool}
}

// SaveConnection stores a new OAuth connection for the user with the given already-encrypted token.
func (s *SheetsStore) SaveConnection(ctx context.Context, userID string, encryptedToken []byte) (SheetsConnection, error) {
	conn := SheetsConnection{UserID: userID}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO google_sheets_connections (user_id, encrypted_token)
		 VALUES ($1, $2) RETURNING id, created_at`,
		userID, encryptedToken,
	).Scan(&conn.ID, &conn.CreatedAt)
	if err != nil {
		return SheetsConnection{}, err
	}
	return conn, nil
}

// GetEncryptedToken returns the encrypted token for a connection owned by the given user.
func (s *SheetsStore) GetEncryptedToken(ctx context.Context, userID, connectionID string) ([]byte, error) {
	var token []byte
	err := s.pool.QueryRow(ctx,
		`SELECT encrypted_token FROM google_sheets_connections WHERE id = $1 AND user_id = $2`,
		connectionID, userID,
	).Scan(&token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrConnectionNotFound
		}
		return nil, err
	}
	return token, nil
}
