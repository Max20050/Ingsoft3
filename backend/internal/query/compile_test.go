package query

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Max20050/docuwave/internal/datasource"
)

// referenceTime fixes "now" so relative date windows compile to known bounds.
var referenceTime = time.Date(2026, 8, 12, 13, 45, 0, 0, time.UTC)

func sqlSchema() datasource.Schema {
	return datasource.Schema{Tables: []datasource.Table{
		{Name: "sales", Columns: []datasource.Column{
			{Name: "region", Type: "text"},
			{Name: "rep", Type: "character varying(255)"},
			{Name: "total", Type: "numeric(10,2)"},
			{Name: "units", Type: "integer"},
			{Name: "closed_at", Type: "timestamp with time zone"},
			{Name: "refunded", Type: "boolean"},
		}},
		// A table outside the default schema keeps its prefix, and one with an
		// awkward name proves identifiers are quoted rather than trusted.
		{Name: "reporting.monthly", Columns: []datasource.Column{
			{Name: `we"ird`, Type: "text"},
		}},
	}}
}

func sheetsSchema() datasource.Schema {
	return datasource.Schema{Fields: []string{"region", "rep", "total", "closed_at"}}
}

func TestCompilePostgres(t *testing.T) {
	spec := Spec{
		Table: "sales",
		Fields: []Field{
			{Column: "region"},
			{Column: "total", Aggregate: AggregateSum},
			{Aggregate: AggregateCount},
		},
		Filters: []Filter{
			{Column: "refunded", Operator: OpEquals, Value: false},
			{Column: "units", Operator: OpGreaterEqual, Value: float64(10)},
		},
		Sorts: []Sort{{Column: "total", Aggregate: AggregateSum, Descending: true}},
		Limit: 500,
	}

	compiled, err := Compile(spec, sqlSchema(), DialectPostgres, referenceTime)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	want := `SELECT "region" AS "region", SUM("total") AS "sum_total", COUNT(*) AS "row_count"
FROM "sales"
WHERE "refunded" = $1 AND "units" >= $2
GROUP BY "region"
ORDER BY SUM("total") DESC
LIMIT 500`
	if compiled.Text != want {
		t.Errorf("got:\n%s\nwant:\n%s", compiled.Text, want)
	}
	if len(compiled.Args) != 2 || compiled.Args[0] != false || compiled.Args[1] != float64(10) {
		t.Errorf("got args %#v, want [false 10]", compiled.Args)
	}
	// The templates map onto these names, so they're part of the contract.
	if got := strings.Join(compiled.Columns, ","); got != "region,sum_total,row_count" {
		t.Errorf("got columns %q, want region,sum_total,row_count", got)
	}
}

func TestCompileMySQL(t *testing.T) {
	spec := Spec{
		Table:   "sales",
		Fields:  []Field{{Column: "region"}, {Column: "units", Aggregate: AggregateAvg}},
		Filters: []Filter{{Column: "region", Operator: OpIn, Values: []any{"North", "South"}}},
		Limit:   10,
	}

	compiled, err := Compile(spec, sqlSchema(), DialectMySQL, referenceTime)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	want := "SELECT `region` AS `region`, AVG(`units`) AS `avg_units`\n" +
		"FROM `sales`\n" +
		"WHERE `region` IN (?, ?)\n" +
		"GROUP BY `region`\n" +
		"LIMIT 10"
	if compiled.Text != want {
		t.Errorf("got:\n%s\nwant:\n%s", compiled.Text, want)
	}
	if len(compiled.Args) != 2 {
		t.Errorf("got args %#v, want two", compiled.Args)
	}
}

// A column name only has to be usable, not tame: it comes from the source's own
// catalog, and quoting is what makes it safe to write down.
func TestCompileQuotesAwkwardIdentifiers(t *testing.T) {
	spec := Spec{Table: "reporting.monthly", Fields: []Field{{Column: `we"ird`}}}

	compiled, err := Compile(spec, sqlSchema(), DialectPostgres, referenceTime)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	if !strings.Contains(compiled.Text, `FROM "reporting"."monthly"`) {
		t.Errorf("qualified table not quoted per part: %s", compiled.Text)
	}
	if !strings.Contains(compiled.Text, `"we""ird"`) {
		t.Errorf("quote inside an identifier not doubled: %s", compiled.Text)
	}
}

