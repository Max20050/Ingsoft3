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

// ErrFieldMappingNotFound is returned when a data source has no stored field
// mapping yet.
var ErrFieldMappingNotFound = errors.New("data source field mapping not found")

// FieldMappingStore keeps a data source's api_field -> our_field mapping.
type FieldMappingStore struct {
	pool *pgxpool.Pool
}

func NewFieldMappingStore(pool *pgxpool.Pool) *FieldMappingStore {
	return &FieldMappingStore{pool: pool}
}

// Save records a data source's field mapping, replacing whatever was stored before.
func (s *FieldMappingStore) Save(ctx context.Context, dataSourceID string, mapping map[string]string) error {
	encoded, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("encode field mapping: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO data_source_field_mappings (data_source_id, mapping, updated_at)
		 VALUES ($1, $2, now())
		 ON CONFLICT (data_source_id)
		 DO UPDATE SET mapping = EXCLUDED.mapping, updated_at = now()`,
		dataSourceID, encoded)
	return err
}

// Get returns a data source's stored field mapping and when it was last
// saved. It returns ErrFieldMappingNotFound if none has been stored.
func (s *FieldMappingStore) Get(ctx context.Context, dataSourceID string) (map[string]string, time.Time, error) {
	var encoded []byte
	var updatedAt time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT mapping, updated_at FROM data_source_field_mappings WHERE data_source_id = $1`,
		dataSourceID).Scan(&encoded, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, time.Time{}, ErrFieldMappingNotFound
		}
		return nil, time.Time{}, err
	}

	var mapping map[string]string
	if err := json.Unmarshal(encoded, &mapping); err != nil {
		return nil, time.Time{}, fmt.Errorf("decode field mapping: %w", err)
	}
	return mapping, updatedAt, nil
}
