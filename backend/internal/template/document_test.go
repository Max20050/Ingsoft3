package template

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestFormatNumber(t *testing.T) {
	// A whole number prints whole; anything with a fraction takes two places, so
	// a column of them lines up on the decimal point.
	tests := map[float64]string{
		0:          "0",
		1200:       "1,200",
		-1200:      "-1,200",
		1234567:    "1,234,567",
		1200.5:     "1,200.50",
		1200.456:   "1,200.46",
		-0.5:       "-0.50",
		999:        "999",
		1000000.75: "1,000,000.75",
	}
	for value, want := range tests {
		if got := formatNumber(value); got != want {
			t.Errorf("formatNumber(%v) = %q, want %q", value, got, want)
		}
	}

	// A division by zero in someone's query shouldn't print "NaN" in a report.
	if got := formatNumber(math.NaN()); got != "—" {
		t.Errorf("formatNumber(NaN) = %q, want an em dash", got)
	}
	if got := formatNumber(math.Inf(1)); got != "—" {
		t.Errorf("formatNumber(+Inf) = %q, want an em dash", got)
	}
}

// Numbers reach a template as driver types, as JSON, or as spreadsheet text, and
// all three have to total.
func TestToNumber(t *testing.T) {
	numeric := []any{
		float64(1.5), float32(1.5), 2, int8(2), int16(2), int32(2), int64(2),
		uint(2), uint8(2), uint16(2), uint32(2), uint64(2),
		json.Number("2.5"), "2.5", " 2.5 ", "1,234.50", "$1,234.50", "-42",
	}
	for _, value := range numeric {
		if _, ok := toNumber(value); !ok {
			t.Errorf("toNumber(%#v) reported not-a-number", value)
		}
	}

	notNumeric := []any{nil, "", "   ", "North", true, json.Number("abc"), []string{"1"}}
	for _, value := range notNumeric {
		if got, ok := toNumber(value); ok {
			t.Errorf("toNumber(%#v) = %v, want not-a-number", value, got)
		}
	}

	if got, _ := toNumber("$1,234.50"); got != 1234.5 {
		t.Errorf("toNumber of formatted currency = %v, want 1234.5", got)
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		value any
		want  string
	}{
		{nil, "—"},
		// Text a source formatted itself is left exactly as typed.
		{"1,000", "1,000"},
		{"North", "North"},
		{true, "true"},
		{int64(1200), "1,200"},
		{1200.5, "1,200.50"},
		{json.Number("1200.5"), "1,200.50"},
	}
	for _, tt := range tests {
		if got := formatValue(tt.value); got != tt.want {
			t.Errorf("formatValue(%#v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

// Alignment follows the data: a column of numbers is right-aligned, and one
// label in it makes the whole column text.
func TestNumericColumnDetection(t *testing.T) {
	data := Data{
		Columns: []string{"region", "revenue", "mixed", "blank"},
		Rows: [][]any{
			{"North", 1200.5, 10, nil},
			{"South", "1,000", "n/a", ""},
		},
	}

	tests := map[string]bool{"region": false, "revenue": true, "mixed": false, "blank": false}
	for name, want := range tests {
		index, ok := data.index(name)
		if !ok {
			t.Fatalf("column %q is missing from the fixture", name)
		}
		if got := data.numeric(index); got != want {
			t.Errorf("numeric(%q) = %v, want %v", name, got, want)
		}
	}
}

// A mapping that no longer matches the query degrades to the columns that do
// exist; Validate is what reports the mismatch.
func TestResolveDropsUnknownColumns(t *testing.T) {
	data := Data{Columns: []string{"region", "revenue"}}

	resolved := data.resolve([]string{"revenue", "profit", "region"})
	if len(resolved) != 2 {
		t.Fatalf("got %d columns, want the 2 the query returns", len(resolved))
	}
	if resolved[0].Name != "revenue" || resolved[1].Name != "region" {
		t.Errorf("got %v, want the mapped order preserved", resolved)
	}
	if resolved[0].Index != 1 {
		t.Errorf("got index %d for revenue, want 1", resolved[0].Index)
	}
}

func TestSumSkipsNonNumbers(t *testing.T) {
	rows := [][]any{{10}, {"n/a"}, {nil}, {"1,000"}, {}}
	if got := sum(rows, 0); got != 1010 {
		t.Errorf("sum = %v, want 1010", got)
	}
}

func TestFooterText(t *testing.T) {
	data := Data{
		Rows:        [][]any{{1}, {2}},
		GeneratedAt: time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC),
	}

	want := "2 rows · generated 12 Aug 2026 09:30 UTC · Excludes refunds"
	if got := footerText(data, "Excludes refunds"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Without a generation time — as in a preview of an unsaved report — the
	// footer just counts the rows.
	if got := footerText(Data{Rows: [][]any{{1}}}, ""); got != "1 row" {
		t.Errorf("got %q, want %q", got, "1 row")
	}
}
