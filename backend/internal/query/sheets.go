package query

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The Google Visualization query language has no parameter binding, so values
// have to be written into the query as literals. That makes literal rendering
// the one place a value could change the meaning of a sheets query, so
// sheetsLiteral is deliberately strict: it picks a quote the value doesn't
// contain, and refuses anything it can't render unambiguously.

// compileSheets renders a resolved spec as a visualization query. Columns are
// addressed by letter, which is how the endpoint names them, and the letter comes
// from the column's position in the header row the schema was read from.
func compileSheets(res resolved) (Compiled, error) {
	var b strings.Builder

	b.WriteString("SELECT ")
	for i, field := range res.fields {
		if i > 0 {
			b.WriteString(", ")
		}
		expression, err := sheetsFieldExpression(field)
		if err != nil {
			return Compiled{}, err
		}
		b.WriteString(expression)
	}

	if len(res.predicates) > 0 {
		b.WriteString(" WHERE ")
		for i, predicate := range res.predicates {
			if i > 0 {
				b.WriteString(" AND ")
			}
			text, err := sheetsPredicate(predicate)
			if err != nil {
				return Compiled{}, err
			}
			b.WriteString(text)
		}
	}

	if groups := res.groupFields(); len(groups) > 0 {
		b.WriteString(" GROUP BY ")
		for i, field := range groups {
			if i > 0 {
				b.WriteString(", ")
			}
			expression, err := sheetsFieldExpression(field)
			if err != nil {
				return Compiled{}, err
			}
			b.WriteString(expression)
		}
	}

	if len(res.sorts) > 0 {
		b.WriteString(" ORDER BY ")
		for i, sort := range res.sorts {
			if i > 0 {
				b.WriteString(", ")
			}
			expression, err := sheetsFieldExpression(sort.field)
			if err != nil {
				return Compiled{}, err
			}
			b.WriteString(expression)
			if sort.descending {
				b.WriteString(" DESC")
			} else {
				b.WriteString(" ASC")
			}
		}
	}

	b.WriteString(" LIMIT " + strconv.Itoa(res.limit))

	// Labels name the returned columns the same way the SQL dialects do, so a
	// report's field mapping doesn't depend on which kind of source it reads.
	b.WriteString(" LABEL ")
	for i, field := range res.fields {
		if i > 0 {
			b.WriteString(", ")
		}
		expression, err := sheetsFieldExpression(field)
		if err != nil {
			return Compiled{}, err
		}
		label, err := sheetsLiteral(field.output)
		if err != nil {
			return Compiled{}, err
		}
		b.WriteString(expression + " " + label)
	}

	return Compiled{Text: b.String(), Columns: res.outputs()}, nil
}

// sheetsFieldExpression renders a field as a column letter, optionally wrapped
// in an aggregate.
func sheetsFieldExpression(field resolvedField) (string, error) {
	if field.counting {
		// The visualization language has no count(*): every aggregate takes a
		// column, and count skips empty cells.
		return "", fmt.Errorf("%w: counting rows isn't supported on Google Sheets — count a specific column instead",
			ErrInvalidSpec)
	}

	letter := columnLetter(field.column.Index)
	if field.aggregate == AggregateNone {
		return letter, nil
	}
	return strings.ToLower(sqlFunction[field.aggregate]) + "(" + letter + ")", nil
}

// columnLetter maps a zero-based column index to its spreadsheet letter
// (0 -> A, 25 -> Z, 26 -> AA).
func columnLetter(index int) string {
	letters := ""
	for index >= 0 {
		letters = string(rune('A'+index%26)) + letters
		index = index/26 - 1
	}
	return letters
}

// sheetsComparisons are the operators the visualization language spells the same
// way SQL does.
var sheetsComparisons = map[Operator]string{
	OpEquals:       "=",
	OpNotEquals:    "!=",
	OpGreater:      ">",
	OpGreaterEqual: ">=",
	OpLess:         "<",
	OpLessEqual:    "<=",
}

func sheetsPredicate(p predicate) (string, error) {
	letter := columnLetter(p.column.Index)

	if comparison, ok := sheetsComparisons[p.operator]; ok {
		literal, err := sheetsLiteral(p.values[0])
		if err != nil {
			return "", err
		}
		return letter + " " + comparison + " " + literal, nil
	}

	switch p.operator {
	case OpIsNull:
		return letter + " IS NULL", nil
	case OpIsNotNull:
		return letter + " IS NOT NULL", nil
	case OpContains:
		literal, err := sheetsLiteral(p.values[0])
		if err != nil {
			return "", err
		}
		return letter + " CONTAINS " + literal, nil
	case OpBetween:
		// The language has no BETWEEN, so it becomes the pair of comparisons it
		// stands for.
		low, err := sheetsLiteral(p.values[0])
		if err != nil {
			return "", err
		}
		high, err := sheetsLiteral(p.values[1])
		if err != nil {
			return "", err
		}
		return "(" + letter + " >= " + low + " AND " + letter + " <= " + high + ")", nil
	case OpIn:
		// Nor an IN list.
		alternatives := make([]string, 0, len(p.values))
		for _, value := range p.values {
			literal, err := sheetsLiteral(value)
			if err != nil {
				return "", err
			}
			alternatives = append(alternatives, letter+" = "+literal)
		}
		return "(" + strings.Join(alternatives, " OR ") + ")", nil
	default:
		return "", fmt.Errorf("%w: %q is not a supported filter", ErrInvalidSpec, p.operator)
	}
}

// sheetsLiteral renders a value for a language with no parameter binding.
//
// A quoted string is closed by its own quote character, so rather than escaping
// one, this picks the quote the value doesn't use. A value containing both
// quote characters, or any control character, has no unambiguous rendering and
// is refused — a filter that can't be expressed exactly is not one to guess at.
func sheetsLiteral(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return sheetsString(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(typed), nil
	case time.Time:
		return "datetime '" + typed.UTC().Format("2006-01-02 15:04:05") + "'", nil
	default:
		return "", fmt.Errorf("%w: %v can't be used in a Google Sheets filter", ErrInvalidSpec, value)
	}
}

func sheetsString(value string) (string, error) {
	if strings.ContainsFunc(value, isControl) {
		return "", fmt.Errorf("%w: a filter value can't contain line breaks or control characters", ErrInvalidSpec)
	}

	switch {
	case !strings.Contains(value, `'`):
		return `'` + value + `'`, nil
	case !strings.Contains(value, `"`):
		return `"` + value + `"`, nil
	default:
		return "", fmt.Errorf(`%w: a Google Sheets filter value can't contain both ' and " — %q does`,
			ErrInvalidSpec, value)
	}
}

func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f
}
