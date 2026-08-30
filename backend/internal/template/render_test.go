package template

import (
	"strings"
	"testing"
	"time"
)

func sampleData() Data {
	return Data{
		Columns: []string{"region", "rep", "revenue", "units"},
		Rows: [][]any{
			{"North", "Ada", 1200.5, int64(3)},
			// A spreadsheet hands back numbers as formatted text.
			{"North", "Bo", "1,000", int64(2)},
			{"South", "Cy", 800.0, int64(1)},
		},
		GeneratedAt: time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC),
	}
}

// Whatever the template, the output is one self-contained page — that's what the
// preview can show in an iframe and what a PDF renderer can consume.
func TestStartersRenderCompleteDocuments(t *testing.T) {
	data := sampleData()
	configs := map[string]Config{
		"tabular": {Columns: map[string][]string{"columns": {"region", "revenue"}}},
		"grouped-totals": {Columns: map[string][]string{
			"group":   {"region"},
			"metrics": {"revenue"},
		}},
		"kpi-summary": {Columns: map[string][]string{"metrics": {"revenue"}}},
	}

	registry := NewRegistry(Starters()...)
	for id, cfg := range configs {
		t.Run(id, func(t *testing.T) {
			html, err := registry.Render(id, data, cfg)
			if err != nil {
				t.Fatalf("Render returned error: %v", err)
			}
			page := string(html)
			for _, want := range []string{"<!DOCTYPE html>", "<style>", "</html>", "3 rows"} {
				if !strings.Contains(page, want) {
					t.Errorf("document does not contain %q", want)
				}
			}
		})
	}
}

