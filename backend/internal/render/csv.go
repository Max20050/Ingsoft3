package render

import (
	"bytes"
	"encoding/csv"
	"strconv"

	"github.com/Max20050/docuwave/internal/template"
)

// csvRenderer writes the document as rows of text.
//
// A CSV has no title, no tiles and no styling, so the document is flattened
// onto the page it does have: the report's heading, then each block — its own
// heading, its headline figures as label/value pairs, its table with a header
// row — separated by blank lines. The ordinary case, a template with one table,
// comes out as a plain sheet of data with a couple of lines of heading above it.
//
// Numbers are written unformatted, without the thousands separators the printed
// document uses: a CSV is read by another program more often than by a person,
// and "1,234.5" is not a number to any of them.
type csvRenderer struct{}

func (csvRenderer) Extension() string { return "csv" }

// ContentType names the charset because the file starts with a byte order mark.
func (csvRenderer) ContentType() string { return "text/csv; charset=utf-8" }

// utf8BOM leads the file so that Excel reads it as UTF-8. Without it Excel
// falls back to the machine's ANSI code page and mangles every accent and
// currency symbol in the user's data.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func (csvRenderer) Render(doc template.Doc) ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(utf8BOM)

	out := csv.NewWriter(&buf)
	rows := csvRows(doc)
	if err := out.WriteAll(rows); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// csvRows lays the document out as records. Records vary in width, which is
// what flattening a document into a sheet costs.
func csvRows(doc template.Doc) [][]string {
	rows := make([][]string, 0, 8)
	heading := func(text string) {
		if text != "" {
			rows = append(rows, []string{text})
		}
	}
	blank := func() {
		if len(rows) > 0 {
			rows = append(rows, nil)
		}
	}

	heading(doc.Title)
	heading(doc.Subtitle)

	for _, block := range doc.Blocks {
		blank()
		heading(block.Title)

		for _, tile := range block.Tiles {
			rows = append(rows, []string{tile.Label, csvValue(tile.Value)})
		}

		if block.Table != nil {
			header := make([]string, 0, len(block.Table.Columns))
			for _, column := range block.Table.Columns {
				header = append(header, column.Name)
			}
			rows = append(rows, header)

			for _, row := range block.Table.Rows {
				rows = append(rows, csvValues(row))
			}
			if len(block.Table.Total) > 0 {
				rows = append(rows, csvValues(block.Table.Total))
			}
		}

		heading(block.Note)
	}

	if doc.Footer != "" {
		blank()
		heading(doc.Footer)
	}
	return rows
}

func csvValues(values []template.Value) []string {
	row := make([]string, 0, len(values))
	for _, value := range values {
		row = append(row, csvValue(value))
	}
	return row
}

func csvValue(value template.Value) string {
	if value.IsNumber {
		return strconv.FormatFloat(value.Number, 'f', -1, 64)
	}
	return value.Text
}
