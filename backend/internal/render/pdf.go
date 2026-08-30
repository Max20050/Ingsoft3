package render

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"

	"github.com/Max20050/docuwave/internal/template"
)

// pdfRenderer prints the document as a paginated A4 document.
//
// It draws the blocks a template projected rather than converting that
// template's HTML, because printing HTML faithfully means shipping a browser:
// a headless Chrome alongside the API is a large, sandboxed, frequently
// patched dependency for a server whose job is to email a few pages of tables.
// Drawing the same structure keeps the PDF true to the document without one —
// the numbers, the order, the grouping and the totals are the template's, and
// only the styling is this file's.
type pdfRenderer struct{}

func (pdfRenderer) Extension() string { return "pdf" }

func (pdfRenderer) ContentType() string { return "application/pdf" }

const (
	// Page geometry, in millimetres on A4.
	pageWidth        = 210.0
	pageHeight       = 297.0
	pageMarginX      = 15.0
	pageMarginTop    = 18.0
	pageMarginBottom = 20.0
	contentWidth     = pageWidth - 2*pageMarginX

	// fontFamily is one of the PDF core fonts, so nothing has to be embedded
	// and the file stays small.
	fontFamily = "Helvetica"

	tableFontSize = 9.0
	rowHeight     = 6.0
	headerHeight  = 7.0
	cellPadding   = 2.5
	minColumnMM   = 14.0

	tileHeight  = 18.0
	tileGap     = 4.0
	tilesPerRow = 3

	// widthSampleRows caps how many rows are measured to size a table's
	// columns. A report can carry thousands, and the first few hundred say
	// everything about how wide a column needs to be.
	widthSampleRows = 200
)

// rgb is a printed colour. The document prints in neutral ink with hairline
// rules and no series colour, exactly as the HTML does: in a report the data is
// the only thing allowed to be loud.
type rgb struct{ r, g, b int }

var (
	inkColor        = rgb{20, 20, 20}
	mutedColor      = rgb{110, 110, 110}
	ruleColor       = rgb{205, 205, 205}
	strongRuleColor = rgb{140, 140, 140}
)

func (p pdfRenderer) Render(doc template.Doc) ([]byte, error) {
	page := newPDFWriter(doc)
	page.write(doc)
	return page.bytes()
}

// pdfWriter draws a document onto a page, keeping the text encoder every string
// has to go through.
type pdfWriter struct {
	pdf *fpdf.Fpdf
	// encode maps UTF-8 onto the code page the core fonts use. Report content is
	// the user's own data, so it arrives as UTF-8 and every string printed here
	// goes through it.
	encode func(string) string
}

func newPDFWriter(doc template.Doc) *pdfWriter {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMarginX, pageMarginTop, pageMarginX)
	pdf.SetAutoPageBreak(true, pageMarginBottom)

	writer := &pdfWriter{pdf: pdf, encode: pdf.UnicodeTranslatorFromDescriptor("")}
	writer.footer(doc.Footer)
	// The page count isn't known until the document is finished, so the footer
	// prints a placeholder that fpdf substitutes on output.
	pdf.AliasNbPages("")
	pdf.AddPage()
	return writer
}

// footer prints the document's footer and the page number on every page.
func (p *pdfWriter) footer(text string) {
	p.pdf.SetFooterFunc(func() {
		p.pdf.SetY(-15)
		p.rule(ruleColor, 0.1, p.pdf.GetY())

		p.pdf.SetFont(fontFamily, "", 8)
		p.ink(mutedColor)
		p.pdf.SetY(p.pdf.GetY() + 1)

		const pageLabelWidth = 40.0
		p.pdf.CellFormat(contentWidth-pageLabelWidth, 6,
			p.fit(text, contentWidth-pageLabelWidth), "", 0, "L", false, 0, "")
		p.pdf.CellFormat(pageLabelWidth, 6,
			p.encode(fmt.Sprintf("Page %d of {nb}", p.pdf.PageNo())), "", 0, "R", false, 0, "")
	})
}

