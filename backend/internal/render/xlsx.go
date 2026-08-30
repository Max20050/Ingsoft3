package render

import (
	"fmt"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"github.com/Max20050/docuwave/internal/template"
)

// xlsxRenderer writes the document as an Excel workbook.
//
// It follows the same shape as the printed document — heading, blocks, tables,
// a total row — but a spreadsheet is a working file rather than a page: every
// value a template marked as a number is written as a number, so a recipient
// can sort, filter and sum the report without retyping it.
type xlsxRenderer struct{}

func (xlsxRenderer) Extension() string { return "xlsx" }

func (xlsxRenderer) ContentType() string {
	return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}

// sheetName is the single sheet a report is written to. Excel limits sheet
// names to 31 characters and forbids several punctuation marks, and a report's
// name is the user's own text, so the sheet is named for what it holds instead.
const sheetName = "Report"

const (
	// minColumnWidth keeps a narrow column readable, maxColumnWidth stops one
	// long value from pushing the rest of the table off the screen.
	minColumnWidth = 10.0
	maxColumnWidth = 60.0
)

func (xlsxRenderer) Render(doc template.Doc) ([]byte, error) {
	file := excelize.NewFile()
	defer file.Close()

	// A new workbook opens with one sheet, which becomes the report's.
	if err := file.SetSheetName(file.GetSheetName(0), sheetName); err != nil {
		return nil, err
	}

	sheet, err := newSheetStyles(file)
	if err != nil {
		return nil, err
	}
	if err := sheet.write(doc); err != nil {
		return nil, err
	}

	buf, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// sheetWriter lays a document out on a worksheet, tracking the row it's on and
// how wide each column has had to be.
type sheetWriter struct {
	file *excelize.File
	row  int
	// widths is the longest text seen in each column, which is what the column
	// is sized to once the whole document is written.
	widths map[int]int

	title    int
	subtitle int
	heading  int
	header   int
	total    int
	muted    int
}

// newSheetStyles registers the handful of styles the sheet prints in: the same
// restraint the HTML document keeps — weight and rules carry the structure, and
// no color is spent on decoration.
func newSheetStyles(file *excelize.File) (*sheetWriter, error) {
	sheet := &sheetWriter{file: file, row: 1, widths: make(map[int]int)}

	styles := []struct {
		into  *int
		style *excelize.Style
	}{
		{&sheet.title, &excelize.Style{Font: &excelize.Font{Bold: true, Size: 15}}},
		{&sheet.subtitle, &excelize.Style{Font: &excelize.Font{Color: "595959"}}},
		{&sheet.heading, &excelize.Style{Font: &excelize.Font{Bold: true, Size: 11, Color: "595959"}}},
		{&sheet.header, &excelize.Style{
			Font:   &excelize.Font{Bold: true},
			Border: []excelize.Border{{Type: "bottom", Style: 1, Color: "BFBFBF"}},
		}},
		{&sheet.total, &excelize.Style{
			Font:   &excelize.Font{Bold: true},
			Border: []excelize.Border{{Type: "top", Style: 2, Color: "808080"}},
		}},
		{&sheet.muted, &excelize.Style{Font: &excelize.Font{Italic: true, Color: "808080"}}},
	}
	for _, entry := range styles {
		id, err := file.NewStyle(entry.style)
		if err != nil {
			return nil, err
		}
		*entry.into = id
	}
	return sheet, nil
}

func (s *sheetWriter) write(doc template.Doc) error {
	if err := s.line(doc.Title, s.title); err != nil {
		return err
	}
	if err := s.line(doc.Subtitle, s.subtitle); err != nil {
		return err
	}

	for _, block := range doc.Blocks {
		s.blank()
		if err := s.line(block.Title, s.heading); err != nil {
			return err
		}
		if err := s.writeBlock(block); err != nil {
			return err
		}
		if err := s.line(block.Note, s.muted); err != nil {
			return err
		}
	}

	if doc.Footer != "" {
		s.blank()
		if err := s.line(doc.Footer, s.muted); err != nil {
			return err
		}
	}
	return s.autoFit()
}

func (s *sheetWriter) writeBlock(block template.Block) error {
	for _, tile := range block.Tiles {
		if err := s.set(1, tile.Label, s.heading); err != nil {
			return err
		}
		if err := s.setValue(2, tile.Value, s.title); err != nil {
			return err
		}
		s.row++
	}

	if block.Table == nil {
		return nil
	}

	for i, column := range block.Table.Columns {
		if err := s.set(i+1, column.Name, s.header); err != nil {
			return err
		}
	}
	s.row++

	for _, row := range block.Table.Rows {
		for i, value := range row {
			if err := s.setValue(i+1, value, 0); err != nil {
				return err
			}
		}
		s.row++
	}

	if len(block.Table.Total) > 0 {
		for i, value := range block.Table.Total {
			if err := s.setValue(i+1, value, s.total); err != nil {
				return err
			}
		}
		s.row++
	}
	return nil
}

// line writes one cell of text on its own row, skipping empty text so the
// document doesn't print blanks where a template had nothing to say. A heading
// spills across the sheet rather than sitting in a column, so its length isn't
// measured: one long title would otherwise size the first column to the whole
// screen.
func (s *sheetWriter) line(text string, style int) error {
	if text == "" {
		return nil
	}
	if err := s.put(1, text, style); err != nil {
		return err
	}
	s.row++
	return nil
}

// blank leaves a spacer row between blocks, but never above the first one.
func (s *sheetWriter) blank() {
	if s.row > 1 {
		s.row++
	}
}

// setValue writes a value in its natural type: a number the recipient can sum,
// or the text the printed document shows.
func (s *sheetWriter) setValue(column int, value template.Value, style int) error {
	if !value.IsNumber {
		return s.set(column, value.Text, style)
	}

	name, err := s.cell(column)
	if err != nil {
		return err
	}
	if err := s.file.SetCellFloat(sheetName, name, value.Number, -1, 64); err != nil {
		return err
	}
	s.measure(column, value.Text)
	return s.style(name, style)
}

// set writes text into a column of the current row and counts it towards that
// column's width.
func (s *sheetWriter) set(column int, text string, style int) error {
	if err := s.put(column, text, style); err != nil {
		return err
	}
	s.measure(column, text)
	return nil
}

// put writes text without measuring it.
func (s *sheetWriter) put(column int, text string, style int) error {
	name, err := s.cell(column)
	if err != nil {
		return err
	}
	if err := s.file.SetCellStr(sheetName, name, text); err != nil {
		return err
	}
	return s.style(name, style)
}

func (s *sheetWriter) style(cell string, style int) error {
	if style == 0 {
		return nil
	}
	return s.file.SetCellStyle(sheetName, cell, cell, style)
}

func (s *sheetWriter) cell(column int) (string, error) {
	name, err := excelize.CoordinatesToCellName(column, s.row)
	if err != nil {
		return "", fmt.Errorf("cell %d,%d: %w", column, s.row, err)
	}
	return name, nil
}

// measure remembers how wide a column's content is, in characters.
func (s *sheetWriter) measure(column int, text string) {
	if width := utf8.RuneCountInString(text); width > s.widths[column] {
		s.widths[column] = width
	}
}

// autoFit sizes each column to the widest value written into it.
func (s *sheetWriter) autoFit() error {
	for column, width := range s.widths {
		name, err := excelize.ColumnNumberToName(column)
		if err != nil {
			return err
		}
		if err := s.file.SetColWidth(sheetName, name, name, columnWidth(width)); err != nil {
			return err
		}
	}
	return nil
}

func columnWidth(characters int) float64 {
	// Excel's width unit is roughly one character of the default font, plus a
	// little padding so values don't touch the gridline.
	width := float64(characters) + 2
	if width < minColumnWidth {
		return minColumnWidth
	}
	if width > maxColumnWidth {
		return maxColumnWidth
	}
	return width
}
