// Package datasource manages user-configured SQL data sources: storing their
// (encrypted) connection details and verifying they're reachable.
package datasource

import (
	"context"
	"fmt"
)

// ConnectionConfig holds the details needed to reach a SQL data source.
type ConnectionConfig struct {
	Host     string
	Port     int
	DBName   string
	Username string
	Password string
}

// Connector verifies that a data source is reachable, reports its structure,
// and runs read-only queries against it. Each supported source type implements
// this behind NewConnector, so adding a new type never requires changing
// store/handler logic.
type Connector interface {
	TestConnection(ctx context.Context) error
	Introspect(ctx context.Context) (Schema, error)
	// QueryLanguage names the language this source speaks, which is what the UI
	// labels a compiled query with.
	QueryLanguage() string
	// RunQuery executes a read-only query and returns at most limit rows. The
	// query is built by the query package from a structured specification, and
	// args are the values its placeholders stand for — no caller-supplied text
	// reaches here.
	RunQuery(ctx context.Context, query string, args []any, limit int) (QueryResult, error)
}

type connectorFactory func(cfg ConnectionConfig) Connector

var connectorRegistry = map[string]connectorFactory{
	"postgres": func(cfg ConnectionConfig) Connector { return &postgresConnector{cfg: cfg} },
	"mysql":    func(cfg ConnectionConfig) Connector { return &mysqlConnector{cfg: cfg} },
}

// SupportedTypes lists the data source type identifiers accepted by NewConnector.
var SupportedTypes = []string{"postgres", "mysql"}

// sheetsSourceType identifies stored Google Sheets sources. They're not in
// connectorRegistry because their connector is built from an OAuth connection
// rather than a ConnectionConfig.
const sheetsSourceType = "google_sheets"

// restSourceType identifies stored REST API sources. Like Sheets, they're not
// in connectorRegistry: their connector is built from a URL/headers/auth
// configuration rather than a ConnectionConfig.
const restSourceType = "rest_api"

// NewConnector builds a Connector for the given source type.
func NewConnector(sourceType string, cfg ConnectionConfig) (Connector, error) {
	factory, ok := connectorRegistry[sourceType]
	if !ok {
		return nil, fmt.Errorf("unsupported data source type: %s", sourceType)
	}
	return factory(cfg), nil
}
