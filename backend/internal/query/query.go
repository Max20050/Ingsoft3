// Package query builds read-only queries from a structured specification.
//
// A Spec never contains query text. It names a table, the fields to select, how
// to filter, sort and limit them — and every one of those names is checked
// against the data source's own introspected schema before anything is
// assembled. Identifiers come from the schema and are quoted; values never
// reach the query text at all on SQL sources, where they're bound as
// parameters. Nothing a client sends is executed as SQL, so there is no query
// text to inject into.
//
// See documents/v1/query-building.md.
package query

import (
	"errors"
	"fmt"
	"slices"
)

// ErrInvalidSpec means the specification doesn't describe a query this data
// source can answer: an unknown table or column, an operator used with the
// wrong values, a limit out of range. It's the caller's to fix.
var ErrInvalidSpec = errors.New("invalid query")

// ErrUnsupportedSource means the data source type has no query compiler.
var ErrUnsupportedSource = errors.New("data source does not support query building")

const (
	// DefaultRowLimit applies when a spec doesn't set one. Reports are
	// summaries; an unbounded query against someone's production database is
	// never what they meant.
	DefaultRowLimit = 1000
	// MaxRowLimit caps what a spec may ask for.
	MaxRowLimit = 5000
)

// Aggregate is how a field's values are collapsed across a group.
type Aggregate string

const (
	// AggregateNone selects the column's own values.
	AggregateNone Aggregate = ""
	AggregateSum  Aggregate = "sum"
	AggregateAvg  Aggregate = "avg"
	AggregateMin  Aggregate = "min"
	AggregateMax  Aggregate = "max"
	// AggregateCount counts rows. With no column it counts every row in the group.
	AggregateCount Aggregate = "count"
)

// Aggregates lists every aggregate a spec may use, which is also what the UI
// offers.
var Aggregates = []Aggregate{
	AggregateNone, AggregateSum, AggregateAvg, AggregateMin, AggregateMax, AggregateCount,
}

// sqlFunction is the function name an aggregate compiles to. Membership in this
// map is the whitelist: an aggregate that isn't here is rejected, so no
// caller-supplied text ever lands in a function position.
var sqlFunction = map[Aggregate]string{
	AggregateSum:   "SUM",
	AggregateAvg:   "AVG",
	AggregateMin:   "MIN",
	AggregateMax:   "MAX",
	AggregateCount: "COUNT",
}

// Operator is a filter comparison.
type Operator string

const (
	OpEquals       Operator = "eq"
	OpNotEquals    Operator = "neq"
	OpGreater      Operator = "gt"
	OpGreaterEqual Operator = "gte"
	OpLess         Operator = "lt"
	OpLessEqual    Operator = "lte"
	OpContains     Operator = "contains"
	OpIn           Operator = "in"
	OpBetween      Operator = "between"
	OpIsNull       Operator = "is_null"
	OpIsNotNull    Operator = "is_not_null"
	// OpLastDays keeps rows from the last N days, counting today, from midnight
	// UTC. Recurring reports are written once and run forever, so their date
	// windows have to be relative.
	OpLastDays Operator = "last_days"
	// OpThisMonth and OpLastMonth cover whole calendar months, UTC.
	OpThisMonth Operator = "this_month"
	OpLastMonth Operator = "last_month"
)

// operatorArity says what values an operator takes, which is how a filter is
// validated before anything is compiled.
type operatorArity int

const (
	// arityOne takes a single value.
	arityOne operatorArity = iota
	// arityMany takes one or more values.
	arityMany
	// arityPair takes exactly two values.
	arityPair
	// arityNone takes no values.
	arityNone
	// arityCount takes a positive whole number of days.
	arityCount
)

// operatorArities is the operator whitelist: anything absent is rejected.
var operatorArities = map[Operator]operatorArity{
	OpEquals:       arityOne,
	OpNotEquals:    arityOne,
	OpGreater:      arityOne,
	OpGreaterEqual: arityOne,
	OpLess:         arityOne,
	OpLessEqual:    arityOne,
	OpContains:     arityOne,
	OpIn:           arityMany,
	OpBetween:      arityPair,
	OpIsNull:       arityNone,
	OpIsNotNull:    arityNone,
	OpLastDays:     arityCount,
	OpThisMonth:    arityNone,
	OpLastMonth:    arityNone,
}