func TestCompileWithoutAggregatesHasNoGroupBy(t *testing.T) {
	spec := Spec{Table: "sales", Fields: []Field{{Column: "region"}, {Column: "total"}}}

	compiled, err := Compile(spec, sqlSchema(), DialectPostgres, referenceTime)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	if strings.Contains(compiled.Text, "GROUP BY") {
		t.Errorf("plain field selection should not group: %s", compiled.Text)
	}
	// An unlimited query against someone's production database is never intended.
	if !strings.Contains(compiled.Text, "LIMIT 1000") {
		t.Errorf("no default limit applied: %s", compiled.Text)
	}
}

func TestCompileContainsEscapesWildcards(t *testing.T) {
	spec := Spec{
		Table:   "sales",
		Fields:  []Field{{Column: "region"}},
		Filters: []Filter{{Column: "rep", Operator: OpContains, Value: "100%_a!b"}},
	}

	compiled, err := Compile(spec, sqlSchema(), DialectPostgres, referenceTime)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	if !strings.Contains(compiled.Text, `"rep" LIKE $1 ESCAPE '!'`) {
		t.Errorf("got %s, want a LIKE with an explicit escape", compiled.Text)
	}
	// Wildcards the user typed are matched literally, so they can't widen the search.
	if want := "%100!%!_a!!b%"; compiled.Args[0] != want {
		t.Errorf("got pattern %q, want %q", compiled.Args[0], want)
	}
}

// Relative windows are what make a scheduled report keep meaning the same thing,
// and they resolve to fixed bounds at compile time rather than to date
// arithmetic inside the query.
func TestCompileRelativeDateWindows(t *testing.T) {
	tests := []struct {
		name   string
		filter Filter
		want   []any
	}{
		{
			// Counting today: 12 Aug back to 6 Aug.
			name:   "last 7 days",
			filter: Filter{Column: "closed_at", Operator: OpLastDays, Value: float64(7)},
			want:   []any{time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)},
		},
		{
			name:   "this month",
			filter: Filter{Column: "closed_at", Operator: OpThisMonth},
			want: []any{
				time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:   "last month",
			filter: Filter{Column: "closed_at", Operator: OpLastMonth},
			want: []any{
				time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := Compile(Spec{
				Table:   "sales",
				Fields:  []Field{{Column: "region"}},
				Filters: []Filter{tt.filter},
			}, sqlSchema(), DialectPostgres, referenceTime)
			if err != nil {
				t.Fatalf("Compile returned error: %v", err)
			}

			if len(compiled.Args) != len(tt.want) {
				t.Fatalf("got args %#v, want %#v", compiled.Args, tt.want)
			}
			for i, want := range tt.want {
				if !compiled.Args[i].(time.Time).Equal(want.(time.Time)) {
					t.Errorf("bound %d: got %v, want %v", i, compiled.Args[i], want)
				}
			}
			// No date functions in the text: the same query means the same thing
			// wherever it runs.
			if strings.Contains(compiled.Text, "now()") || strings.Contains(compiled.Text, "INTERVAL") {
				t.Errorf("window compiled to date arithmetic: %s", compiled.Text)
			}
		})
	}
}

