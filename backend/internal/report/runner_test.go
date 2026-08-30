package report

import (
	"context"
	"errors"
	"strings"
	"testing"

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
