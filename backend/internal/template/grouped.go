package template

import (
	"strconv"
)

// GroupedTotals collapses the query's rows into one line per distinct value of
// the grouping column, totalling the chosen measures within each group and
// again across the whole report. Groups print in the order they first appear in
// the query output, so a query's own ORDER BY still decides the arrangement.
type GroupedTotals struct{}

func (GroupedTotals) Meta() Meta {
	return Meta{
		ID:          "grouped-totals",
		Name:        "Grouped summary with totals",
		Description: "One row per group with the measures you pick totalled per group and overall.",
		Slots: []Slot{
			{
				Key:         "title",
				Label:       "Title",
				Kind:        SlotText,
				Description: "Printed at the top of the report.",
			},
			{
				Key:         "group",
				Label:       "Group rows by",
				Kind:        SlotColumn,
				Description: "The column whose distinct values become the report's rows.",
				Required:    true,
			},
			{
				Key:         "metrics",
				Label:       "Columns to total",
				Kind:        SlotColumns,
				Description: "Numeric columns summed within each group and across the report.",
				Required:    true,
				Numeric:     true,
			},
			{
				Key:         "note",
				Label:       "Footer note",
				Kind:        SlotText,
				Description: "Optional line printed under the table, next to the row count.",
			},
		},
	}
}

type groupedView struct {
	page
	GroupLabel string
	Metrics    []column
	Groups     []groupedRow
	Totals     []cell
	TotalRows  string
}

type groupedRow struct {
	Label  string
	Rows   string
	Totals []cell
}

// groupedUnmappedNote is printed until the mapping names both a grouping column
// and something to total, which is the state a preview starts in.
const groupedUnmappedNote = "Map a grouping column and at least one column to total."

// rowsColumn heads the count of query rows behind each group.
const rowsColumn = "Rows"

// totalLabel heads the summary row printed under the groups.
const totalLabel = "Total"

var groupedBody = mustDocument(`
{{if and .GroupLabel .Metrics}}
<section>
<table>
<thead><tr>
<th>{{.GroupLabel}}</th>
<th class="num">` + rowsColumn + `</th>
{{range .Metrics}}<th class="num">{{.Name}}</th>{{end}}
</tr></thead>
<tbody>{{range .Groups}}<tr>
<td>{{.Label}}</td>
<td class="num">{{.Rows}}</td>
{{range .Totals}}<td class="{{.Class}}">{{.Text}}</td>{{end}}
</tr>{{end}}</tbody>
{{if .Groups}}<tfoot><tr>
<td>` + totalLabel + `</td>
<td class="num">{{.TotalRows}}</td>
{{range .Totals}}<td class="{{.Class}}">{{.Text}}</td>{{end}}
</tr></tfoot>{{end}}
</table>
{{if not .Groups}}<p class="empty">` + noRowsNote + `</p>{{end}}
</section>
{{else}}<p class="empty">` + groupedUnmappedNote + `</p>{{end}}
`)

// grouping is the arrangement this template prints: the column the rows were
// bucketed by, the measures totalled within each bucket, and the buckets
// themselves. It's computed once and projected into HTML or into a Doc, so a
// report's page and its spreadsheet can't disagree about the groups.
type grouping struct {
	group   column
	metrics []column
	buckets []groupBucket
}

// groupBucket is one group: its printed label and the query rows behind it.
type groupBucket struct {
	label string
	rows  [][]any
}

// total sums a measure across a bucket.
func (b groupBucket) total(metric column) float64 { return sum(b.rows, metric.Index) }

// group buckets the query's rows by the mapped column. It reports false when
// the mapping doesn't yet name both a grouping column and a measure, which is
// the state a preview starts in.
func (t GroupedTotals) group(data Data, cfg Config) (grouping, bool) {
	groupColumns := data.resolve([]string{cfg.ColumnFor("group")})
	metrics := data.resolve(cfg.ColumnsFor("metrics"))
	if len(groupColumns) == 0 || len(metrics) == 0 {
		return grouping{}, false
	}

	result := grouping{group: groupColumns[0], metrics: metrics}

	// Rows are bucketed by their printed group value, keeping first-appearance
	// order: the query decided the arrangement, and this shouldn't re-sort it.
	at := make(map[string]int)
	for _, row := range data.Rows {
		label := formatValue(valueAt(row, result.group.Index))
		index, seen := at[label]
		if !seen {
			index = len(result.buckets)
			at[label] = index
			result.buckets = append(result.buckets, groupBucket{label: label})
		}
		result.buckets[index].rows = append(result.buckets[index].rows, row)
	}

	return result, true
}

func (t GroupedTotals) Render(data Data, cfg Config) ([]byte, error) {
	view := groupedView{
		page:      page{Title: cfg.TextFor("title"), Footer: footerText(data, cfg.TextFor("note"))},
		TotalRows: groupThousands(strconv.Itoa(len(data.Rows))),
	}

	grouped, ok := t.group(data, cfg)
	if !ok {
		return renderDocument(groupedBody, view)
	}

	view.GroupLabel = grouped.group.Name
	view.Metrics = grouped.metrics
	view.Subtitle = "Grouped by " + grouped.group.Name

	for _, bucket := range grouped.buckets {
		totals := make([]cell, 0, len(grouped.metrics))
		for _, metric := range grouped.metrics {
			totals = append(totals, numericCell(bucket.total(metric)))
		}
		view.Groups = append(view.Groups, groupedRow{
			Label:  bucket.label,
			Rows:   groupThousands(strconv.Itoa(len(bucket.rows))),
			Totals: totals,
		})
	}

	for _, metric := range grouped.metrics {
		view.Totals = append(view.Totals, numericCell(sum(data.Rows, metric.Index)))
	}

	return renderDocument(groupedBody, view)
}

func (t GroupedTotals) Document(data Data, cfg Config) Doc {
	doc := Doc{
		Title:       cfg.TextFor("title"),
		Footer:      footerText(data, cfg.TextFor("note")),
		GeneratedAt: data.GeneratedAt,
	}

	grouped, ok := t.group(data, cfg)
	if !ok {
		doc.Blocks = []Block{{Note: groupedUnmappedNote}}
		return doc
	}
	doc.Subtitle = "Grouped by " + grouped.group.Name

	// The printed table is one row per group, not one per query row: the group
	// label, how many rows it covers, then each measure's total.
	table := &Table{Columns: []TableColumn{
		{Name: grouped.group.Name},
		{Name: rowsColumn, Numeric: true},
	}}
	for _, metric := range grouped.metrics {
		table.Columns = append(table.Columns, TableColumn{Name: metric.Name, Numeric: true})
	}

	for _, bucket := range grouped.buckets {
		row := []Value{textValue(bucket.label), computedValue(float64(len(bucket.rows)))}
		for _, metric := range grouped.metrics {
			row = append(row, computedValue(bucket.total(metric)))
		}
		table.Rows = append(table.Rows, row)
	}

	if len(table.Rows) > 0 {
		table.Total = []Value{textValue(totalLabel), computedValue(float64(len(data.Rows)))}
		for _, metric := range grouped.metrics {
			table.Total = append(table.Total, computedValue(sum(data.Rows, metric.Index)))
		}
	}

	doc.Blocks = []Block{{Table: table, Note: emptyNote(data.Rows)}}
	return doc
}
