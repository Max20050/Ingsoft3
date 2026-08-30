package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	htmltmpl "html/template"
	"math"
	"strconv"
	"strings"
)

// documentCSS is the shared look every template prints in: neutral ink on a
// light or dark surface, hairline rules, and no series colors — a report is a
// document, so the data is the only thing allowed to be loud. Numbers align in
// columns with tabular figures; standalone tile values keep the font's
// proportional figures so they don't read loose at display size.
const documentCSS = `
*, *::before, *::after { box-sizing: border-box; }
:root {
  color-scheme: light;
  --surface: #fcfcfb;
  --plane: #f9f9f7;
  --ink: #0b0b0b;
  --ink-2: #52514e;
  --ink-muted: #898781;
  --rule: #e1e0d9;
  --rule-strong: #c3c2b7;
}
@media (prefers-color-scheme: dark) {
  :root {
    color-scheme: dark;
    --surface: #1a1a19;
    --plane: #0d0d0d;
    --ink: #ffffff;
    --ink-2: #c3c2b7;
    --ink-muted: #898781;
    --rule: #2c2c2a;
    --rule-strong: #383835;
  }
}
body {
  margin: 0;
  padding: 32px;
  background: var(--plane);
  color: var(--ink);
  font: 14px/1.5 system-ui, -apple-system, "Segoe UI", sans-serif;
}
.doc {
  max-width: 840px;
  margin: 0 auto;
  padding: 40px;
  background: var(--surface);
  border: 1px solid var(--rule);
  border-radius: 12px;
}
.doc-head { margin-bottom: 28px; }
.doc-title { margin: 0; font-size: 24px; font-weight: 600; letter-spacing: -0.01em; }
.doc-sub { margin: 4px 0 0; color: var(--ink-2); font-size: 13px; }
.doc-foot {
  margin-top: 28px;
  padding-top: 12px;
  border-top: 1px solid var(--rule);
  color: var(--ink-muted);
  font-size: 12px;
}
section + section { margin-top: 28px; }
.section-title {
  margin: 0 0 10px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--ink-muted);
}
.empty { margin: 0; color: var(--ink-2); font-size: 13px; }
table { width: 100%; border-collapse: collapse; }
th, td { padding: 8px 12px; text-align: left; border-bottom: 1px solid var(--rule); vertical-align: top; }
thead th {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--ink-muted);
  border-bottom: 1px solid var(--rule-strong);
  white-space: nowrap;
}
tbody tr:last-child td { border-bottom: 0; }
th.num, td.num { text-align: right; font-variant-numeric: tabular-nums; }
tfoot td {
  border-top: 2px solid var(--rule-strong);
  border-bottom: 0;
  font-weight: 600;
}
.tiles { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; }
.tile { padding: 14px 16px; border: 1px solid var(--rule); border-radius: 10px; }
.tile-label { margin: 0; font-size: 12px; color: var(--ink-2); }
.tile-value { margin: 6px 0 0; font-size: 28px; font-weight: 600; line-height: 1.1; }
@page { margin: 16mm; }
@media print {
  body { padding: 0; background: #ffffff; }
  .doc { max-width: none; padding: 0; border: 0; border-radius: 0; }
  tr, .tile, section { break-inside: avoid; }
}
`

