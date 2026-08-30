package query

import (
	"encoding/json"
	"fmt"
)

// restPlan is the compiled form of a spec against a REST API source: which
// mapped fields to select, in order. There is no REST query language to
// render text in the way SQL or the Sheets visualization language have one,
// so Compiled.Text carries this encoded as JSON, and the REST connector's
// RunQuery decodes it back (matching this shape by field name — see
// backend/internal/datasource/rest_connector.go).
type restPlan struct {
	Fields []string `json:"fields"`
}

// compileREST renders a resolved spec as a REST query plan.
//
// A REST API source has no filters, joins, sorts or aggregates to speak of —
// only a configured request whose response is remapped and reordered. resolve()
// still validates filters and sorts generically against the source's schema
// (so a bad column name is still caught early), but rendering them would mean
// either expanding this package's dialect abstraction with an in-memory
// filter/sort engine, or building one inside the connector — both more than
// this issue's scope. Rather than silently drop a filter or sort the user
// configured (which would produce a report that looks filtered/sorted in the
// builder but isn't), compileREST refuses to compile a spec that has either:
// the report builder's preview surfaces that refusal as a clear error instead
// of quietly returning the wrong rows. Aggregates are refused for the same
// reason.
func compileREST(res resolved) (Compiled, error) {
	if len(res.predicates) > 0 {
		return Compiled{}, fmt.Errorf(
			"%w: filtering isn't supported for REST API sources yet — remove the filters to run this report",
			ErrInvalidSpec)
	}
	if len(res.sorts) > 0 {
		return Compiled{}, fmt.Errorf(
			"%w: sorting isn't supported for REST API sources yet — remove the sort to run this report",
			ErrInvalidSpec)
	}
	if res.grouped {
		return Compiled{}, fmt.Errorf(
			"%w: aggregates aren't supported for REST API sources yet — remove the aggregate to run this report",
			ErrInvalidSpec)
	}

	fields := make([]string, 0, len(res.fields))
	for _, field := range res.fields {
		fields = append(fields, field.column.Name)
	}

	encoded, err := json.Marshal(restPlan{Fields: fields})
	if err != nil {
		return Compiled{}, fmt.Errorf("encode rest query plan: %w", err)
	}

	return Compiled{Text: string(encoded), Columns: res.outputs()}, nil
}
