package datasource

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrIntrospectFailed wraps a failure to read a data source's structure — the
// source is unreachable, the credentials no longer work, the account lost access
// to the spreadsheet.
var ErrIntrospectFailed = errors.New("could not read schema from data source")

// SchemaProvider hands out a data source's structure. It reads it from the
// source the first time and remembers it, because query building consults the
// schema constantly and the answer only changes when the user changes their
// database.
type SchemaProvider struct {
	resolver *Resolver
	store    *SchemaStore
}

func NewSchemaProvider(resolver *Resolver, store *SchemaStore) *SchemaProvider {
	return &SchemaProvider{resolver: resolver, store: store}
}

// Schema returns a data source's stored structure, reading and storing it if
// nothing has been stored yet.
func (p *SchemaProvider) Schema(ctx context.Context, userID, dataSourceID string) (Schema, time.Time, error) {
	schema, fetchedAt, err := p.store.Get(ctx, dataSourceID)
	if err == nil {
		return schema, fetchedAt, nil
	}
	if !errors.Is(err, ErrSchemaNotFound) {
		return Schema{}, time.Time{}, err
	}
	return p.Refresh(ctx, userID, dataSourceID)
}

// Refresh re-reads a data source's structure and stores it, which is what picks
// up a table or column the user has since added.
func (p *SchemaProvider) Refresh(ctx context.Context, userID, dataSourceID string) (Schema, time.Time, error) {
	_, connector, err := p.resolver.Resolve(ctx, userID, dataSourceID)
	if err != nil {
		return Schema{}, time.Time{}, err
	}

	introspectCtx, cancel := context.WithTimeout(ctx, introspectTimeout)
	defer cancel()

	schema, err := connector.Introspect(introspectCtx)
	if err != nil {
		return Schema{}, time.Time{}, fmt.Errorf("%w: %v", ErrIntrospectFailed, err)
	}

	if err := p.store.Save(ctx, dataSourceID, schema); err != nil {
		return Schema{}, time.Time{}, err
	}
	// Reading it back would cost a round trip to learn a timestamp we just set.
	return schema, time.Now(), nil
}
