package datasource

import (
	"context"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5"
)

// postgresColumnsQuery lists every column of every user table, skipping the
// server's own catalogs. Ordering by ordinal_position keeps columns in their
// declared order.
const postgresColumnsQuery = `
	SELECT table_schema, table_name, column_name, data_type
	FROM information_schema.columns
	WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
	ORDER BY table_schema, table_name, ordinal_position`

type postgresConnector struct {
	cfg ConnectionConfig
}

func (c *postgresConnector) dsn() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=prefer",
		url.QueryEscape(c.cfg.Username), url.QueryEscape(c.cfg.Password),
		c.cfg.Host, c.cfg.Port, c.cfg.DBName,
	)
}

func (c *postgresConnector) TestConnection(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, c.dsn())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	return conn.Ping(ctx)
}

func (c *postgresConnector) Introspect(ctx context.Context) (Schema, error) {
	conn, err := pgx.Connect(ctx, c.dsn())
	if err != nil {
		return Schema{}, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, postgresColumnsQuery)
	if err != nil {
		return Schema{}, err
	}
	defer rows.Close()

	columns := make([]columnRow, 0)
	for rows.Next() {
		var row columnRow
		if err := rows.Scan(&row.Schema, &row.Table, &row.Column, &row.Type); err != nil {
			return Schema{}, err
		}
		columns = append(columns, row)
	}
	if err := rows.Err(); err != nil {
		return Schema{}, err
	}

	return Schema{Tables: groupColumns(columns)}, nil
}

func (c *postgresConnector) QueryLanguage() string {
	return "PostgreSQL SQL"
}

// RunQuery executes the query inside a read-only transaction. The query is
// assembled from a validated specification rather than supplied as text; the
// transaction is the second line of defence, guaranteeing that whatever runs
// cannot modify the user's database.
func (c *postgresConnector) RunQuery(ctx context.Context, query string, args []any, limit int) (QueryResult, error) {
	conn, err := pgx.Connect(ctx, c.dsn())
	if err != nil {
		return QueryResult{}, err
	}
	defer conn.Close(ctx)

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return QueryResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return QueryResult{}, err
	}
	defer rows.Close()

	result := QueryResult{Columns: make([]string, 0), Rows: make([][]any, 0)}
	for _, field := range rows.FieldDescriptions() {
		result.Columns = append(result.Columns, field.Name)
	}

	for rows.Next() {
		if len(result.Rows) == limit {
			result.Truncated = true
			break
		}
		values, err := rows.Values()
		if err != nil {
			return QueryResult{}, err
		}
		row := make([]any, len(values))
		for i, value := range values {
			row[i] = normalizeValue(value)
		}
		result.Rows = append(result.Rows, row)
	}
	// Closing before Err reports a mid-stream failure even when the loop
	// stopped early at the limit.
	rows.Close()
	if err := rows.Err(); err != nil {
		return QueryResult{}, err
	}

	return result, nil
}
