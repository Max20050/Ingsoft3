package template

import (
	"errors"
	"strings"
	"testing"
)

// texts is a table's printed values, row by row, for comparing a projection
// against what the document shows.
func texts(rows [][]Value) []string {
	printed := make([]string, 0, len(rows))
	for _, row := range rows {
		cells := make([]string, 0, len(row))
		for _, value := range row {
			cells = append(cells, value.Text)
		}
		printed = append(printed, strings.Join(cells, "|"))
	}
	return printed
}

func columnNames(columns []TableColumn) string {
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, column.Name)
	}
	return strings.Join(names, "|")
}

// The document a spreadsheet or a PDF is built from has to show the columns the
// user mapped, in their order, with the same text the page prints.
func TestTabularDocument(t *testing.T) {
	doc := Tabular{}.Document(sampleData(), Config{
		Columns: map[string][]string{"columns": {"revenue", "region"}},
		Text:    map[string]string{"title": "Q3 revenue", "note": "Excludes refunds"},
	})

	if doc.Title != "Q3 revenue" {
		t.Errorf("got title %q, want the mapped title", doc.Title)
	}
	if !strings.Contains(doc.Footer, "3 rows") || !strings.Contains(doc.Footer, "Excludes refunds") {
		t.Errorf("got footer %q, want the row count and the user's note", doc.Footer)
	}
	if doc.GeneratedAt != sampleData().GeneratedAt {
		t.Error("the document does not carry when it was generated")
	}

	if len(doc.Blocks) != 1 || doc.Blocks[0].Table == nil {
		t.Fatalf("got %d blocks, want one table", len(doc.Blocks))
	}
	table := doc.Blocks[0].Table

	if got := columnNames(table.Columns); got != "revenue|region" {
		t.Errorf("got columns %q, want them in the order they were mapped", got)
	}
	if table.Columns[0].Numeric == false || table.Columns[1].Numeric {
		t.Error("alignment does not follow the data: revenue is numeric, region is not")
	}

	want := []string{"1,200.50|North", "1,000|North", "800|South"}
	for i, row := range texts(table.Rows) {
		if row != want[i] {
			t.Errorf("row %d prints %q, want %q", i, row, want[i])
		}
	}
	if len(table.Total) != 0 {
		t.Error("a plain table has no total row")
	}
}

// A value only carries a number when its whole column reads as numbers, and a
// spreadsheet is where that matters: a total the recipient computes has to
// agree with the one the report printed.
func TestDocumentValuesCarryTheirNumbers(t *testing.T) {
	doc := Tabular{}.Document(sampleData(), Config{
		Columns: map[string][]string{"columns": {"region", "revenue"}},
	})
	rows := doc.Blocks[0].Table.Rows

	if rows[0][0].IsNumber {
		t.Error("a text column was projected as numbers")
	}
	// A spreadsheet source hands back "1,000" as text; the report treats it as
	// the number it is, and the printed text stays as the source typed it.
	if !rows[1][1].IsNumber || rows[1][1].Number != 1000 {
		t.Errorf("got %+v for a source-formatted number, want 1000", rows[1][1])
	}
	if rows[1][1].Text != "1,000" {
		t.Errorf("got text %q, want the value as the source formatted it", rows[1][1].Text)
	}
}

// The grouped template's document is one row per group, not one per query row,
// with the same totals the page carries.
func TestGroupedTotalsDocument(t *testing.T) {
	doc := GroupedTotals{}.Document(sampleData(), Config{
		Columns: map[string][]string{"group": {"region"}, "metrics": {"revenue", "units"}},
	})

	if doc.Subtitle != "Grouped by region" {
		t.Errorf("got subtitle %q, want the grouping column", doc.Subtitle)
	}

	table := doc.Blocks[0].Table
	if got := columnNames(table.Columns); got != "region|Rows|revenue|units" {
		t.Errorf("got columns %q, want the group, its row count, then the measures", got)
	}

	want := []string{"North|2|2,200.50|5", "South|1|800|1"}
	for i, row := range texts(table.Rows) {
		if row != want[i] {
			t.Errorf("group %d prints %q, want %q", i, row, want[i])
		}
	}

	if got := strings.Join(texts([][]Value{table.Total}), ""); got != "Total|3|3,000.50|6" {
		t.Errorf("got total row %q, want the whole report's totals", got)
	}
}

