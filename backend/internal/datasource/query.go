package datasource

import (
	"fmt"
	"strings"
	"time"
)

// PreviewRowLimit is how many rows a query preview returns. Generated queries
// are shown to the user before they're saved, so the preview only needs to be
// big enough to judge whether the query is right.
const PreviewRowLimit = 50

// QueryResult is the outcome of running a query against a data source.
// Truncated reports that the source had more rows than the caller's limit.
type QueryResult struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	Truncated bool     `json:"truncated"`
}

// normalizeValue converts a driver-specific value into something that survives
// JSON encoding: database drivers hand back []byte for text and time.Time for
// timestamps, neither of which serializes the way a client expects.
func normalizeValue(v any) any {
	switch value := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(value)
	case time.Time:
		return value.Format(time.RFC3339)
	default:
		return value
	}
}

// columnLetter maps a zero-based column index to its spreadsheet letter
// (0 -> A, 25 -> Z, 26 -> AA), which is how the Sheets query language names
// columns.
func columnLetter(index int) string {
	letters := ""
	for index >= 0 {
		letters = string(rune('A'+index%26)) + letters
		index = index/26 - 1
	}
	return letters
}

// Describe renders the schema as the plain text handed to an LLM. SQL sources
// are listed as tables with typed columns; Sheets sources are listed by column
// letter, because that's how their query language refers to them.
func (s Schema) Describe() string {
	var b strings.Builder

	if len(s.Tables) > 0 {
		for _, table := range s.Tables {
			fmt.Fprintf(&b, "Table %s\n", table.Name)
			for _, column := range table.Columns {
				fmt.Fprintf(&b, "  - %s (%s)\n", column.Name, column.Type)
			}
		}
	}

	for i, field := range s.Fields {
		name := field
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Fprintf(&b, "Column %s: %s\n", columnLetter(i), name)
	}

	if b.Len() == 0 {
		return "(the data source reported no tables or columns)"
	}
	return strings.TrimRight(b.String(), "\n")
}