func (p *pdfWriter) write(doc template.Doc) {
	p.heading(doc)
	for i, block := range doc.Blocks {
		if i > 0 {
			p.pdf.Ln(6)
		}
		p.block(block)
	}
}

func (p *pdfWriter) heading(doc template.Doc) {
	if doc.Title != "" {
		p.pdf.SetFont(fontFamily, "B", 17)
		p.ink(inkColor)
		p.pdf.MultiCell(contentWidth, 8, p.encode(doc.Title), "", "L", false)
	}
	if doc.Subtitle != "" {
		p.pdf.SetFont(fontFamily, "", 10)
		p.ink(mutedColor)
		p.pdf.MultiCell(contentWidth, 5, p.encode(doc.Subtitle), "", "L", false)
	}
	if doc.Title != "" || doc.Subtitle != "" {
		p.pdf.Ln(4)
	}
}

func (p *pdfWriter) block(block template.Block) {
	if block.Title != "" {
		p.pdf.SetFont(fontFamily, "B", 9)
		p.ink(mutedColor)
		p.pdf.CellFormat(contentWidth, 6, p.encode(strings.ToUpper(block.Title)), "", 1, "L", false, 0, "")
	}
	if len(block.Tiles) > 0 {
		p.tiles(block.Tiles)
	}
	if block.Table != nil {
		p.table(block.Table)
	}
	if block.Note != "" {
		p.pdf.SetFont(fontFamily, "I", 9)
		p.ink(mutedColor)
		p.pdf.MultiCell(contentWidth, 5, p.encode(block.Note), "", "L", false)
	}
}

// tiles prints headline figures as a row of boxes, wrapping onto further rows.
func (p *pdfWriter) tiles(tiles []template.Tile) {
	width := (contentWidth - tileGap*(tilesPerRow-1)) / tilesPerRow
	y := p.pdf.GetY()

	for i, tile := range tiles {
		column := i % tilesPerRow
		if i > 0 && column == 0 {
			y += tileHeight + tileGap
		}
		if column == 0 && y+tileHeight > p.pageLimit() {
			p.pdf.AddPage()
			y = p.pdf.GetY()
		}
		p.tile(tile, pageMarginX+float64(column)*(width+tileGap), y, width)
	}

	p.pdf.SetXY(pageMarginX, y+tileHeight+2)
}

func (p *pdfWriter) tile(tile template.Tile, x, y, width float64) {
	inner := width - 2*cellPadding

	p.pdf.SetDrawColor(ruleColor.r, ruleColor.g, ruleColor.b)
	p.pdf.SetLineWidth(0.2)
	p.pdf.Rect(x, y, width, tileHeight, "D")

	p.pdf.SetFont(fontFamily, "", 8)
	p.ink(mutedColor)
	p.pdf.SetXY(x+cellPadding, y+3)
	p.pdf.CellFormat(inner, 4, p.fit(tile.Label, inner), "", 0, "L", false, 0, "")

	p.pdf.SetFont(fontFamily, "B", 14)
	p.ink(inkColor)
	p.pdf.SetXY(x+cellPadding, y+8)
	p.pdf.CellFormat(inner, 8, p.fit(tile.Value.Text, inner), "", 0, "L", false, 0, "")
}

// table prints a block's rows, repeating the header on every page it runs onto.
func (p *pdfWriter) table(table *template.Table) {
	if len(table.Columns) == 0 {
		return
	}

	widths := p.columnWidths(table)
	p.tableHeader(table.Columns, widths)

	for _, row := range table.Rows {
		if p.pdf.GetY()+rowHeight > p.pageLimit() {
			p.pdf.AddPage()
			p.tableHeader(table.Columns, widths)
		}
		p.tableRow(row, table.Columns, widths, false)
	}

	if len(table.Total) > 0 {
		if p.pdf.GetY()+rowHeight > p.pageLimit() {
			p.pdf.AddPage()
			p.tableHeader(table.Columns, widths)
		}
		p.rule(strongRuleColor, 0.4, p.pdf.GetY())
		p.tableRow(table.Total, table.Columns, widths, true)
	}
}