// The KPI template leads with its figures, and only prints a table once the
// user has mapped one.
func TestKPISummaryDocument(t *testing.T) {
	cfg := Config{Columns: map[string][]string{"metrics": {"revenue"}}}
	doc := KPISummary{}.Document(sampleData(), cfg)

	if len(doc.Blocks) != 1 {
		t.Fatalf("got %d blocks, want only the headline figures", len(doc.Blocks))
	}
	tiles := doc.Blocks[0].Tiles
	if len(tiles) != 1 || tiles[0].Label != "revenue" {
		t.Fatalf("got tiles %+v, want one for revenue", tiles)
	}
	if tiles[0].Value.Number != 3000.5 || tiles[0].Value.Text != "3,000.50" {
		t.Errorf("got %+v, want the revenue total as both a number and printed text", tiles[0].Value)
	}

	cfg.Columns["columns"] = []string{"region", "rep"}
	withTable := KPISummary{}.Document(sampleData(), cfg)
	if len(withTable.Blocks) != 2 || withTable.Blocks[1].Table == nil {
		t.Fatalf("got %d blocks, want the figures and the supporting table", len(withTable.Blocks))
	}
	if withTable.Blocks[1].Title != "Breakdown" {
		t.Errorf("got block title %q, want the breakdown heading", withTable.Blocks[1].Title)
	}
}

// A preview is rendered while the user is still filling slots in, so a document
// with nothing mapped says what's missing instead of failing — and says it in
// the same words the page does.
func TestDocumentsExplainAnUnfinishedMapping(t *testing.T) {
	tests := map[string]Template{
		"tabular":        Tabular{},
		"grouped-totals": GroupedTotals{},
		"kpi-summary":    KPISummary{},
	}

	for name, starter := range tests {
		t.Run(name, func(t *testing.T) {
			doc := DocumentOf(starter, sampleData(), Config{})
			if len(doc.Blocks) == 0 || doc.Blocks[0].Note == "" {
				t.Fatalf("got %+v, want a block explaining what is missing", doc.Blocks)
			}

			html, err := starter.Render(sampleData(), Config{})
			if err != nil {
				t.Fatalf("Render returned error: %v", err)
			}
			if !strings.Contains(string(html), doc.Blocks[0].Note) {
				t.Errorf("the page and the document disagree: the page does not say %q",
					doc.Blocks[0].Note)
			}
		})
	}
}

// An empty result is a normal outcome, and every projection has to say so.
func TestDocumentsReportEmptyResults(t *testing.T) {
	data := Data{Columns: []string{"region", "revenue"}}
	doc := Tabular{}.Document(data, Config{
		Columns: map[string][]string{"columns": {"region", "revenue"}},
	})

	if doc.Blocks[0].Note != noRowsNote {
		t.Errorf("got note %q, want %q", doc.Blocks[0].Note, noRowsNote)
	}
	if len(doc.Blocks[0].Table.Columns) != 2 {
		t.Error("the columns the user mapped are printed even with no rows to fill them")
	}
}

// unprojected is a template that renders HTML but doesn't describe its
// document — the shape a future builder's output could take before it does.
type unprojected struct{}

func (unprojected) Meta() Meta {
	return Meta{ID: "unprojected", Name: "Unprojected", Slots: []Slot{
		{Key: "title", Label: "Title", Kind: SlotText},
	}}
}

func (unprojected) Render(Data, Config) ([]byte, error) {
	return []byte("<!DOCTYPE html><html></html>"), nil
}

// Every template is downloadable, whether or not it describes its document: one
// that doesn't falls back to the query's own output rather than failing.
func TestDocumentOfFallsBackToTheQueryOutput(t *testing.T) {
	doc := DocumentOf(unprojected{}, sampleData(), Config{})

	table := doc.Blocks[0].Table
	if got := columnNames(table.Columns); got != "region|rep|revenue|units" {
		t.Errorf("got columns %q, want every column the query returned", got)
	}
	if len(table.Rows) != 3 {
		t.Errorf("got %d rows, want every row the query returned", len(table.Rows))
	}
}

// Downloading a report goes through the same gate as rendering it, so a mapping
// that has gone stale is reported rather than silently printing less.
func TestRegistryDocumentChecksTheMapping(t *testing.T) {
	registry := NewRegistry(Starters()...)

	doc, err := registry.Document("tabular", sampleData(), Config{
		Columns: map[string][]string{"columns": {"region"}},
	})
	if err != nil {
		t.Fatalf("Document returned error: %v", err)
	}
	if len(doc.Blocks) != 1 {
		t.Fatalf("got %d blocks, want the table", len(doc.Blocks))
	}

	_, err = registry.Document("tabular", sampleData(), Config{
		Columns: map[string][]string{"columns": {"profit"}},
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("got %v for a column the query no longer returns, want ErrInvalidConfig", err)
	}
	if err == nil || !strings.Contains(err.Error(), "profit") {
		t.Errorf("got %v, want an error naming the stale column", err)
	}

	if _, err := registry.Document("dashboard", sampleData(), Config{}); !errors.Is(err, ErrUnknownTemplate) {
		t.Errorf("got %v for a template that isn't registered, want ErrUnknownTemplate", err)
	}
}
