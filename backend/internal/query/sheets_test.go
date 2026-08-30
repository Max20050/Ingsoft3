package query

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Max20050/docuwave/internal/datasource"
)

func TestCompileSheets(t *testing.T) {
	spec := Spec{
		Fields: []Field{{Column: "region"}, {Column: "total", Aggregate: AggregateSum}},
		Filters: []Filter{
			{Column: "closed_at", Operator: OpThisMonth},
			{Column: "rep", Operator: OpContains, Value: "Ada"},
		},
		Sorts: []Sort{{Column: "total", Aggregate: AggregateSum, Descending: true}},
		Limit: 100,
	}

	compiled, err := Compile(spec, sheetsSchema(), DialectSheets, referenceTime)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	// Columns are addressed by letter, which is how the endpoint names them, and
	// the letter is the column's position in the header row.
	want := "SELECT A, sum(C)" +
		" WHERE D >= datetime '2026-08-01 00:00:00' AND D < datetime '2026-09-01 00:00:00'" +
		" AND B CONTAINS 'Ada'" +
		" GROUP BY A" +
		" ORDER BY sum(C) DESC" +
		" LIMIT 100" +
		" LABEL A 'region', sum(C) 'sum_total'"
	if compiled.Text != want {
		t.Errorf("got:\n%s\nwant:\n%s", compiled.Text, want)
	}
	// The endpoint takes no parameters, so there is nothing to bind.
	if len(compiled.Args) != 0 {
		t.Errorf("got args %#v, want none", compiled.Args)
	}
	// Labels make the returned names match what the SQL dialects return, so a
	// report's field mapping doesn't depend on the kind of source.
	if got := strings.Join(compiled.Columns, ","); got != "region,sum_total" {
		t.Errorf("got columns %q, want region,sum_total", got)
	}
}

// The language has neither IN nor BETWEEN, so both compile to what they mean.
func TestCompileSheetsLowersMissingOperators(t *testing.T) {
	compiled, err := Compile(Spec{
		Fields: []Field{{Column: "region"}},
		Filters: []Filter{
			{Column: "region", Operator: OpIn, Values: []any{"North", "South"}},
			{Column: "total", Operator: OpBetween, Values: []any{float64(1), float64(10)}},
		},
		Limit: 10,
	}, sheetsSchema(), DialectSheets, referenceTime)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	for _, want := range []string{
		"(A = 'North' OR A = 'South')",
		"(C >= 1 AND C <= 10)",
	} {
		if !strings.Contains(compiled.Text, want) {
			t.Errorf("got %s, want it to contain %s", compiled.Text, want)
		}
	}
}

// With no parameter binding, the literal is the only defence, so a value that
// can't be written down unambiguously is refused rather than guessed at.
func TestSheetsLiteralQuoting(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"plain text", "North", `'North'`},
		// A quote inside the value picks the other quote character rather than
		// being escaped, which the language has no syntax for.
		{"text with an apostrophe", "O'Brien", `"O'Brien"`},
		{"text with a double quote", `say "hi"`, `'say "hi"'`},
		{"an injection attempt is just text", "' OR 1=1 --", `"' OR 1=1 --"`},
		{"number", float64(1.5), "1.5"},
		{"boolean", true, "true"},
		{"time", time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC), "datetime '2026-08-01 09:30:00'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sheetsLiteral(tt.value)
			if err != nil {
				t.Fatalf("sheetsLiteral returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSheetsLiteralRefusesWhatItCannotRender(t *testing.T) {
	tests := map[string]any{
		"both quote characters": `he said "it's fine"`,
		"a line break":          "North\nSouth",
		"a control character":   "North\x00",
		"an unexpected type":    []string{"North"},
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := sheetsLiteral(value); !errors.Is(err, ErrInvalidSpec) {
				t.Errorf("got %v, want ErrInvalidSpec", err)
			}
		})
	}
}

// A filter value that can't be rendered has to fail the compile, not slip
// through into a query.
func TestCompileSheetsRejectsUnrenderableValue(t *testing.T) {
	_, err := Compile(Spec{
		Fields:  []Field{{Column: "region"}},
		Filters: []Filter{{Column: "region", Operator: OpEquals, Value: `he said "it's fine"`}},
	}, sheetsSchema(), DialectSheets, referenceTime)

	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("got %v, want ErrInvalidSpec", err)
	}
}

func TestCompileSheetsRejectsCountOfRows(t *testing.T) {
	_, err := Compile(Spec{Fields: []Field{{Aggregate: AggregateCount}}},
		sheetsSchema(), DialectSheets, referenceTime)

	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("got %v, want ErrInvalidSpec", err)
	}
	if !strings.Contains(err.Error(), "count a specific column") {
		t.Errorf("error %q does not say what to do instead", err)
	}
}

// Spreadsheet headers can repeat in a way database columns can't, and a repeated
// name has no single answer.
func TestCompileSheetsRejectsAmbiguousHeader(t *testing.T) {
	schema := datasource.Schema{Fields: []string{"total", "region", "total"}}

	_, err := Compile(Spec{Fields: []Field{{Column: "total"}}}, schema, DialectSheets, referenceTime)
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("got %v, want ErrInvalidSpec", err)
	}
	if !strings.Contains(err.Error(), "more than once") {
		t.Errorf("error %q does not explain the ambiguity", err)
	}
}

func TestCompileSheetsRequiresAKnownHeader(t *testing.T) {
	_, err := Compile(Spec{Fields: []Field{{Column: "region"}}},
		datasource.Schema{}, DialectSheets, referenceTime)

	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("got %v, want ErrInvalidSpec", err)
	}
	if !strings.Contains(err.Error(), "refresh") {
		t.Errorf("error %q does not tell the user to refresh the schema", err)
	}
}

func TestColumnLetter(t *testing.T) {
	for index, want := range map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA"} {
		if got := columnLetter(index); got != want {
			t.Errorf("columnLetter(%d) = %q, want %q", index, got, want)
		}
	}
}