// Values arrive as JSON, and the column's declared type decides what they have
// to become before they're bound.
func TestCompileCoercesValuesToColumnTypes(t *testing.T) {
	tests := []struct {
		name   string
		filter Filter
		want   any
	}{
		{"number from text", Filter{Column: "units", Operator: OpEquals, Value: "42"}, float64(42)},
		{"number stays a number", Filter{Column: "total", Operator: OpEquals, Value: float64(1.5)}, 1.5},
		{"boolean from text", Filter{Column: "refunded", Operator: OpEquals, Value: "true"}, true},
		{"text from a number", Filter{Column: "region", Operator: OpEquals, Value: float64(42)}, "42"},
		{
			"date from a plain date",
			Filter{Column: "closed_at", Operator: OpEquals, Value: "2026-08-01"},
			time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := Compile(Spec{
				Table:   "sales",
				Fields:  []Field{{Column: "region"}},
				Filters: []Filter{tt.filter},
			}, sqlSchema(), DialectPostgres, referenceTime)
			if err != nil {
				t.Fatalf("Compile returned error: %v", err)
			}
			if len(compiled.Args) != 1 {
				t.Fatalf("got args %#v, want one", compiled.Args)
			}
			if want, ok := tt.want.(time.Time); ok {
				if !compiled.Args[0].(time.Time).Equal(want) {
					t.Errorf("got %v, want %v", compiled.Args[0], want)
				}
				return
			}
			if compiled.Args[0] != tt.want {
				t.Errorf("got %#v, want %#v", compiled.Args[0], tt.want)
			}
		})
	}
}

func TestCompileRejectsBadSpecs(t *testing.T) {
	tests := []struct {
		name     string
		spec     Spec
		schema   datasource.Schema
		dialect  Dialect
		wantText string
	}{
		{
			name:     "no fields",
			spec:     Spec{Table: "sales"},
			wantText: "at least one field",
		},
		{
			name:     "no table",
			spec:     Spec{Fields: []Field{{Column: "region"}}},
			wantText: "choose a table",
		},
		{
			// The classic injection attempt: it isn't escaped, it's rejected,
			// because it isn't a table this source has.
			name:     "table that isn't in the schema",
			spec:     Spec{Table: "sales; DROP TABLE users", Fields: []Field{{Column: "region"}}},
			wantText: "is not a table",
		},
		{
			name:     "column that isn't in the table",
			spec:     Spec{Table: "sales", Fields: []Field{{Column: "region) FROM users --"}}},
			wantText: "is not a column",
		},
		{
			name:     "aggregate that isn't supported",
			spec:     Spec{Table: "sales", Fields: []Field{{Column: "total", Aggregate: "median"}}},
			wantText: "not a supported aggregate",
		},
		{
			name: "filter that isn't supported",
			spec: Spec{Table: "sales", Fields: []Field{{Column: "region"}},
				Filters: []Filter{{Column: "region", Operator: "matches_regex", Value: ".*"}}},
			wantText: "not a supported filter",
		},
		{
			name: "filter missing its value",
			spec: Spec{Table: "sales", Fields: []Field{{Column: "region"}},
				Filters: []Filter{{Column: "region", Operator: OpEquals}}},
			wantText: "needs a value",
		},
		{
			name: "between with one bound",
			spec: Spec{Table: "sales", Fields: []Field{{Column: "region"}},
				Filters: []Filter{{Column: "units", Operator: OpBetween, Values: []any{float64(1)}}}},
			wantText: "needs two values",
		},
		{
			name: "is_null with a value",
			spec: Spec{Table: "sales", Fields: []Field{{Column: "region"}},
				Filters: []Filter{{Column: "region", Operator: OpIsNull, Value: "x"}}},
			wantText: "takes no value",
		},
		{
			name: "relative window on something that isn't a date",
			spec: Spec{Table: "sales", Fields: []Field{{Column: "region"}},
				Filters: []Filter{{Column: "region", Operator: OpThisMonth}}},
			wantText: "needs a date column",
		},
		{
			name: "contains against a number",
			spec: Spec{Table: "sales", Fields: []Field{{Column: "region"}},
				Filters: []Filter{{Column: "units", Operator: OpContains, Value: "5"}}},
			wantText: "needs text",
		},
		{
			name: "value that isn't a number for a numeric column",
			spec: Spec{Table: "sales", Fields: []Field{{Column: "region"}},
				Filters: []Filter{{Column: "units", Operator: OpEquals, Value: "ten"}}},
			wantText: "is not a number",
		},
		{
			name:     "the same field twice",
			spec:     Spec{Table: "sales", Fields: []Field{{Column: "region"}, {Column: "region"}}},
			wantText: "selected twice",
		},
		{
			name: "sorting by something not selected",
			spec: Spec{Table: "sales", Fields: []Field{{Column: "region"}},
				Sorts: []Sort{{Column: "total", Aggregate: AggregateSum}}},
			wantText: "one of the selected fields",
		},
		{
			name:     "a limit beyond the cap",
			spec:     Spec{Table: "sales", Fields: []Field{{Column: "region"}}, Limit: MaxRowLimit + 1},
			wantText: "cannot exceed",
		},
		{
			name:     "a negative limit",
			spec:     Spec{Table: "sales", Fields: []Field{{Column: "region"}}, Limit: -1},
			wantText: "cannot be negative",
		},
		{
			name:     "counting rows with an aggregate that needs a column",
			spec:     Spec{Table: "sales", Fields: []Field{{Aggregate: AggregateSum}}},
			wantText: "choose a column",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := tt.schema
			if len(schema.Tables) == 0 && len(schema.Fields) == 0 {
				schema = sqlSchema()
			}
			dialect := tt.dialect
			if dialect == "" {
				dialect = DialectPostgres
			}

			_, err := Compile(tt.spec, schema, dialect, referenceTime)
			if err == nil {
				t.Fatal("got no error, want an invalid spec")
			}
			if !errors.Is(err, ErrInvalidSpec) {
				t.Errorf("error %v does not wrap ErrInvalidSpec", err)
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("error %q does not mention %q", err, tt.wantText)
			}
		})
	}
}

