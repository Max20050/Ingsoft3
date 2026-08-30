package datasource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSchemaNotFound is returned when a data source has no stored schema yet,
// which is the cue to read it from the source and store it.
var ErrSchemaNotFound = errors.New("data source schema not found")

// SchemaStore keeps the structure read from a data source, so query building
// works against known tables and columns without reaching out to the user's
// database every time.
type SchemaStore struct {
	pool *pgxpool.Pool
}

func NewSchemaStore(pool *pgxpool.Pool) *SchemaStore {
	return &SchemaStore{pool: pool}
}

// Save records a data source's schema, replacing whatever was stored before.
func (s *SchemaStore) Save(ctx context.Context, dataSourceID string, schema Schema) error {
	encoded, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("encode schema: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO data_source_schemas (data_source_id, schema, fetched_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (data_source_id)
		 DO UPDATE SET schema = EXCLUDED.schema, fetched_at = now()`,
		dataSourceID, encoded)
	return err
}

// Get returns a data source's stored schema and when it was read. It returns
// ErrSchemaNotFound if none has been stored.
func (s *SchemaStore) Get(ctx context.Context, dataSourceID string) (Schema, time.Time, error) {
	var encoded []byte
	var fetchedAt time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT schema, fetched_at FROM data_source_schemas WHERE data_source_id = $1`,
		dataSourceID).Scan(&encoded, &fetchedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Schema{}, time.Time{}, ErrSchemaNotFound
		}
		return Schema{}, time.Time{}, err
	}

	var schema Schema
	if err := json.Unmarshal(encoded, &schema); err != nil {
		return Schema{}, time.Time{}, fmt.Errorf("decode schema: %w", err)
	}
	return schema, fetchedAt, nil
}
