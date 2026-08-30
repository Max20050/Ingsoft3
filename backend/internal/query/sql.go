package query

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Max20050/docuwave/internal/datasource"
)

// Compile turns a specification into a query the data source can run.
//
// Every identifier in the result came from schema, and on SQL sources every
// value is bound as a parameter rather than written into the text. now fixes the
// point relative date windows are measured from, so a compile is reproducible.
func Compile(spec Spec, schema datasource.Schema, dialect Dialect, now time.Time) (Compiled, error) {
	res, err := resolve(spec, schema, dialect, now)
	if err != nil {
		return Compiled{}, err
	}

	switch dialect {
	case DialectPostgres:
		return compileSQL(res, postgresSyntax)
	case DialectMySQL:
		return compileSQL(res, mysqlSyntax)
	case DialectSheets:
		return compileSheets(res)
	case DialectREST:
		return compileREST(res)
	default:
		return Compiled{}, fmt.Errorf("%w: %s", ErrUnsupportedSource, dialect)
	}
}

// sqlSyntax is everything that differs between the SQL engines: how identifiers
// are quoted, and how a bound value is referred to.
type sqlSyntax struct {
	quoteIdentifier func(name string) string
	quoteTable      func(name string) string
	placeholder     func(position int) string
}

var postgresSyntax = sqlSyntax{
	quoteIdentifier: doubleQuote,
	// Introspection reports a table as schema.table, so the prefix is quoted as
	// its own identifier. A dot in a schema name would be misread here; it can't
	// be misused, because the name has already been matched against the schema.
	quoteTable: func(name string) string {
		schema, table, qualified := strings.Cut(name, ".")
		if !qualified {
			return doubleQuote(name)
		}
		return doubleQuote(schema) + "." + doubleQuote(table)
	},
	placeholder: func(position int) string { return "$" + strconv.Itoa(position) },
}

var mysqlSyntax = sqlSyntax{
	quoteIdentifier: backtickQuote,
	// MySQL connections name one database, so introspection reports bare tables.
	quoteTable:  backtickQuote,
	placeholder: func(int) string { return "?" },
}

// doubleQuote quotes an identifier for engines that follow the SQL standard,
// doubling any quote inside it.
func doubleQuote(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// backtickQuote quotes an identifier for MySQL, doubling any backtick inside it.
func backtickQuote(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func compileSQL(res resolved, syntax sqlSyntax) (Compiled, error) {
	args := make([]any, 0)
	// bind adds a value to the arguments and returns the placeholder standing in
	// for it. Values only ever reach the query this way.
	bind := func(value any) string {
		args = append(args, value)
		return syntax.placeholder(len(args))
	}

	var b strings.Builder
	b.WriteString("SELECT ")
	for i, field := range res.fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(sqlFieldExpression(field, syntax))
		b.WriteString(" AS ")
		b.WriteString(syntax.quoteIdentifier(field.output))
	}

	b.WriteString("\nFROM ")
	b.WriteString(syntax.quoteTable(res.table))

	if len(res.predicates) > 0 {
		b.WriteString("\nWHERE ")
		for i, predicate := range res.predicates {
			if i > 0 {
				b.WriteString(" AND ")
			}
			text, err := sqlPredicate(predicate, syntax, bind)
			if err != nil {
				return Compiled{}, err
			}
			b.WriteString(text)
		}
	}

	if groups := res.groupFields(); len(groups) > 0 {
		b.WriteString("\nGROUP BY ")
		for i, field := range groups {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(sqlFieldExpression(field, syntax))
		}
	}

	if len(res.sorts) > 0 {
		b.WriteString("\nORDER BY ")
		for i, sort := range res.sorts {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(sqlFieldExpression(sort.field, syntax))
			if sort.descending {
				b.WriteString(" DESC")
			} else {
				b.WriteString(" ASC")
			}
		}
	}

	// The limit is a number this package validated, so it's written in directly.
	b.WriteString("\nLIMIT " + strconv.Itoa(res.limit))

	return Compiled{Text: b.String(), Args: args, Columns: res.outputs()}, nil
}

// sqlFieldExpression renders a field. The function name comes from sqlFunction,
// never from the request.
func sqlFieldExpression(field resolvedField, syntax sqlSyntax) string {
	if field.counting {
		return "COUNT(*)"
	}
	column := syntax.quoteIdentifier(field.column.Name)
	if field.aggregate == AggregateNone {
		return column
	}
	return sqlFunction[field.aggregate] + "(" + column + ")"
}

// sqlComparisons are the operators that compile to a plain binary comparison.
var sqlComparisons = map[Operator]string{
	OpEquals:       "=",
	OpNotEquals:    "<>",
	OpGreater:      ">",
	OpGreaterEqual: ">=",
	OpLess:         "<",
	OpLessEqual:    "<=",
}

func sqlPredicate(p predicate, syntax sqlSyntax, bind func(any) string) (string, error) {
	column := syntax.quoteIdentifier(p.column.Name)

	if comparison, ok := sqlComparisons[p.operator]; ok {
		return column + " " + comparison + " " + bind(p.values[0]), nil
	}

	switch p.operator {
	case OpIsNull:
		return column + " IS NULL", nil
	case OpIsNotNull:
		return column + " IS NOT NULL", nil
	case OpBetween:
		return column + " BETWEEN " + bind(p.values[0]) + " AND " + bind(p.values[1]), nil
	case OpIn:
		placeholders := make([]string, 0, len(p.values))
		for _, value := range p.values {
			placeholders = append(placeholders, bind(value))
		}
		return column + " IN (" + strings.Join(placeholders, ", ") + ")", nil
	case OpContains:
		// The pattern is a bound value like any other; only the escape character
		// is part of the text. '!' avoids the backslash, which the engines
		// disagree about inside string literals.
		return column + " LIKE " + bind(likePattern(p.values[0].(string))) + " ESCAPE '!'", nil
	default:
		return "", fmt.Errorf("%w: %q is not a supported filter", ErrInvalidSpec, p.operator)
	}
}

// likePattern turns a search term into a LIKE pattern matching it anywhere,
// with the wildcards a user typed treated as the literal characters they are.
func likePattern(term string) string {
	escaped := strings.ReplaceAll(term, "!", "!!")
	escaped = strings.ReplaceAll(escaped, "%", "!%")
	escaped = strings.ReplaceAll(escaped, "_", "!_")
	return "%" + escaped + "%"
}
