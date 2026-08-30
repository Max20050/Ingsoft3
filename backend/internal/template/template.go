// Package template turns a report's query output into a presentation-ready
// document.
//
// A template is any type implementing Template: Meta describes the slots a user
// can fill in, and Render turns rows plus that mapping into HTML. The rendering
// layer only ever talks to the interface and looks templates up in a Registry,
// so a new template — including one produced by a future drag-and-drop builder —
// is added by implementing Template and registering it, with no change to the
// pipeline. See documents/v1/report-templates.md.
package template

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrUnknownTemplate is returned when no registered template has the given ID.
var ErrUnknownTemplate = errors.New("unknown report template")

// ErrInvalidConfig is returned when a slot mapping doesn't satisfy the
// template's slots — a required slot left empty, or a column the query doesn't
// return.
var ErrInvalidConfig = errors.New("invalid template configuration")

// SlotKind says what a slot can be filled with, which is what the UI needs to
// know to render the right control for it.
type SlotKind string

const (
	// SlotText is filled with literal text the user types, e.g. a title.
	SlotText SlotKind = "text"
	// SlotColumn is filled with exactly one query output column.
	SlotColumn SlotKind = "column"
	// SlotColumns is filled with an ordered list of query output columns.
	SlotColumns SlotKind = "columns"
)

// Slot is a named hole in a template's layout that the user fills from their
// query output.
type Slot struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Kind        SlotKind `json:"kind"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	// Numeric marks a slot meant for columns of numbers, such as the measures a
	// template totals. It's a hint for the UI's field mapping, not a rule:
	// whether a column holds numbers is only knowable once the query has run,
	// and a spreadsheet's "1,234.50" counts.
	Numeric bool `json:"numeric"`
}

// Meta describes a template to the UI: what it is, and what it needs.
type Meta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Slots       []Slot `json:"slots"`
}

// Config is the user's mapping of slots to their query output. It is stored with
// the report configuration, so its JSON shape is a persisted format.
type Config struct {
	// Columns maps a column or columns slot key to query output column names,
	// in the order they should appear.
	Columns map[string][]string `json:"columns,omitempty"`
	// Text maps a text slot key to the literal value the user typed.
	Text map[string]string `json:"text,omitempty"`
}

// ColumnsFor returns the columns mapped to a slot.
func (c Config) ColumnsFor(key string) []string { return c.Columns[key] }

// ColumnFor returns the single column mapped to a slot, or "" if none is.
func (c Config) ColumnFor(key string) string {
	if mapped := c.Columns[key]; len(mapped) > 0 {
		return mapped[0]
	}
	return ""
}

// TextFor returns the trimmed text a user typed into a slot.
func (c Config) TextFor(key string) string { return strings.TrimSpace(c.Text[key]) }

// Data is the query output a template renders. GeneratedAt is optional and
// prints in the document footer when set.
type Data struct {
	Columns     []string
	Rows        [][]any
	GeneratedAt time.Time
}

// Template renders report data into a document. Implementations must be safe to
// share: a registry holds one value per template and renders concurrently.
type Template interface {
	// Meta describes the template and the slots it exposes.
	Meta() Meta
	// Render produces the document for this data and slot mapping. The bytes are
	// a complete, self-contained HTML document, which is what the preview shows
	// and what the PDF renderer consumes.
	Render(Data, Config) ([]byte, error)
}

// Validate reports whether a slot mapping is usable for a template. Pass the
// query's output columns to also check that every mapped column exists; pass nil
// when they aren't known yet, since saving a report doesn't run its query.
func Validate(t Template, cfg Config, queryColumns []string) error {
	known := make(map[string]bool, len(queryColumns))
	for _, name := range queryColumns {
		known[name] = true
	}

	for _, slot := range t.Meta().Slots {
		switch slot.Kind {
		case SlotText:
			if slot.Required && cfg.TextFor(slot.Key) == "" {
				return fmt.Errorf("%w: %s is required", ErrInvalidConfig, slot.Label)
			}
		case SlotColumn, SlotColumns:
			mapped := cfg.ColumnsFor(slot.Key)
			if slot.Required && len(mapped) == 0 {
				return fmt.Errorf("%w: map a column to %s", ErrInvalidConfig, slot.Label)
			}
			if slot.Kind == SlotColumn && len(mapped) > 1 {
				return fmt.Errorf("%w: %s takes a single column", ErrInvalidConfig, slot.Label)
			}
			if queryColumns == nil {
				continue
			}
			for _, name := range mapped {
				if !known[name] {
					return fmt.Errorf("%w: %s is mapped to %q, which the query does not return",
						ErrInvalidConfig, slot.Label, name)
				}
			}
		default:
			return fmt.Errorf("%w: slot %s has unknown kind %q", ErrInvalidConfig, slot.Key, slot.Kind)
		}
	}
	return nil
}
