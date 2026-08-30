package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pgUniqueViolation = "23505"

// ErrEmailTaken is returned when registering with an email that already exists.
var ErrEmailTaken = errors.New("email already registered")

// ErrUserNotFound is returned when no user matches the given lookup.
var ErrUserNotFound = errors.New("user not found")

// User is a row from the users table.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	GoogleID     *string
}

// Store persists and retrieves users from PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateUser inserts a new user with the given email and pre-hashed password, returning its ID.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`,
		email, passwordHash,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return "", ErrEmailTaken
		}
		return "", err
	}
	return id, nil
}

// GetUserByEmail looks up a user by email.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, google_id FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.GoogleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// GetUserByID looks up a user by ID.
func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, google_id FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.GoogleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// GetUserByGoogleID looks up a user by their linked Google account ID.
func (s *Store) GetUserByGoogleID(ctx context.Context, googleID string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, google_id FROM users WHERE google_id = $1`, googleID,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.GoogleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// LinkGoogleID attaches a Google account ID to an existing user.
func (s *Store) LinkGoogleID(ctx context.Context, userID, googleID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET google_id = $1 WHERE id = $2`, googleID, userID,
	)
	return err
}

// CreateGoogleUser inserts a new user authenticated via Google, with no password set.
func (s *Store) CreateGoogleUser(ctx context.Context, email, googleID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, google_id) VALUES ($1, $2) RETURNING id`,
		email, googleID,
	).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return "", ErrEmailTaken
		}
		return "", err
	}
	return id, nil
}
