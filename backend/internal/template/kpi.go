package template

// KPISummary leads with the headline numbers — each chosen measure totalled
// across the whole query — and optionally prints a supporting table underneath.
// It's the template for a report whose point is a handful of figures rather
// than the rows behind them.
type KPISummary struct{}

func (KPISummary) Meta() Meta {
	return Meta{
		ID:          "kpi-summary",
		Name:        "KPI summary",
		Description: "Headline totals as tiles, with an optional supporting table beneath them.",
		Slots: []Slot{
			{
				Key:         "title",
				Label:       "Title",
				Kind:        SlotText,
				Description: "Printed at the top of the report.",
			},
			{
				Key:         "metrics",
				Label:       "Headline metrics",
				Kind:        SlotColumns,
				Description: "Each column becomes a tile showing its total across every row.",
				Required:    true,
				Numeric:     true,
			},
			{
				Key:         "columns",
				Label:       "Supporting table columns",
				Kind:        SlotColumns,
				Description: "Optional. Columns printed as a table under the tiles.",
			},
			{
				Key:         "note",
				Label:       "Footer note",
				Kind:        SlotText,
				Description: "Optional line printed under the report, next to the row count.",
			},
		},
	}
}

type kpiView struct {
	page
	Tiles   []tile
	Columns []column
	Rows    [][]cell
}

// tile is one headline figure. Its label sits in secondary ink above the value,
// which keeps the number the loudest thing on the page.
type tile struct {
	Label string
	Value string
}

// kpiUnmappedNote is printed while no column has been mapped to a metric, which
// is the state a preview starts in.
const kpiUnmappedNote = "Map at least one column to a headline metric."

// breakdownTitle heads the optional table under the tiles.
const breakdownTitle = "Breakdown"

var kpiBody = mustDocument(`
{{if .Tiles}}
<section>
<div class="tiles">{{range .Tiles}}
<div class="tile"><p class="tile-label">{{.Label}}</p><p class="tile-value">{{.Value}}</p></div>{{end}}
</div>
</section>
{{else}}<p class="empty">` + kpiUnmappedNote + `</p>{{end}}
{{if .Columns}}
<section>
<h2 class="section-title">` + breakdownTitle + `</h2>
<table>
<thead><tr>{{range .Columns}}<th class="{{.Class}}">{{.Name}}</th>{{end}}</tr></thead>
<tbody>{{range .Rows}}<tr>{{range .}}<td class="{{.Class}}">{{.Text}}</td>{{end}}</tr>{{end}}</tbody>
</table>
{{if not .Rows}}<p class="empty">` + noRowsNote + `</p>{{end}}
</section>
{{end}}
`)

// totals sums each metric across every row, which is the figure a tile shows.
func (t KPISummary) totals(data Data, metrics []column) []float64 {
	totals := make([]float64, 0, len(metrics))
	for _, metric := range metrics {
		totals = append(totals, sum(data.Rows, metric.Index))
	}
	return totals
}

func (t KPISummary) Render(data Data, cfg Config) ([]byte, error) {
	metrics := data.resolve(cfg.ColumnsFor("metrics"))
	tiles := make([]tile, 0, len(metrics))
	for i, total := range t.totals(data, metrics) {
		tiles = append(tiles, tile{Label: metrics[i].Name, Value: formatNumber(total)})
	}

	columns := data.resolve(cfg.ColumnsFor("columns"))
	rows := make([][]cell, 0, len(data.Rows))
	for _, row := range data.Rows {
		rows = append(rows, rowCells(row, columns))
	}

	return renderDocument(kpiBody, kpiView{
		page: page{
			Title:  cfg.TextFor("title"),
			Footer: footerText(data, cfg.TextFor("note")),
		},
		Tiles:   tiles,
		Columns: columns,
		Rows:    rows,
	})
}

func (t KPISummary) Document(data Data, cfg Config) Doc {
	doc := Doc{
		Title:       cfg.TextFor("title"),
		Footer:      footerText(data, cfg.TextFor("note")),
		GeneratedAt: data.GeneratedAt,
	}

	metrics := data.resolve(cfg.ColumnsFor("metrics"))
	headline := Block{}
	for i, total := range t.totals(data, metrics) {
		headline.Tiles = append(headline.Tiles, Tile{Label: metrics[i].Name, Value: computedValue(total)})
	}
	if len(headline.Tiles) == 0 {
		headline.Note = kpiUnmappedNote
	}
	doc.Blocks = append(doc.Blocks, headline)

	// The supporting table is optional, so it only becomes a block once the user
	// has mapped columns to it.
	if columns := data.resolve(cfg.ColumnsFor("columns")); len(columns) > 0 {
		doc.Blocks = append(doc.Blocks, Block{
			Title: breakdownTitle,
			Table: tableFrom(columns, data.Rows),
			Note:  emptyNote(data.Rows),
		})
	}
	return doc
}
