package template

import "time"

// Doc is a template's document described as structure rather than as markup.
//
// Render produces HTML, which is what the layout preview shows and what a
// report's email body carries. The file formats can't read that HTML: a
// spreadsheet wants cells it can sum, and a PDF wants blocks to lay out on a
// page. So a template describes the same document a second time as blocks, and
// the file renderers build from those. Both projections come out of one
// computation inside the template, so a report's PDF, spreadsheet and preview
// always agree.
type Doc struct {
	Title    string
	Subtitle string
	// Footer is the line printed under the document: how much data it covers,
	// when it was generated, and the user's own note.
	Footer      string
	GeneratedAt time.Time
	Blocks      []Block
}

// Block is one part of a document: a row of headline figures, a table, or a
// note explaining why there is neither.
type Block struct {
	// Title heads the block when it has one.
	Title string
	// Note is printed instead of, or under, the block's content — an empty
	// result or a mapping the user hasn't finished filling in.
	Note  string
	Tiles []Tile
	// Table is the block's rows, or nil when the block has none.
	Table *Table
}

// Tile is one headline figure.
type Tile struct {
	Label string
	Value Value
}

// Table is a block of rows. Total, when set, is a summary row aligned with
// Columns that prints beneath the body.
type Table struct {
	Columns []TableColumn
	Rows    [][]Value
	Total   []Value
}

// TableColumn is a printed column. Numeric drives alignment in a PDF and
// decides whether a spreadsheet gets numbers or text in the column.
type TableColumn struct {
	Name    string
	Numeric bool
}

// Class is the alignment class an HTML render should use for this column's
// header and cells.
func (c TableColumn) Class() string {
	if c.Numeric {
		return "num"
	}
	return ""
}

// Value is one printed value: the text a reader sees, plus the number behind it
// when the value is one. A spreadsheet cell has to hold the number to stay
// summable, while a page prints the text — and the text is the same string the
// HTML document shows, so the formats never disagree about a value.
type Value struct {
	Text     string
	Number   float64
	IsNumber bool
}

// Class is the alignment class an HTML render should use for this value.
func (v Value) Class() string {
	if v.IsNumber {
		return "num"
	}
	return ""
}

// textValue is a value with no number behind it, such as a label.
func textValue(text string) Value { return Value{Text: text} }

// computedValue is a number this package worked out — a total or a tile — which
// is a number in every format regardless of how its source column was typed.
func computedValue(number float64) Value {
	return Value{Text: formatNumber(number), Number: number, IsNumber: true}
}

// cellValue prints one of a row's values. It only carries a number when its
// whole column reads as numbers: one label in a column makes the column text,
// and a spreadsheet shouldn't disagree with the document about that.
func cellValue(raw any, col column) Value {
	value := Value{Text: formatValue(raw)}
	if col.Numeric {
		if number, ok := toNumber(raw); ok {
			value.Number, value.IsNumber = number, true
		}
	}
	return value
}

// rowValues projects one row onto the given columns.
func rowValues(row []any, columns []column) []Value {
	values := make([]Value, 0, len(columns))
	for _, col := range columns {
		values = append(values, cellValue(valueAt(row, col.Index), col))
	}
	return values
}

// tableFrom builds a table block from resolved columns and the rows behind them.
func tableFrom(columns []column, rows [][]any) *Table {
	table := &Table{
		Columns: make([]TableColumn, 0, len(columns)),
		Rows:    make([][]Value, 0, len(rows)),
	}
	for _, col := range columns {
		table.Columns = append(table.Columns, TableColumn{Name: col.Name, Numeric: col.Numeric})
	}
	for _, row := range rows {
		table.Rows = append(table.Rows, rowValues(row, columns))
	}
	return table
}

// noRowsNote is what a document says when the query came back with nothing. An
// empty result is a normal outcome for a report, not a failure. The HTML bodies
// build it into their markup, so every format says the same thing.
const noRowsNote = "The query returned no rows."

// emptyNote is the note a block carries when there are no rows to print.
func emptyNote(rows [][]any) string {
	if len(rows) > 0 {
		return ""
	}
	return noRowsNote
}

// Documenter is implemented by templates that also describe their document
// structurally. A template that doesn't is still renderable to every file
// format — DocumentOf falls back to printing the query's own output — so
// implementing it is how a template controls its PDF and spreadsheet, not a
// condition of being rendered at all.
type Documenter interface {
	Document(Data, Config) Doc
}

// DocumentOf projects a template's document, falling back to the query output
// for a template that doesn't describe one.
func DocumentOf(t Template, data Data, cfg Config) Doc {
	if documenter, ok := t.(Documenter); ok {
		return documenter.Document(data, cfg)
	}
	return rawDocument(data)
}

// rawDocument prints every column the query returned, which is the most a
// template that hasn't described its document can be assumed to want.
func rawDocument(data Data) Doc {
	columns := data.resolve(data.Columns)
	return Doc{
		Footer:      footerText(data, ""),
		GeneratedAt: data.GeneratedAt,
		Blocks: []Block{{
			Table: tableFrom(columns, data.Rows),
			Note:  emptyNote(data.Rows),
		}},
	}
}
