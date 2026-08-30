package datasource

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// mysqlColumnsQuery lists every column of the connected database. column_type
// is preferred over data_type because it carries the full declaration
// (varchar(255), decimal(10,2)), which is more useful downstream.
const mysqlColumnsQuery = `
	SELECT table_name, column_name, column_type
	FROM information_schema.columns
	WHERE table_schema = ?
	ORDER BY table_name, ordinal_position`

type mysqlConnector struct {
	cfg ConnectionConfig
}

func (c *mysqlConnector) open() (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?timeout=5s",
		c.cfg.Username, c.cfg.Password, c.cfg.Host, c.cfg.Port, c.cfg.DBName,
	)
	return sql.Open("mysql", dsn)
}

func (c *mysqlConnector) TestConnection(ctx context.Context) error {
	db, err := c.open()
	if err != nil {
		return err
	}
	defer db.Close()

	return db.PingContext(ctx)
}

func (c *mysqlConnector) Introspect(ctx context.Context) (Schema, error) {
	db, err := c.open()
	if err != nil {
		return Schema{}, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, mysqlColumnsQuery, c.cfg.DBName)
	if err != nil {
		return Schema{}, err
	}
	defer rows.Close()

	columns := make([]columnRow, 0)
	for rows.Next() {
		var row columnRow
		if err := rows.Scan(&row.Table, &row.Column, &row.Type); err != nil {
			return Schema{}, err
		}
		columns = append(columns, row)
	}
	if err := rows.Err(); err != nil {
		return Schema{}, err
	}

	return Schema{Tables: groupColumns(columns)}, nil
}

func (c *mysqlConnector) QueryLanguage() string {
	return "MySQL SQL"
}

// RunQuery executes the query inside a read-only transaction. The query is
// assembled from a validated specification rather than supplied as text; the
// transaction is the second line of defence, guaranteeing that whatever runs
// cannot modify the user's database.
func (c *mysqlConnector) RunQuery(ctx context.Context, query string, args []any, limit int) (QueryResult, error) {
	db, err := c.open()
	if err != nil {
		return QueryResult{}, err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return QueryResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return QueryResult{}, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return QueryResult{}, err
	}

	result := QueryResult{Columns: columns, Rows: make([][]any, 0)}
	if result.Columns == nil {
		result.Columns = make([]string, 0)
	}

	for rows.Next() {
		if len(result.Rows) == limit {
			result.Truncated = true
			break
		}
		// Scan needs a pointer per column; database/sql fills *any with the
		// driver's own type, which normalizeValue then makes JSON-safe.
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return QueryResult{}, err
		}
		for i, value := range values {
			values[i] = normalizeValue(value)
		}
		result.Rows = append(result.Rows, values)
	}
	if err := rows.Err(); err != nil {
		return QueryResult{}, err
	}

	return result, nil
}
