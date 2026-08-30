package datasource

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a data source doesn't exist or isn't owned by the requesting user.
var ErrNotFound = errors.New("data source not found")

// DataSource is a row from the data_sources table (never carries decrypted
// secrets). Host/Port/DBName/Username/EncryptedPassword apply to SQL sources;
// SpreadsheetID/SpreadsheetName/GoogleConnectionID apply to Google Sheets
// sources; RestURL/RestMethod/RestHeaders/RestAuthType/RestBody apply to REST
// API sources. Fields that don't apply to a given Type are nil.
type DataSource struct {
	ID                 string
	UserID             string
	Name               string
	Type               string
	Host               *string
	Port               *int
	DBName             *string
	Username           *string
	SpreadsheetID      *string
	SpreadsheetName    *string
	GoogleConnectionID *string
	RestURL            *string
	RestMethod         *string
	// RestHeaders is the JSON-encoded []RestHeader for this source, or nil.
	RestHeaders  *string
	RestAuthType *string
	RestBody     *string
	CreatedAt    time.Time
}

// Store persists and retrieves data sources from PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create inserts a new SQL data source with an already-encrypted password, returning the saved row.
func (s *Store) Create(ctx context.Context, ds DataSource, encryptedPassword []byte) (DataSource, error) {
	var created DataSource
	err := s.pool.QueryRow(ctx,
		`INSERT INTO data_sources (user_id, name, type, host, port, db_name, username, encrypted_password)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, user_id, name, type, host, port, db_name, username,
			spreadsheet_id, spreadsheet_name, google_connection_id,
			rest_url, rest_method, rest_headers, rest_auth_type, rest_body, created_at`,
		ds.UserID, ds.Name, ds.Type, ds.Host, ds.Port, ds.DBName, ds.Username, encryptedPassword,
	).Scan(&created.ID, &created.UserID, &created.Name, &created.Type, &created.Host,
		&created.Port, &created.DBName, &created.Username,
		&created.SpreadsheetID, &created.SpreadsheetName, &created.GoogleConnectionID,
		&created.RestURL, &created.RestMethod, &created.RestHeaders, &created.RestAuthType, &created.RestBody,
		&created.CreatedAt)
	if err != nil {
		return DataSource{}, err
	}
	return created, nil
}

// CreateSheetsSource inserts a new Google Sheets data source, returning the saved row.
func (s *Store) CreateSheetsSource(ctx context.Context, ds DataSource) (DataSource, error) {
	var created DataSource
	err := s.pool.QueryRow(ctx,
		`INSERT INTO data_sources (user_id, name, type, spreadsheet_id, spreadsheet_name, google_connection_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, name, type, host, port, db_name, username,
			spreadsheet_id, spreadsheet_name, google_connection_id,
			rest_url, rest_method, rest_headers, rest_auth_type, rest_body, created_at`,
		ds.UserID, ds.Name, ds.Type, ds.SpreadsheetID, ds.SpreadsheetName, ds.GoogleConnectionID,
	).Scan(&created.ID, &created.UserID, &created.Name, &created.Type, &created.Host,
		&created.Port, &created.DBName, &created.Username,
		&created.SpreadsheetID, &created.SpreadsheetName, &created.GoogleConnectionID,
		&created.RestURL, &created.RestMethod, &created.RestHeaders, &created.RestAuthType, &created.RestBody,
		&created.CreatedAt)
	if err != nil {
		return DataSource{}, err
	}
	return created, nil
}

// CreateRestSource inserts a new REST API data source with an
// already-encrypted auth config, returning the saved row.
func (s *Store) CreateRestSource(ctx context.Context, ds DataSource, encryptedAuthConfig []byte) (DataSource, error) {
	var created DataSource
	err := s.pool.QueryRow(ctx,
		`INSERT INTO data_sources (user_id, name, type, rest_url, rest_method, rest_headers,
			rest_auth_type, rest_encrypted_auth_config, rest_body)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, user_id, name, type, host, port, db_name, username,
			spreadsheet_id, spreadsheet_name, google_connection_id,
			rest_url, rest_method, rest_headers, rest_auth_type, rest_body, created_at`,
		ds.UserID, ds.Name, ds.Type, ds.RestURL, ds.RestMethod, ds.RestHeaders,
		ds.RestAuthType, encryptedAuthConfig, ds.RestBody,
	).Scan(&created.ID, &created.UserID, &created.Name, &created.Type, &created.Host,
		&created.Port, &created.DBName, &created.Username,
		&created.SpreadsheetID, &created.SpreadsheetName, &created.GoogleConnectionID,
		&created.RestURL, &created.RestMethod, &created.RestHeaders, &created.RestAuthType, &created.RestBody,
		&created.CreatedAt)
	if err != nil {
		return DataSource{}, err
	}
	return created, nil
}

// List returns all data sources owned by the given user, most recently created first.
func (s *Store) List(ctx context.Context, userID string) ([]DataSource, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, type, host, port, db_name, username,
			spreadsheet_id, spreadsheet_name, google_connection_id,
			rest_url, rest_method, rest_headers, rest_auth_type, rest_body, created_at
		 FROM data_sources WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := make([]DataSource, 0)
	for rows.Next() {
		var ds DataSource
		if err := rows.Scan(&ds.ID, &ds.UserID, &ds.Name, &ds.Type, &ds.Host,
			&ds.Port, &ds.DBName, &ds.Username,
			&ds.SpreadsheetID, &ds.SpreadsheetName, &ds.GoogleConnectionID,
			&ds.RestURL, &ds.RestMethod, &ds.RestHeaders, &ds.RestAuthType, &ds.RestBody,
			&ds.CreatedAt); err != nil {
			return nil, err
		}
		sources = append(sources, ds)
	}
	return sources, rows.Err()
}

// Get returns a data source owned by the given user, along with its
// still-encrypted SQL password and REST auth config (nil for source types
// that don't use them). It returns ErrNotFound if no row matched.
func (s *Store) Get(ctx context.Context, userID, id string) (DataSource, []byte, []byte, error) {
	var ds DataSource
	var encryptedPassword, encryptedAuthConfig []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, type, host, port, db_name, username,
			spreadsheet_id, spreadsheet_name, google_connection_id,
			rest_url, rest_method, rest_headers, rest_auth_type, rest_body, created_at,
			encrypted_password, rest_encrypted_auth_config
		 FROM data_sources WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&ds.ID, &ds.UserID, &ds.Name, &ds.Type, &ds.Host,
		&ds.Port, &ds.DBName, &ds.Username,
		&ds.SpreadsheetID, &ds.SpreadsheetName, &ds.GoogleConnectionID,
		&ds.RestURL, &ds.RestMethod, &ds.RestHeaders, &ds.RestAuthType, &ds.RestBody, &ds.CreatedAt,
		&encryptedPassword, &encryptedAuthConfig)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DataSource{}, nil, nil, ErrNotFound
		}
		return DataSource{}, nil, nil, err
	}
	return ds, encryptedPassword, encryptedAuthConfig, nil
}

// Delete removes a data source owned by the given user. It returns ErrNotFound if no row matched.
func (s *Store) Delete(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM data_sources WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
