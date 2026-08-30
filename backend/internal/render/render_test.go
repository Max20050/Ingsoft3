package render

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/Max20050/docuwave/internal/template"
)

// sampleDoc is a document with everything a template can project: a heading,
// headline figures, a table of mixed text and numbers, and a total row.
func sampleDoc() template.Doc {
	return template.Doc{
		Title:       "Monthly sales",
		Subtitle:    "Grouped by region",
		Footer:      "3 rows · generated 12 Aug 2026 09:30 UTC",
		GeneratedAt: time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC),
		Blocks: []template.Block{
			{
				Tiles: []template.Tile{
					{Label: "revenue", Value: template.Value{Text: "12,500.5", Number: 12500.5, IsNumber: true}},
				},
			},
			{
				Title: "Breakdown",
				Table: &template.Table{
					Columns: []template.TableColumn{
						{Name: "region"},
						{Name: "revenue", Numeric: true},
					},
					Rows: [][]template.Value{
						{
							{Text: "North"},
							{Text: "9,000", Number: 9000, IsNumber: true},
						},
						{
							{Text: "South"},
							{Text: "3,500.5", Number: 3500.5, IsNumber: true},
						},
					},
					Total: []template.Value{
						{Text: "Total"},
						{Text: "12,500.5", Number: 12500.5, IsNumber: true},
					},
				},
			},
		},
	}
}

// Every format has to come back as a file the recipient's software recognises,
// which for all three is decided by the first few bytes.
func TestRenderProducesEachFormat(t *testing.T) {
	tests := []struct {
		format      Format
		wantName    string
		wantType    string
		wantPrefix  []byte
		wantMinSize int
	}{
		{FormatPDF, "monthly-sales-2026-08-12.pdf", "application/pdf", []byte("%PDF-"), 800},
		{
			format:   FormatXLSX,
			wantName: "monthly-sales-2026-08-12.xlsx",
			wantType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			// An .xlsx is a zip of XML parts.
			wantPrefix:  []byte("PK\x03\x04"),
			wantMinSize: 800,
		},
		{FormatCSV, "monthly-sales-2026-08-12.csv", "text/csv; charset=utf-8", utf8BOM, 40},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			artifact, err := Render(sampleDoc(), tt.format, "Monthly sales")
			if err != nil {
				t.Fatalf("Render returned error: %v", err)
			}

			if artifact.Filename != tt.wantName {
				t.Errorf("got filename %q, want %q", artifact.Filename, tt.wantName)
			}
			if artifact.ContentType != tt.wantType {
				t.Errorf("got content type %q, want %q", artifact.ContentType, tt.wantType)
			}
			if !bytes.HasPrefix(artifact.Bytes, tt.wantPrefix) {
				t.Errorf("file does not start with %q", tt.wantPrefix)
			}
			if len(artifact.Bytes) < tt.wantMinSize {
				t.Errorf("got %d bytes, want at least %d — the document looks empty",
					len(artifact.Bytes), tt.wantMinSize)
			}
		})
	}
}

// A PDF is drawn rather than described, so what's checked is that a document
// with more rows than fit on a page becomes more pages.
func TestPDFPaginatesLongReports(t *testing.T) {
	doc := sampleDoc()
	rows := make([][]template.Value, 0, 200)
	for range 200 {
		rows = append(rows, []template.Value{
			{Text: "North"},
			{Text: "1", Number: 1, IsNumber: true},
		})
	}
	doc.Blocks[1].Table.Rows = rows

	artifact, err := Render(doc, FormatPDF, "Long report")
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	// Each page is its own object in the page tree.
	if pages := bytes.Count(artifact.Bytes, []byte("/Type /Page\n")); pages < 4 {
		t.Errorf("got %d pages for 200 rows, want at least 4", pages)
	}
}

// The spreadsheet is the format a recipient works in, so a value the document
// prints as a number has to arrive as one — text that merely looks numeric
// can't be summed.
func TestXLSXWritesNumbersAsNumbers(t *testing.T) {
	artifact, err := Render(sampleDoc(), FormatXLSX, "Monthly sales")
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	file, err := excelize.OpenReader(bytes.NewReader(artifact.Bytes))
	if err != nil {
		t.Fatalf("the workbook does not open: %v", err)
	}
	defer file.Close()

	rows, err := file.GetRows(sheetName)
	if err != nil {
		t.Fatalf("GetRows returned error: %v", err)
	}
	if got := rows[0][0]; got != "Monthly sales" {
		t.Errorf("got %q in the first cell, want the report's title", got)
	}

	flat := make([]string, 0, len(rows))
	for _, row := range rows {
		flat = append(flat, strings.Join(row, "|"))
	}
	sheet := strings.Join(flat, "\n")
	for _, want := range []string{"Breakdown", "region|revenue", "North|9000", "Total|12500.5"} {
		if !strings.Contains(sheet, want) {
			t.Errorf("the sheet does not contain %q:\n%s", want, sheet)
		}
	}

	// B8 and B9 are the two regions' revenue, under the "region | revenue"
	// header. Summing them is the thing a recipient does with a spreadsheet, and
	// it only works if the cells hold numbers rather than text that looks like
	// one — which is the whole reason a Value carries its number.
	if err := file.SetCellFormula(sheetName, "D1", "SUM(B8:B9)"); err != nil {
		t.Fatalf("SetCellFormula returned error: %v", err)
	}
	total, err := file.CalcCellValue(sheetName, "D1")
	if err != nil {
		t.Fatalf("CalcCellValue returned error: %v", err)
	}
	if total != "12500.5" {
		t.Errorf("got %q summing the revenue column, want 12500.5", total)
	}
}