// documentHTML is the shell every template renders inside, so a report prints
// the same whichever template produced it. Each template supplies the "body".
const documentHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if .Title}}{{.Title}}{{else}}Report{{end}}</title>
<style>` + documentCSS + `</style>
</head>
<body>
<article class="doc">
{{if or .Title .Subtitle}}<header class="doc-head">
{{if .Title}}<h1 class="doc-title">{{.Title}}</h1>{{end}}
{{if .Subtitle}}<p class="doc-sub">{{.Subtitle}}</p>{{end}}
</header>{{end}}
{{template "body" .}}
{{if .Footer}}<footer class="doc-foot">{{.Footer}}</footer>{{end}}
</article>
</body>
</html>
`

var documentShell = htmltmpl.Must(htmltmpl.New("document").Parse(documentHTML))

// mustDocument clones the shared shell and parses a template's own body into it.
// It panics on a bad body, which is a compile-time-shaped mistake in a template
// literal — every call happens at package initialization.
func mustDocument(body string) *htmltmpl.Template {
	shell := htmltmpl.Must(documentShell.Clone())
	return htmltmpl.Must(shell.New("body").Parse(body))
}

// renderDocument executes a template's document, returning a complete HTML page.
func renderDocument(t *htmltmpl.Template, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "document", data); err != nil {
		return nil, fmt.Errorf("render document: %w", err)
	}
	return buf.Bytes(), nil
}

// page holds what the shared shell prints. Each template's view embeds it.
type page struct {
	Title    string
	Subtitle string
	Footer   string
}

// column is a query output column that a slot resolved to. Numeric drives
// alignment, so the templates carry no formatting logic.
type column struct {
	Name    string
	Index   int
	Numeric bool
}

// Class is the alignment class for this column's header and cells.
func (c column) Class() string {
	if c.Numeric {
		return "num"
	}
	return ""
}

// cell is one printed value.
type cell struct {
	Text  string
	Class string
}

// numericCell is a computed number — a total or a tile value — which is always
// right-aligned regardless of how the source column was typed.
func numericCell(value float64) cell {
	return cell{Text: formatNumber(value), Class: "num"}
}

// index finds a column's position in the query output.
func (d Data) index(name string) (int, bool) {
	for i, column := range d.Columns {
		if column == name {
			return i, true
		}
	}
	return 0, false
}

// resolve pairs mapped column names with their position in the query output.
// Names the query doesn't return are dropped rather than failing the render:
// Validate is the gate that reports them, and a preview is more useful than an
// error when a mapping goes stale.
func (d Data) resolve(names []string) []column {
	resolved := make([]column, 0, len(names))
	for _, name := range names {
		index, ok := d.index(name)
		if !ok {
			continue
		}
		resolved = append(resolved, column{Name: name, Index: index, Numeric: d.numeric(index)})
	}
	return resolved
}

// numeric reports whether a column holds numbers, which is what decides
// alignment. Every value present has to read as a number: one label in the
// column and it's text.
func (d Data) numeric(index int) bool {
	found := false
	for _, row := range d.Rows {
		value := valueAt(row, index)
		if isBlank(value) {
			continue
		}
		if _, ok := toNumber(value); !ok {
			return false
		}
		found = true
	}
	return found
}

// rowCells renders one row's cells for the given columns.
func rowCells(row []any, columns []column) []cell {
	cells := make([]cell, 0, len(columns))
	for _, column := range columns {
		cells = append(cells, cell{Text: formatValue(valueAt(row, column.Index)), Class: column.Class()})
	}
	return cells
}

// sum totals a column across the given rows, ignoring values that aren't
// numbers so one stray label doesn't void the total.
func sum(rows [][]any, index int) float64 {
	var total float64
	for _, row := range rows {
		if number, ok := toNumber(valueAt(row, index)); ok {
			total += number
		}
	}
	return total
}

// valueAt reads a cell, tolerating short rows.
func valueAt(row []any, index int) any {
	if index < 0 || index >= len(row) {
		return nil
	}
	return row[index]
}

func isBlank(value any) bool {
	if value == nil {
		return true
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) == ""
}

// formatValue prints a value the way a report reader expects. Values arrive
// straight from a data source, so a spreadsheet's "1,234.5" stays as typed
// while a driver's float64 gets grouped.
func formatValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "—"
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		if number, err := typed.Float64(); err == nil {
			return formatNumber(number)
		}
		return typed.String()
	default:
		if number, ok := toNumber(value); ok {
			return formatNumber(number)
		}
		return fmt.Sprint(value)
	}
}

// formatNumber groups thousands and keeps decimals only when the value has
// them, rounded to two places.
func formatNumber(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "—"
	}
	rounded := math.Round(value*100) / 100
	digits := 2
	if rounded == math.Trunc(rounded) {
		digits = 0
	}
	return groupThousands(strconv.FormatFloat(rounded, 'f', digits, 64))
}

// groupThousands inserts thousands separators into a plain decimal string.
func groupThousands(number string) string {
	sign := ""
	if strings.HasPrefix(number, "-") {
		sign, number = "-", number[1:]
	}
	whole, fraction := number, ""
	if dot := strings.IndexByte(number, '.'); dot >= 0 {
		whole, fraction = number[:dot], number[dot:]
	}

	var grouped strings.Builder
	for i, digit := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}
	return sign + grouped.String() + fraction
}

// toNumber coerces a data source value into a number for totalling. Numbers
// reach us as driver types, as JSON floats, or — from spreadsheets — as text
// like "$1,234.50", so text is parsed after stripping the formatting a human
// typed.
func toNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case string:
		return parseNumber(typed)
	default:
		return 0, false
	}
}

func parseNumber(text string) (float64, bool) {
	cleaned := strings.TrimSpace(text)
	cleaned = strings.TrimLeft(cleaned, "$€£")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	if cleaned == "" {
		return 0, false
	}
	number, err := strconv.ParseFloat(cleaned, 64)
	return number, err == nil
}

// rowLabel prints a row count for the document footer.
func rowLabel(count int) string {
	if count == 1 {
		return "1 row"
	}
	return fmt.Sprintf("%s rows", groupThousands(strconv.Itoa(count)))
}

// footerText assembles the footer: how much data the document covers, when it
// was generated, and whatever note the user added.
func footerText(data Data, note string) string {
	parts := []string{rowLabel(len(data.Rows))}
	if !data.GeneratedAt.IsZero() {
		parts = append(parts, "generated "+data.GeneratedAt.UTC().Format("2 Jan 2006 15:04 MST"))
	}
	if note != "" {
		parts = append(parts, note)
	}
	return strings.Join(parts, " · ")
}
