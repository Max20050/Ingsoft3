// Package report manages saved report configurations: the data source a report
// reads, the user's description of it, the specification its query is built
// from, and the template its output is rendered through.
package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Max20050/docuwave/internal/query"
	"github.com/Max20050/docuwave/internal/render"
	"github.com/Max20050/docuwave/internal/template"
)

// ErrNotFound is returned when a report doesn't exist or isn't owned by the requesting user.
var ErrNotFound = errors.New("report not found")

// Report is a row from the reports table.
type Report struct {
	ID             string
	UserID         string
	DataSourceID   string
	DataSourceName string
	Name   string
	Prompt string
	// QuerySpec is what the report reads, structurally. Query is the SQL it last
	// compiled to, kept so the user can see it; the spec is the source of truth
	// and is recompiled on every run.
	QuerySpec query.Spec
	Query     string
	// TemplateID names the template the report renders through, and
	// TemplateConfig is that template's slot mapping.
	TemplateID     string
	TemplateConfig template.Config
	// Formats are the files the report is delivered as. A report always has at
	// least one.
	Formats   []render.Format
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store persists and retrieves report configurations from PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// selectReports reads reports with their data source's name, which is what the
// UI lists them by. The join is inner because the foreign key cascades on
// delete, so a report always has its source.
const selectReports = `
	SELECT r.id, r.user_id, r.data_source_id, d.name, r.name, r.prompt,
	       r.query, r.query_spec, r.template_id, r.template_config, r.formats,
	       r.created_at, r.updated_at
	FROM reports r
	JOIN data_sources d ON d.id = r.data_source_id`

func scanReport(row pgx.Row) (Report, error) {
	var rep Report
	// The query specification and the slot mapping are JSONB, so they come back
	// as raw JSON to decode here rather than as scannable column types.
	var spec, config []byte
	// Formats are stored as a text array; the format names are this server's, so
	// they're read back as plain strings and re-checked.
	var formats []string
	err := row.Scan(&rep.ID, &rep.UserID, &rep.DataSourceID, &rep.DataSourceName,
		&rep.Name, &rep.Prompt, &rep.Query, &spec, &rep.TemplateID, &config, &formats,
		&rep.CreatedAt, &rep.UpdatedAt)
	if err != nil {
		return Report{}, err
	}
	// A format this build doesn't know would be one removed from the registry
	// while a report still asked for it, so the report is reported as broken
	// rather than quietly delivered in fewer formats than configured.
	if rep.Formats, err = render.ParseFormats(formats); err != nil {
		return Report{}, fmt.Errorf("report %s: %w", rep.ID, err)
	}
	if len(spec) > 0 {
		if err := json.Unmarshal(spec, &rep.QuerySpec); err != nil {
			return Report{}, fmt.Errorf("decode query spec: %w", err)
		}
	}
	if len(config) > 0 {
		if err := json.Unmarshal(config, &rep.TemplateConfig); err != nil {
			return Report{}, fmt.Errorf("decode template config: %w", err)
		}
	}
	return rep, nil
}

// Create inserts a new report configuration, returning the saved row.
func (s *Store) Create(ctx context.Context, rep Report) (Report, error) {
	spec, err := json.Marshal(rep.QuerySpec)
	if err != nil {
		return Report{}, fmt.Errorf("encode query spec: %w", err)
	}
	config, err := json.Marshal(rep.TemplateConfig)
	if err != nil {
		return Report{}, fmt.Errorf("encode template config: %w", err)
	}

	var id string
	err = s.pool.QueryRow(ctx,
		`INSERT INTO reports (user_id, data_source_id, name, prompt, query, query_spec, template_id, template_config, formats)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		rep.UserID, rep.DataSourceID, rep.Name, rep.Prompt, rep.Query, spec, rep.TemplateID, config,
		render.Strings(rep.Formats),
	).Scan(&id)
	if err != nil {
		return Report{}, err
	}
	return s.Get(ctx, rep.UserID, id)
}

// Get returns a report owned by the given user. It returns ErrNotFound if no row matched.
func (s *Store) Get(ctx context.Context, userID, id string) (Report, error) {
	rep, err := scanReport(s.pool.QueryRow(ctx,
		selectReports+` WHERE r.id = $1 AND r.user_id = $2`, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Report{}, ErrNotFound
		}
		return Report{}, err
	}
	return rep, nil
}

// List returns all reports owned by the given user, most recently created first.
func (s *Store) List(ctx context.Context, userID string) ([]Report, error) {
	rows, err := s.pool.Query(ctx,
		selectReports+` WHERE r.user_id = $1 ORDER BY r.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := make([]Report, 0)
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, rep)
	}
	return reports, rows.Err()
}

// Delete removes a report owned by the given user. It returns ErrNotFound if no row matched.
func (s *Store) Delete(ctx context.Context, userID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM reports WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