func TestTabularRender(t *testing.T) {
	html, err := Tabular{}.Render(sampleData(), Config{
		Columns: map[string][]string{"columns": {"revenue", "region"}},
		Text:    map[string]string{"title": "Q3 revenue", "note": "Excludes refunds"},
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	page := string(html)

	for _, want := range []string{
		"Q3 revenue",
		"Excludes refunds",
		"1,200.50",
		// Text a source pre-formatted is printed as it came.
		"1,000",
		"North",
		"3 rows",
		"generated 12 Aug 2026 09:30 UTC",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("document does not contain %q", want)
		}
	}

	// Columns the user didn't map stay out of the report.
	if strings.Contains(page, "Ada") {
		t.Error("document includes a column that was not mapped")
	}

	// The mapped order decides the columns, not the query's order.
	if strings.Index(page, ">revenue<") > strings.Index(page, ">region<") {
		t.Error("columns are not printed in the mapped order")
	}

	// A column of numbers is right-aligned; a column of labels isn't.
	if !strings.Contains(page, `<th class="num">revenue</th>`) {
		t.Error("the numeric column is not right-aligned")
	}
	if !strings.Contains(page, `<th class="">region</th>`) {
		t.Error("the text column should not be right-aligned")
	}
}

// Report content comes from a user's own database, so it is data, not markup.
func TestRenderEscapesData(t *testing.T) {
	data := Data{
		Columns: []string{"name"},
		Rows:    [][]any{{`<script>alert("x")</script>`}},
	}
	html, err := Tabular{}.Render(data, Config{
		Columns: map[string][]string{"columns": {"name"}},
		Text:    map[string]string{"title": `Sales & <b>margins</b>`},
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	page := string(html)

	if strings.Contains(page, "<script>") {
		t.Error("a cell value was printed as markup")
	}
	if !strings.Contains(page, "&lt;script&gt;") {
		t.Error("the cell value is missing from the document")
	}
	if strings.Contains(page, "<b>margins</b>") {
		t.Error("a title was printed as markup")
	}
	if !strings.Contains(page, "Sales &amp; ") {
		t.Error("the title is missing from the document")
	}
}

func TestGroupedTotalsRender(t *testing.T) {
	html, err := GroupedTotals{}.Render(sampleData(), Config{
		Columns: map[string][]string{
			"group":   {"region"},
			"metrics": {"revenue", "units"},
		},
		Text: map[string]string{"title": "Revenue by region"},
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	page := string(html)

	for _, want := range []string{
		"Revenue by region",
		"Grouped by region",
		// North: 1,200.50 + 1,000 (as text) = 2,200.50 over 2 rows.
		"2,200.50",
		// South: a single 800 row.
		">800<",
		// Grand totals across every row.
		"3,000.50",
		">6<",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("document does not contain %q", want)
		}
	}

	// Groups keep the order the query returned them in.
	if strings.Index(page, ">North<") > strings.Index(page, ">South<") {
		t.Error("groups are not in first-appearance order")
	}
	if !strings.Contains(page, "<td>Total</td>") {
		t.Error("document has no grand total row")
	}
}

func TestKPISummaryRender(t *testing.T) {
	html, err := KPISummary{}.Render(sampleData(), Config{
		Columns: map[string][]string{
			"metrics": {"revenue", "units"},
			"columns": {"region", "rep"},
		},
		Text: map[string]string{"title": "Q3 at a glance"},
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	page := string(html)

	for _, want := range []string{
		"Q3 at a glance",
		`<p class="tile-label">revenue</p>`,
		`<p class="tile-value">3,000.50</p>`,
		`<p class="tile-value">6</p>`,
		"Breakdown",
		"Ada",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("document does not contain %q", want)
		}
	}
}

// The supporting table is optional, so a KPI report can be tiles alone.
func TestKPISummaryWithoutSupportingTable(t *testing.T) {
	html, err := KPISummary{}.Render(sampleData(), Config{
		Columns: map[string][]string{"metrics": {"revenue"}},
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if strings.Contains(string(html), "Breakdown") {
		t.Error("document includes a breakdown table that was not configured")
	}
}

// An empty result is a normal outcome — a report that says so beats a broken
// page or an error.
func TestRenderWithNoRows(t *testing.T) {
	empty := Data{Columns: []string{"region", "revenue"}}
	configs := map[Template]Config{
		Tabular{}: {Columns: map[string][]string{"columns": {"region"}}},
		GroupedTotals{}: {Columns: map[string][]string{
			"group":   {"region"},
			"metrics": {"revenue"},
		}},
		KPISummary{}: {Columns: map[string][]string{"metrics": {"revenue"}}},
	}

	for template, cfg := range configs {
		id := template.Meta().ID
		t.Run(id, func(t *testing.T) {
			html, err := template.Render(empty, cfg)
			if err != nil {
				t.Fatalf("Render returned error: %v", err)
			}
			page := string(html)
			if !strings.Contains(page, "0 rows") {
				t.Error("document does not report an empty result")
			}
			if id == "kpi-summary" && !strings.Contains(page, `<p class="tile-value">0</p>`) {
				t.Error("a metric with no rows should total zero")
			}
		})
	}
}

// Rendering never depends on a mapping being complete: the preview has to show
// something while the user is still filling slots in.
func TestRenderWithEmptyConfig(t *testing.T) {
	for _, template := range Starters() {
		t.Run(template.Meta().ID, func(t *testing.T) {
			html, err := template.Render(sampleData(), Config{})
			if err != nil {
				t.Fatalf("Render returned error: %v", err)
			}
			if !strings.Contains(string(html), "</html>") {
				t.Error("an unmapped template did not produce a document")
			}
		})
	}
}

// Rows shorter than the header — a ragged spreadsheet — print as missing values.
func TestRenderToleratesShortRows(t *testing.T) {
	data := Data{
		Columns: []string{"region", "revenue"},
		Rows:    [][]any{{"North"}},
	}
	html, err := Tabular{}.Render(data, Config{
		Columns: map[string][]string{"columns": {"region", "revenue"}},
	})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !strings.Contains(string(html), "—") {
		t.Error("a missing value should print as an em dash")
	}
}
