package report

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Max20050/docuwave/internal/datasource"
	"github.com/Max20050/docuwave/internal/llm"
	"github.com/Max20050/docuwave/internal/query"
	"github.com/Max20050/docuwave/internal/render"
	"github.com/Max20050/docuwave/internal/template"
)

// A report saved before query building has query text but no specification, and
// nothing runs stored text — so it fails before it reaches the data source.
func TestDocumentRejectsALegacyReport(t *testing.T) {
	runner := NewRunner(nil, nil, nil, nil, nil)

	_, err := runner.Document(context.Background(), Report{
		ID:    "r-legacy",
		Query: "SELECT * FROM sales",
	})
	if !errors.Is(err, ErrNotRunnable) {
		t.Fatalf("got %v, want ErrNotRunnable", err)
	}
}

// The formats a report is configured for are what its client agreed to receive,
// so a request for another one is refused before the query runs.
func TestRenderFormatRejectsAnUnconfiguredFormat(t *testing.T) {
	runner := NewRunner(nil, nil, nil, nil, nil)
	rep := Report{
		ID:        "r-1",
		Name:      "Monthly sales",
		QuerySpec: query.Spec{Table: "sales", Fields: []query.Field{{Column: "region"}}},
		Formats:   []render.Format{render.FormatPDF},
	}

	_, err := runner.RenderFormat(context.Background(), rep, render.FormatCSV)
	if !errors.Is(err, render.ErrUnknownFormat) {
		t.Fatalf("got %v, want ErrUnknownFormat", err)
	}
	// The message has to say what the report does offer, because the fix is to
	// ask for one of those or to reconfigure the report.
	if err == nil || !strings.Contains(err.Error(), "pdf") {
		t.Errorf("got %v, want an error naming the formats the report is configured for", err)
	}
}

// The compiled query carries the specification's limit; the connector is capped
// at the same number so a source that ignores a limit can't stream more than
// the report asked for.
func TestRunLimit(t *testing.T) {
	tests := map[string]struct {
		spec query.Spec
		want int
	}{
		"a report with its own limit":  {query.Spec{Limit: 250}, 250},
		"a report that didn't set one": {query.Spec{}, query.DefaultRowLimit},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := runLimit(tt.spec); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// formatColumnsAsText is what turns a report's rows into the plain-text
// context an ai-summary block's prompt is built from.
func TestFormatColumnsAsText(t *testing.T) {
	data := template.Data{
		Columns: []string{"region", "revenue", "notes"},
		Rows: [][]any{
			{"North", 100, "steady"},
			{"South", 200, "growing"},
		},
	}

	// A stale column the query no longer returns is dropped rather than
	// failing the whole summary.
	got := formatColumnsAsText("Sales", []string{"region", "revenue", "profit"}, data)
	if !strings.Contains(got, "Sales (region, revenue):") {
		t.Errorf("got %q, want a header naming only the columns the query returned", got)
	}
	if !strings.Contains(got, "North | 100") || !strings.Contains(got, "South | 200") {
		t.Errorf("got %q, want both rows printed", got)
	}
	if strings.Contains(got, "profit") {
		t.Errorf("got %q, want the stale column left out entirely", got)
	}

	// None of the requested columns exist: nothing worth printing.
	if got := formatColumnsAsText("Sales", []string{"profit"}, data); got != "" {
		t.Errorf("got %q, want an empty section when no requested column exists", got)
	}
}

// aiSummaryText never calls a model when the block has nothing to call it
// with — no configured generator, or no prompt — and says why instead of
// panicking or returning an empty string.
func TestAISummaryTextWithoutAProvider(t *testing.T) {
	runner := &Runner{}
	rep := Report{UserID: "u-1", TemplateConfig: template.Config{
		Text: map[string]string{"b1:prompt": "Summarize this."},
	}}
	block := template.BlockDef{ID: "b1", Kind: template.BlockAISummary}

	got := runner.aiSummaryText(context.Background(), rep, template.Data{}, block)
	if !strings.Contains(got, "not configured") {
		t.Errorf("got %q, want a message explaining summaries aren't configured", got)
	}
}

func TestAISummaryTextWithoutAPrompt(t *testing.T) {
	// A generator is configured here, so this exercises the "no prompt"
	// message specifically, not the "not configured" one above.
	runner := &Runner{summaries: llm.NewGenerator(nil, nil)}
	rep := Report{UserID: "u-1"}
	block := template.BlockDef{ID: "b1", Kind: template.BlockAISummary}

	got := runner.aiSummaryText(context.Background(), rep, template.Data{}, block)
	if !strings.Contains(got, "no prompt") {
		t.Errorf("got %q, want a message explaining the block has no prompt", got)
	}
}

// spyConnector is a mock datasource.Connector: it records every RunQuery call
// instead of reaching a real data source, so a test can verify how it was
// used rather than just what it returned.
type spyConnector struct {
	calls  int
	query  string
	args   []any
	limit  int
	result datasource.QueryResult
}

func (s *spyConnector) TestConnection(ctx context.Context) error { return nil }

func (s *spyConnector) Introspect(ctx context.Context) (datasource.Schema, error) {
	return datasource.Schema{}, nil
}

func (s *spyConnector) QueryLanguage() string { return "sql" }

func (s *spyConnector) RunQuery(ctx context.Context, q string, args []any, limit int) (datasource.QueryResult, error) {
	s.calls++
	s.query = q
	s.args = args
	s.limit = limit
	return s.result, nil
}

// runnable.run is the one place a report's compiled query reaches a
// connector, for every report in the app — the preview, the download, and
// every scheduled delivery. This verifies it hands the connector exactly the
// compiled text, args and limit it was given, and does so exactly once: a
// duplicate call here would mean a source gets queried twice for one report.
func TestRunnableRunCallsConnectorWithTheCompiledQuery(t *testing.T) {
	connector := &spyConnector{result: datasource.QueryResult{Columns: []string{"region"}}}
	r := runnable{
		connector: connector,
		compiled:  query.Compiled{Text: "SELECT region FROM sales WHERE year = $1", Args: []any{2026}},
	}

	if _, err := r.run(context.Background(), 50); err != nil {
		t.Fatalf("run returned %v, want no error", err)
	}

	if connector.calls != 1 {
		t.Fatalf("RunQuery called %d times, want exactly once", connector.calls)
	}
	if connector.query != "SELECT region FROM sales WHERE year = $1" {
		t.Errorf("RunQuery got query %q, want the compiled text unchanged", connector.query)
	}
	if len(connector.args) != 1 || connector.args[0] != 2026 {
		t.Errorf("RunQuery got args %v, want the compiled args unchanged", connector.args)
	}
	if connector.limit != 50 {
		t.Errorf("RunQuery got limit %d, want the limit run was called with", connector.limit)
	}
}