// Operators lists every operator a spec may use.
func Operators() []Operator {
	return []Operator{
		OpEquals, OpNotEquals, OpGreater, OpGreaterEqual, OpLess, OpLessEqual,
		OpContains, OpIn, OpBetween, OpIsNull, OpIsNotNull,
		OpLastDays, OpThisMonth, OpLastMonth,
	}
}

// IsOperator reports whether op is one Operators lists. Callers validating a
// specification before it's compiled use this instead of reaching into the
// unexported arity table.
func IsOperator(op Operator) bool {
	return slices.Contains(Operators(), op)
}

// Field is one column of the result, optionally aggregated.
type Field struct {
	// Column names a column of the spec's table. It is empty only for a count
	// of rows.
	Column    string    `json:"column"`
	Aggregate Aggregate `json:"aggregate,omitempty"`
}

// Filter narrows the rows. Value carries a single value; Values carries the
// list that "in" and "between" need.
type Filter struct {
	Column   string   `json:"column"`
	Operator Operator `json:"operator"`
	Value    any      `json:"value,omitempty"`
	Values   []any    `json:"values,omitempty"`
}

// values normalizes the two shapes into one list.
func (f Filter) values() []any {
	if len(f.Values) > 0 {
		return f.Values
	}
	if f.Value == nil {
		return nil
	}
	return []any{f.Value}
}

// Sort orders the result by one of the selected fields.
type Sort struct {
	Column     string    `json:"column"`
	Aggregate  Aggregate `json:"aggregate,omitempty"`
	Descending bool      `json:"descending,omitempty"`
}

// PlaceholderFilter is a filter whose value isn't known yet: it names a
// recipient attribute the value will come from once the report is run for a
// specific recipient ("email", "name", or a key into their free-form
// attributes). It never reaches Compile — resolving it into a literal Filter
// is the delivery pipeline's job, done immediately before compiling for a
// given recipient, so an unresolved placeholder can never reach compiled SQL.
type PlaceholderFilter struct {
	Column         string   `json:"column"`
	Operator       Operator `json:"operator"`
	RecipientField string   `json:"recipientField"`
}

// Spec is a report's query: what to read, from where, and in what shape. It is
// stored with the report and compiled afresh on every run.
type Spec struct {
	// Table is a table name from the source's schema. Sources that expose a
	// single sheet rather than tables leave it empty.
	Table   string   `json:"table,omitempty"`
	Fields  []Field  `json:"fields"`
	Filters []Filter `json:"filters,omitempty"`
	// PlaceholderFilters are not part of Filters and Compile never sees them —
	// see PlaceholderFilter.
	PlaceholderFilters []PlaceholderFilter `json:"placeholderFilters,omitempty"`
	Sorts              []Sort              `json:"sorts,omitempty"`
	// Limit caps the rows the report covers. Zero means DefaultRowLimit.
	Limit int `json:"limit,omitempty"`
}

// IsZero reports whether a spec describes nothing. Reports saved before query
// building replaced LLM-written SQL have one, and can't be run.
func (s Spec) IsZero() bool {
	return len(s.Fields) == 0
}

// WithLimit returns a copy of the spec reading at most n rows. It's how a
// preview asks for a page of what the report would cover.
func (s Spec) WithLimit(n int) Spec {
	s.Limit = n
	return s
}

// Compiled is a query ready to run: text with placeholders, the values to bind
// to them, and the column names it will return.
type Compiled struct {
	Text    string
	Args    []any
	Columns []string
}

// Dialect is the query language a data source speaks.
type Dialect string

const (
	DialectPostgres Dialect = "postgres"
	DialectMySQL    Dialect = "mysql"
	// DialectSheets is the Google Visualization query language, the only thing
	// Sheets offers that runs a query.
	DialectSheets Dialect = "google_sheets"
	// DialectREST has no query language at all: a REST API source only
	// supports selecting and reordering its already-mapped fields, so
	// compileREST produces a small JSON plan instead of query text. See
	// rest.go.
	DialectREST Dialect = "rest_api"
)

// dialects maps a stored data source type onto the query language its connector
// speaks. Adding a data source type means adding an entry here — this is the
// one place query building has to learn about a new source.
var dialects = map[string]Dialect{
	"postgres":      DialectPostgres,
	"mysql":         DialectMySQL,
	"google_sheets": DialectSheets,
	"rest_api":      DialectREST,
}

// DialectFor returns the dialect for a stored data source type.
func DialectFor(sourceType string) (Dialect, error) {
	dialect, ok := dialects[sourceType]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedSource, sourceType)
	}
	return dialect, nil
}
