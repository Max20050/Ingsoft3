package datasource

import (
	"context"
	"encoding/json"
	"errors"
)

// Resolver rebuilds a live Connector for an already-saved data source. It
// needs both the SQL and Google Sheets dependencies because a stored source
// may be either kind, and it's shared by everything that has to reach a saved
// source — schema introspection and query execution alike.
type Resolver struct {
	store        *Store
	connections  *SheetsStore
	encryptor    *Encryptor
	sheetsConfig *GoogleSheetsConfig
	// mappings is a REST source's api_field -> our_field mapping store. A REST
	// connector needs it to remap query results (see restConnector.RunQuery);
	// no other source type uses it.
	mappings *FieldMappingStore
}

func NewResolver(
	store *Store,
	connections *SheetsStore,
	encryptor *Encryptor,
	sheetsConfig *GoogleSheetsConfig,
	mappings *FieldMappingStore,
) *Resolver {
	return &Resolver{
		store:        store,
		connections:  connections,
		encryptor:    encryptor,
		sheetsConfig: sheetsConfig,
		mappings:     mappings,
	}
}

// Resolve loads a data source owned by the user and returns it alongside a
// Connector for it. It returns ErrNotFound when the user owns no such source.
func (r *Resolver) Resolve(ctx context.Context, userID, dataSourceID string) (DataSource, Connector, error) {
	ds, encryptedPassword, encryptedAuthConfig, err := r.store.Get(ctx, userID, dataSourceID)
	if err != nil {
		return DataSource{}, nil, err
	}

	connector, err := r.connectorFor(ctx, userID, ds, encryptedPassword, encryptedAuthConfig)
	if err != nil {
		return DataSource{}, nil, err
	}
	return ds, connector, nil
}

// connectorFor builds a Connector for a loaded data source, decrypting
// whichever credential that source type uses.
func (r *Resolver) connectorFor(ctx context.Context, userID string, ds DataSource, encryptedPassword, encryptedAuthConfig []byte) (Connector, error) {
	switch ds.Type {
	case sheetsSourceType:
		if ds.GoogleConnectionID == nil {
			return nil, errors.New("google sheets data source has no connection")
		}
		token, err := sheetsToken(ctx, r.connections, r.encryptor, userID, *ds.GoogleConnectionID)
		if err != nil {
			return nil, err
		}
		return &googleSheetsConnector{
			oauthConfig:   r.sheetsConfig.oauth,
			token:         token,
			spreadsheetID: deref(ds.SpreadsheetID),
		}, nil

	case restSourceType:
		// An empty/never-saved mapping means the connector remaps nothing —
		// mirrored on the field_mapping_handlers.go Get handler, which treats
		// ErrFieldMappingNotFound as "empty mapping", not an error.
		mapping, _, err := r.mappings.Get(ctx, ds.ID)
		if err != nil {
			if !errors.Is(err, ErrFieldMappingNotFound) {
				return nil, err
			}
			mapping = map[string]string{}
		}
		return restConnectorFor(r.encryptor, ds, encryptedAuthConfig, mapping)

	default:
		password, err := r.encryptor.Decrypt(encryptedPassword)
		if err != nil {
			return nil, err
		}
		return NewConnector(ds.Type, ConnectionConfig{
			Host:     deref(ds.Host),
			Port:     deref(ds.Port),
			DBName:   deref(ds.DBName),
			Username: deref(ds.Username),
			Password: password,
		})
	}
}

// restConnectorFor rebuilds a restConnector from a stored REST API data
// source, decrypting its auth config.
func restConnectorFor(encryptor *Encryptor, ds DataSource, encryptedAuthConfig []byte, mapping map[string]string) (Connector, error) {
	var auth restAuthConfig
	if encryptedAuthConfig != nil {
		decrypted, err := encryptor.Decrypt(encryptedAuthConfig)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(decrypted), &auth); err != nil {
			return nil, err
		}
	}

	headers, err := decodeRestHeaders(ds.RestHeaders)
	if err != nil {
		return nil, err
	}

	return &restConnector{
		url:     deref(ds.RestURL),
		method:  deref(ds.RestMethod),
		headers: headers,
		auth:    auth,
		body:    deref(ds.RestBody),
		mapping: mapping,
	}, nil
}

// decodeRestHeaders parses a data source's stored JSON header list, treating
// a nil/empty value as no headers rather than an error.
func decodeRestHeaders(raw *string) ([]RestHeader, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	var headers []RestHeader
	if err := json.Unmarshal([]byte(*raw), &headers); err != nil {
		return nil, err
	}
	return headers, nil
}