// A CSV is read by another program more often than by a person: it has to parse
// as CSV, and its numbers have to parse as numbers.
func TestCSVIsParseableAndUnformatted(t *testing.T) {
	artifact, err := Render(sampleDoc(), FormatCSV, "Monthly sales")
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	body := bytes.TrimPrefix(artifact.Bytes, utf8BOM)
	reader := csv.NewReader(bytes.NewReader(body))
	// The document is flattened onto one sheet, so its records vary in width.
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("the file does not parse as CSV: %v", err)
	}

	if got := records[0][0]; got != "Monthly sales" {
		t.Errorf("got %q as the first record, want the report's title", got)
	}

	joined := make([]string, 0, len(records))
	for _, record := range records {
		joined = append(joined, strings.Join(record, ","))
	}
	text := strings.Join(joined, "\n")
	for _, want := range []string{"revenue,12500.5", "region,revenue", "North,9000", "Total,12500.5"} {
		if !strings.Contains(text, want) {
			t.Errorf("the file does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "12,500.5") {
		t.Error("numbers are written with thousands separators, which nothing can parse")
	}
}

// An empty result is a normal outcome for a report, and every format still has
// to produce a file that says so.
func TestRenderEmptyDocument(t *testing.T) {
	doc := template.Doc{
		Title:  "Nothing to report",
		Footer: "0 rows",
		Blocks: []template.Block{{
			Table: &template.Table{Columns: []template.TableColumn{{Name: "region"}}},
			Note:  "The query returned no rows.",
		}},
	}

	for _, format := range Formats() {
		t.Run(string(format), func(t *testing.T) {
			artifact, err := Render(doc, format, "Nothing to report")
			if err != nil {
				t.Fatalf("Render returned error: %v", err)
			}
			if len(artifact.Bytes) == 0 {
				t.Error("got an empty file")
			}
		})
	}
}

func TestRenderAllProducesEveryConfiguredFormat(t *testing.T) {
	artifacts, err := RenderAll(sampleDoc(), []Format{FormatCSV, FormatPDF}, "Monthly sales")
	if err != nil {
		t.Fatalf("RenderAll returned error: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2", len(artifacts))
	}
	if artifacts[0].Format != FormatCSV || artifacts[1].Format != FormatPDF {
		t.Errorf("got %v, want the formats in the order asked for", artifacts)
	}

	if _, err := RenderAll(sampleDoc(), nil, "Monthly sales"); !errors.Is(err, ErrNoFormats) {
		t.Errorf("got %v for a report with no formats, want ErrNoFormats", err)
	}
}

func TestParseFormats(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []Format
	}{
		{"one format", []string{"csv"}, []Format{FormatCSV}},
		{"stored in registry order", []string{"csv", "pdf"}, []Format{FormatPDF, FormatCSV}},
		{"duplicates collapse", []string{"pdf", "PDF", " pdf "}, []Format{FormatPDF}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFormats(tt.input)
			if err != nil {
				t.Fatalf("ParseFormats returned error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}

	if _, err := ParseFormats([]string{"docx"}); !errors.Is(err, ErrUnknownFormat) {
		t.Errorf("got %v for an unsupported format, want ErrUnknownFormat", err)
	}
	// The name the caller asked for is what tells them what to fix.
	if _, err := ParseFormats([]string{"docx"}); err == nil || !strings.Contains(err.Error(), "docx") {
		t.Errorf("got %v, want an error naming the format", err)
	}
	if _, err := ParseFormats(nil); !errors.Is(err, ErrNoFormats) {
		t.Errorf("got %v for no formats, want ErrNoFormats", err)
	}
}

// A report's name reaches a filesystem, a Content-Disposition header and a mail
// client, so it is reduced to a slug rather than passed through.
func TestFilename(t *testing.T) {
	at := time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC)

	tests := []struct {
		name   string
		report string
		want   string
	}{
		{"words become a slug", "Monthly Sales — EU", "monthly-sales-eu-2026-08-12.pdf"},
		{"punctuation cannot escape the name", `../../etc/passwd`, "etc-passwd-2026-08-12.pdf"},
		{"a nameless report still gets a file", "", "report-2026-08-12.pdf"},
		{"a name of punctuation still gets a file", "///", "report-2026-08-12.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Filename(tt.report, at, "pdf"); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}

	// A long name would otherwise run past what a mail client shows.
	long := Filename(strings.Repeat("sales report ", 20), at, "csv")
	if len(long) > maxSlugLength+len("-2026-08-12.csv") {
		t.Errorf("got a %d character filename: %s", len(long), long)
	}
}
