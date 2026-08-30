package query

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Max20050/docuwave/internal/datasource"
)

// restSchema is what report.Runner hands Compile for a REST source: the
// mapped our_field names, not the raw api_field names Introspect reports.
func restSchema() datasource.Schema {
	return datasource.Schema{Fields: []string{"customer_name", "order_total", "region"}}
}

func TestCompileREST(t *testing.T) {
	spec := Spec{
		Fields: []Field{{Column: "customer_name"}, {Column: "order_total"}},
		Limit:  25,
	}

	compiled, err := Compile(spec, restSchema(), DialectREST, referenceTime)
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}

	var plan restPlan
	if err := json.Unmarshal([]byte(compiled.Text), &plan); err != nil {
		t.Fatalf("compiled text is not a valid rest plan: %v", err)
	}
	if want := []string{"customer_name", "order_total"}; !stringsEqual(plan.Fields, want) {
		t.Errorf("got plan fields %#v, want %#v", plan.Fields, want)
	}
	if want := []string{"customer_name", "order_total"}; !stringsEqual(compiled.Columns, want) {
		t.Errorf("got columns %#v, want %#v", compiled.Columns, want)
	}
	// The REST query language is a JSON plan, not parameterized SQL: nothing
	// to bind.
	if len(compiled.Args) != 0 {
		t.Errorf("got args %#v, want none", compiled.Args)
	}
}

// A field the source's mapping doesn't cover isn't a column this schema
// reports, so it's rejected the same way an unknown SQL column is.
func TestCompileRESTRejectsUnmappedField(t *testing.T) {
	_, err := Compile(Spec{Fields: []Field{{Column: "unmapped_field"}}}, restSchema(), DialectREST, referenceTime)
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("got %v, want ErrInvalidSpec", err)
	}
}

// Filters/sorts/aggregates aren't rendered for REST sources — compileREST
// refuses to compile a spec carrying any, rather than silently dropping them
// and returning a report the builder shows as filtered but isn't.
func TestCompileRESTRejectsFilters(t *testing.T) {
	_, err := Compile(Spec{
		Fields:  []Field{{Column: "region"}},
		Filters: []Filter{{Column: "region", Operator: OpEquals, Value: "North"}},
	}, restSchema(), DialectREST, referenceTime)
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("got %v, want ErrInvalidSpec", err)
	}
}

func TestCompileRESTRejectsSorts(t *testing.T) {
	_, err := Compile(Spec{
		Fields: []Field{{Column: "region"}},
		Sorts:  []Sort{{Column: "region"}},
	}, restSchema(), DialectREST, referenceTime)
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("got %v, want ErrInvalidSpec", err)
	}
}

func TestCompileRESTRejectsAggregates(t *testing.T) {
	_, err := Compile(Spec{
		Fields: []Field{{Column: "order_total", Aggregate: AggregateSum}},
	}, restSchema(), DialectREST, referenceTime)
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("got %v, want ErrInvalidSpec", err)
	}
}

func TestCompileRESTRequiresMappedFields(t *testing.T) {
	_, err := Compile(Spec{Fields: []Field{{Column: "region"}}}, datasource.Schema{}, DialectREST, referenceTime)
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("got %v, want ErrInvalidSpec", err)
	}
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