func (p *pdfWriter) tableHeader(columns []template.TableColumn, widths []float64) {
	p.pdf.SetFont(fontFamily, "B", 8)
	p.ink(mutedColor)

	y := p.pdf.GetY()
	p.pdf.SetXY(pageMarginX, y)
	for i, column := range columns {
		p.pdf.CellFormat(widths[i], headerHeight,
			p.fit(strings.ToUpper(column.Name), widths[i]-2*cellPadding), "", 0,
			alignFor(column), false, 0, "")
	}

	p.rule(strongRuleColor, 0.2, y+headerHeight)
	p.pdf.SetXY(pageMarginX, y+headerHeight)
}

func (p *pdfWriter) tableRow(row []template.Value, columns []template.TableColumn, widths []float64, total bool) {
	style := ""
	if total {
		style = "B"
	}
	p.pdf.SetFont(fontFamily, style, tableFontSize)
	p.ink(inkColor)

	y := p.pdf.GetY()
	p.pdf.SetXY(pageMarginX, y)
	for i, column := range columns {
		text := ""
		if i < len(row) {
			text = row[i].Text
		}
		p.pdf.CellFormat(widths[i], rowHeight, p.fit(text, widths[i]-2*cellPadding), "", 0,
			alignFor(column), false, 0, "")
	}

	if !total {
		p.rule(ruleColor, 0.1, y+rowHeight)
	}
	p.pdf.SetXY(pageMarginX, y+rowHeight)
}

// alignFor right-aligns numbers, which is what makes a column of them readable.
func alignFor(column template.TableColumn) string {
	if column.Numeric {
		return "R"
	}
	return "L"
}

// columnWidths sizes a table's columns from their content and then scales them
// to the page, so a table always spans the text column exactly as it does in
// the HTML document.
func (p *pdfWriter) columnWidths(table *template.Table) []float64 {
	widths := make([]float64, len(table.Columns))

	p.pdf.SetFont(fontFamily, "B", 8)
	for i, column := range table.Columns {
		widths[i] = p.pdf.GetStringWidth(p.encode(strings.ToUpper(column.Name)))
	}

	p.pdf.SetFont(fontFamily, "", tableFontSize)
	measure := func(row []template.Value) {
		for i, value := range row {
			if i >= len(widths) {
				return
			}
			if width := p.pdf.GetStringWidth(p.encode(value.Text)); width > widths[i] {
				widths[i] = width
			}
		}
	}
	for i, row := range table.Rows {
		if i >= widthSampleRows {
			break
		}
		measure(row)
	}
	measure(table.Total)

	natural := 0.0
	for i := range widths {
		widths[i] += 2 * cellPadding
		if widths[i] < minColumnMM {
			widths[i] = minColumnMM
		}
		natural += widths[i]
	}

	scale := contentWidth / natural
	for i := range widths {
		widths[i] *= scale
	}
	return widths
}

// pageLimit is the lowest a row may start on a page.
func (p *pdfWriter) pageLimit() float64 { return pageHeight - pageMarginBottom }

// rule draws a hairline across the text column.
func (p *pdfWriter) rule(color rgb, weight, y float64) {
	p.pdf.SetDrawColor(color.r, color.g, color.b)
	p.pdf.SetLineWidth(weight)
	p.pdf.Line(pageMarginX, y, pageMarginX+contentWidth, y)
}

func (p *pdfWriter) ink(color rgb) { p.pdf.SetTextColor(color.r, color.g, color.b) }

// fit encodes text for printing, shortening it to the given width when it
// doesn't fit. The result is already encoded, so callers pass it straight to
// the page.
func (p *pdfWriter) fit(text string, width float64) string {
	encoded := p.encode(text)
	if p.pdf.GetStringWidth(encoded) <= width {
		return encoded
	}

	ellipsis := p.encode("…")
	runes := []rune(text)
	for n := len(runes) - 1; n > 0; n-- {
		candidate := p.encode(strings.TrimRight(string(runes[:n]), " ")) + ellipsis
		if p.pdf.GetStringWidth(candidate) <= width {
			return candidate
		}
	}
	return ellipsis
}

func (p *pdfWriter) bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := p.pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