// The point of the whole package: nothing a client sends is ever part of the
// query text on a SQL source.
func TestCompileNeverWritesValuesIntoSQL(t *testing.T) {
	hostile := []any{
		"North'; DROP TABLE users; --",
		`" OR 1=1 --`,
		"'; SELECT pg_sleep(10); --",
	}

	for _, dialect := range []Dialect{DialectPostgres, DialectMySQL} {
		compiled, err := Compile(Spec{
			Table:  "sales",
			Fields: []Field{{Column: "region"}},
			Filters: []Filter{
				{Column: "region", Operator: OpIn, Values: hostile},
				{Column: "rep", Operator: OpContains, Value: hostile[0]},
			},
		}, sqlSchema(), dialect, referenceTime)
		if err != nil {
			t.Fatalf("%s: Compile returned error: %v", dialect, err)
		}

		for _, value := range hostile {
			if strings.Contains(compiled.Text, value.(string)) {
				t.Errorf("%s: query text contains a caller-supplied value: %s", dialect, compiled.Text)
			}
		}
		if strings.Contains(compiled.Text, "DROP") || strings.Contains(compiled.Text, "--") {
			t.Errorf("%s: query text carries injected SQL: %s", dialect, compiled.Text)
		}
		if len(compiled.Args) != 4 {
			t.Errorf("%s: got %d bound values, want 4", dialect, len(compiled.Args))
		}
	}
}

func TestDialectFor(t *testing.T) {
	for sourceType, want := range map[string]Dialect{
		"postgres":      DialectPostgres,
		"mysql":         DialectMySQL,
		"google_sheets": DialectSheets,
	} {
		got, err := DialectFor(sourceType)
		if err != nil {
			t.Errorf("DialectFor(%q) returned error: %v", sourceType, err)
		}
		if got != want {
			t.Errorf("DialectFor(%q) = %q, want %q", sourceType, got, want)
		}
	}

	if _, err := DialectFor("mssql"); !errors.Is(err, ErrUnsupportedSource) {
		t.Errorf("got %v for an unsupported type, want ErrUnsupportedSource", err)
	}
}

func TestSpecHelpers(t *testing.T) {
	if !(Spec{}).IsZero() {
		t.Error("an empty spec should report itself as empty")
	}
	// Reports saved before query building have a spec like this and can't be run.
	if !(Spec{Table: "sales"}).IsZero() {
		t.Error("a spec with no fields should report itself as empty")
	}
	if (Spec{Fields: []Field{{Column: "region"}}}).IsZero() {
		t.Error("a spec with a field is not empty")
	}

	spec := Spec{Table: "sales", Fields: []Field{{Column: "region"}}, Limit: 1000}
	if got := spec.WithLimit(50); got.Limit != 50 || spec.Limit != 1000 {
		t.Errorf("WithLimit should copy: got %d, original now %d", got.Limit, spec.Limit)
	}
}
