package template

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"strings"

	"github.com/Max20050/docuwave/internal/query"
)

// BlockKind names a type in the block catalog a custom design is composed
// from. New kinds (a chart, an image) are added by extending this catalog and
// blockSlots/blockDoc below — the composition engine, the ordering UI, and the
// rendering pipeline don't change.
type BlockKind string

const (
	// BlockTable prints the query's rows as a table of chosen columns, in the
	// order they were chosen — the same shape as the Tabular starter.
	BlockTable BlockKind = "table"
	// BlockGrouped collapses rows into one line per distinct value of a
	// grouping column, totalling the chosen measures — the same aggregation
	// the GroupedTotals starter uses.
	BlockGrouped BlockKind = "grouped-table"
	// BlockKPI prints each chosen measure as a headline tile, totalled across
	// every row — the same aggregation the KPISummary starter uses.
	BlockKPI BlockKind = "kpi-tiles"
	// BlockText is a freeform title/note with no data binding, for separating
	// parts of a report.
	BlockText BlockKind = "text"
	// BlockAISummary prints text an LLM generates from a prompt the user
	// writes, given the columns of the report's own (already filtered) query
	// output the user chooses to share, plus whatever additional queries the
	// block defines against the same data source — run only to feed the model,
	// never shown in the document themselves.
	BlockAISummary BlockKind = "ai-summary"
)

// isKnownBlockKind reports whether kind is in the catalog. It's the one place
// that would need extending for a new block type — blockSlots and blockDoc
// have their own switches to extend alongside it, but the composition engine
// and everything upstream of it stay untouched.
func isKnownBlockKind(kind BlockKind) bool {
	switch kind {
	case BlockTable, BlockGrouped, BlockKPI, BlockText, BlockAISummary:
		return true
	default:
		return false
	}
}

// BlockDef is one block in a composed custom template: its type, its
// user-facing title — which also doubles as the block's identifier when a
// design has more than one block of the same type — and, for a text block,
// the freeform note it prints. A block's data-bound slots (which columns feed
// its table, its grouping, its metrics) aren't part of the definition: they're
// mapped per report, the same way a starter template's slots are, so the same
// saved design can be reused against different query columns.
type BlockDef struct {
	// ID identifies the block across edits, independent of its position in the
	// list — reordering never changes it. It's what a block's slot keys are
	// namespaced by, so an edit that only reorders or renames blocks doesn't
	// strand a report's existing column mapping.
	ID    string    `json:"id"`
	Kind  BlockKind `json:"kind"`
	Title string    `json:"title"`
	// Note is the freeform text a "text" block prints. Every other kind
	// ignores it.
	Note string `json:"note,omitempty"`
	// Queries are additional queries an "ai-summary" block runs against the
	// report's own data source to gather extra context for the model. Their
	// results are never printed in the document — only the report's own query
	// output and this block's chosen columns are. Every other kind ignores it.
	Queries []AISummaryQuery `json:"queries,omitempty"`
}

// AISummaryQuery is one additional query an "ai-summary" block runs purely to
// gather context for its prompt. Title labels it for the model (and for the
// person configuring the block); Spec is compiled and run the same way a
// report's own query is, against the same data source.
type AISummaryQuery struct {
	Title string     `json:"title"`
	Spec  query.Spec `json:"spec"`
}

// newBlockID generates a short random identifier for a block that doesn't
// have one yet.
func newBlockID() string {
	raw := make([]byte, 8)
	// crypto/rand.Read on the standard reader never returns an error worth
	// handling — a zeroed buffer just yields a collision newBlockID's caller
	// already retries on.
	_, _ = rand.Read(raw)
	return "b_" + hex.EncodeToString(raw)
}

// PrepareBlocks assigns a stable ID to any block that arrives without one —
// preserving the ID of a block that already has one, so editing a saved
// template's title, note, or order doesn't strand a report that already
// mapped columns onto that block's slots — and checks that every block names
// a known catalog kind and ends up with a unique ID.
func PrepareBlocks(blocks []BlockDef) ([]BlockDef, error) {
	prepared := make([]BlockDef, len(blocks))
	seen := make(map[string]bool, len(blocks))
	for i, b := range blocks {
		if !isKnownBlockKind(b.Kind) {
			return nil, fmt.Errorf("%w: block %d has unknown kind %q", ErrInvalidConfig, i, b.Kind)
		}
		for b.ID == "" || seen[b.ID] {
			b.ID = newBlockID()
		}
		seen[b.ID] = true
		prepared[i] = b
	}
	return prepared, nil
}

// slotKey namespaces a block's data slot so two blocks of the same kind never
// collide, regardless of whether their titles do.
func slotKey(blockID, sub string) string { return blockID + ":" + sub }

// blockLabel prefixes a slot's label with the block's own title, so the
// mapping UI can show "Sales -> Table columns" and "Returns -> Table columns"
// as distinct controls even though both blocks are tables.
func blockLabel(b BlockDef, label string) string {
	if b.Title == "" {
		return label
	}
	return b.Title + " – " + label
}

// blockSlots declares the data-bound slots one block contributes to its
// template's Meta. A text block contributes none: it has no data binding.
func blockSlots(b BlockDef) []Slot {
	switch b.Kind {
	case BlockTable:
		return []Slot{{
			Key:         slotKey(b.ID, "columns"),
			Label:       blockLabel(b, "Table columns"),
			Kind:        SlotColumns,
			Description: "The columns this block prints, in order.",
			Required:    true,
		}}
	case BlockGrouped:
		return []Slot{
			{
				Key:         slotKey(b.ID, "group"),
				Label:       blockLabel(b, "Group rows by"),
				Kind:        SlotColumn,
				Description: "The column whose distinct values become this block's rows.",
				Required:    true,
			},
			{
				Key:         slotKey(b.ID, "metrics"),
				Label:       blockLabel(b, "Columns to total"),
				Kind:        SlotColumns,
				Description: "Numeric columns summed within each group and across the block.",
				Required:    true,
				Numeric:     true,
			},
		}
	case BlockKPI:
		return []Slot{{
			Key:         slotKey(b.ID, "metrics"),
			Label:       blockLabel(b, "Headline metrics"),
			Kind:        SlotColumns,
			Description: "Each column becomes a tile showing its total across every row.",
			Required:    true,
			Numeric:     true,
		}}
	case BlockAISummary:
		return []Slot{
			{
				Key:         slotKey(b.ID, "columns"),
				Label:       blockLabel(b, "Context columns"),
				Kind:        SlotColumns,
				Description: "Columns from the report's own (already filtered) query output to share with the model. Nothing else from the query is sent.",
			},
			{
				Key:         slotKey(b.ID, "prompt"),
				Label:       blockLabel(b, "Prompt"),
				Kind:        SlotText,
				Description: "Instructions for the model — what to write about the data above.",
				Required:    true,
			},
		}
	case BlockText:
		return nil
	default:
		return nil
	}
}

// aiSummarySlotKey namespaces the internal slot an ai-summary block's
// generated text is threaded back through. It's never declared in Meta.Slots
// — a report's runner fills it in right before rendering, so it never shows
// up as a control for a user to edit directly.
func aiSummarySlotKey(blockID string) string { return slotKey(blockID, "summary") }

// AISummaryColumns returns the columns an ai-summary block's config chose to
// share with the model.
func AISummaryColumns(b BlockDef, cfg Config) []string {
	return cfg.ColumnsFor(slotKey(b.ID, "columns"))
}

// AISummaryPrompt returns the free-text instructions a user wrote for an
// ai-summary block.
func AISummaryPrompt(b BlockDef, cfg Config) string {
	return cfg.TextFor(slotKey(b.ID, "prompt"))
}

// WithAISummaryText returns a copy of cfg with an ai-summary block's
// generated text (or, on failure, an inline error message) set for Document
// and Render to print. It's how a report's runner threads a result computed
// outside this package — calling a model, running extra queries — back into
// the same Config a template renders with, without Document or Render ever
// needing to know a network call happened.
func WithAISummaryText(cfg Config, b BlockDef, text string) Config {
	out := cfg
	texts := make(map[string]string, len(cfg.Text)+1)
	maps.Copy(texts, cfg.Text)
	texts[aiSummarySlotKey(b.ID)] = text
	out.Text = texts
	return out
}

// AISummaryBlocks returns a template's ai-summary blocks, in order. Only a
// block-composed template (CustomTemplate) ever has any — a starter template
// always returns nil.
func AISummaryBlocks(t Template) []BlockDef {
	custom, ok := t.(CustomTemplate)
	if !ok {
		return nil
	}
	var blocks []BlockDef
	for _, b := range custom.Blocks {
		if b.Kind == BlockAISummary {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

// Unmapped-slot notes for a composed block, printed in place of its content
// while its columns are still unmapped — the same treatment a starter
// template gives an unfinished mapping.
const (
	blockTableUnmappedNote   = "No columns are mapped to this block yet."
	blockGroupedUnmappedNote = "Map a grouping column and at least one column to total."
	blockKPIUnmappedNote     = "Map at least one column to a headline metric."
	// noBlocksNote is what a custom design says when it has no blocks at all —
	// a design that shouldn't naturally happen, but has to print something
	// rather than fail if it's ever saved or previewed that way.
	noBlocksNote = "This design has no blocks yet."
)

// bucket is one group of rows sharing a printed grouping value.
type bucket struct {
	label string
	rows  [][]any
}

// bucketRows groups rows by the printed value of the given column, keeping
// first-appearance order so a query's own ORDER BY still decides the
// arrangement — the same rule GroupedTotals groups by.
func bucketRows(rows [][]any, group column) []bucket {
	at := make(map[string]int)
	var buckets []bucket
	for _, row := range rows {
		label := formatValue(valueAt(row, group.Index))
		index, seen := at[label]
		if !seen {
			index = len(buckets)
			at[label] = index
			buckets = append(buckets, bucket{label: label})
		}
		buckets[index].rows = append(buckets[index].rows, row)
	}
	return buckets
}

// blockDoc projects one block's content into the document, given the report's
// data and the slot mapping the user filled in for this block.
func blockDoc(b BlockDef, data Data, cfg Config) Block {
	switch b.Kind {
	case BlockTable:
		return tableBlockDoc(b, data, cfg)
	case BlockGrouped:
		return groupedBlockDoc(b, data, cfg)
	case BlockKPI:
		return kpiBlockDoc(b, data, cfg)
	case BlockText:
		return Block{Title: b.Title, Note: b.Note}
	case BlockAISummary:
		return aiSummaryBlockDoc(b, cfg)
	default:
		return Block{Title: b.Title, Note: fmt.Sprintf("Unknown block type %q.", b.Kind)}
	}
}

// blockAISummaryUnmappedNote is what an ai-summary block prints when nothing
// has generated its text yet — a report preview (which never calls the
// model) and a saved-but-not-yet-run report both look like this.
const blockAISummaryUnmappedNote = "This summary has not been generated yet."

// aiSummaryBlockDoc prints whatever text a report's runner already generated
// and threaded into cfg via WithAISummaryText. This package never calls a
// model itself — Data and Config are both plain, already-fetched values, and
// nothing here has a network client or a user's provider credentials.
func aiSummaryBlockDoc(b BlockDef, cfg Config) Block {
	text := strings.TrimSpace(cfg.TextFor(aiSummarySlotKey(b.ID)))
	if text == "" {
		return Block{Title: b.Title, Note: blockAISummaryUnmappedNote}
	}
	return Block{Title: b.Title, Note: text}
}

func tableBlockDoc(b BlockDef, data Data, cfg Config) Block {
	columns := data.resolve(cfg.ColumnsFor(slotKey(b.ID, "columns")))
	if len(columns) == 0 {
		return Block{Title: b.Title, Note: blockTableUnmappedNote}
	}
	return Block{Title: b.Title, Table: tableFrom(columns, data.Rows), Note: emptyNote(data.Rows)}
}

func groupedBlockDoc(b BlockDef, data Data, cfg Config) Block {
	groupColumns := data.resolve([]string{cfg.ColumnFor(slotKey(b.ID, "group"))})
	metrics := data.resolve(cfg.ColumnsFor(slotKey(b.ID, "metrics")))
	if len(groupColumns) == 0 || len(metrics) == 0 {
		return Block{Title: b.Title, Note: blockGroupedUnmappedNote}
	}
	group := groupColumns[0]

	table := &Table{Columns: []TableColumn{
		{Name: group.Name},
		{Name: rowsColumn, Numeric: true},
	}}
	for _, metric := range metrics {
		table.Columns = append(table.Columns, TableColumn{Name: metric.Name, Numeric: true})
	}

	buckets := bucketRows(data.Rows, group)
	for _, bkt := range buckets {
		row := []Value{textValue(bkt.label), computedValue(float64(len(bkt.rows)))}
		for _, metric := range metrics {
			row = append(row, computedValue(sum(bkt.rows, metric.Index)))
		}
		table.Rows = append(table.Rows, row)
	}

	if len(table.Rows) > 0 {
		total := []Value{textValue(totalLabel), computedValue(float64(len(data.Rows)))}
		for _, metric := range metrics {
			total = append(total, computedValue(sum(data.Rows, metric.Index)))
		}
		table.Total = total
	}

	return Block{Title: b.Title, Table: table, Note: emptyNote(data.Rows)}
}

func kpiBlockDoc(b BlockDef, data Data, cfg Config) Block {
	metrics := data.resolve(cfg.ColumnsFor(slotKey(b.ID, "metrics")))
	if len(metrics) == 0 {
		return Block{Title: b.Title, Note: blockKPIUnmappedNote}
	}
	block := Block{Title: b.Title}
	for _, metric := range metrics {
		block.Tiles = append(block.Tiles, Tile{Label: metric.Name, Value: computedValue(sum(data.Rows, metric.Index))})
	}
	return block
}

// CustomTemplate is a user-saved, block-composed template: an ordered list of
// blocks the user built inside a report, kept as a live definition every
// report referencing it renders through — editing it changes every report
// that uses it, going forward, with no versioning or snapshotting.
//
// It implements Template and Documenter the same way the starter templates
// do: Meta declares the slots its blocks need mapped, Render and Document
// both come from one composition of those blocks so the two can't disagree.
type CustomTemplate struct {
	ID          string
	UserID      string
	Name        string
	Description string
	Blocks      []BlockDef
}

func (t CustomTemplate) Meta() Meta {
	slots := []Slot{
		{
			Key:         "title",
			Label:       "Title",
			Kind:        SlotText,
			Description: "Printed at the top of the report.",
		},
		{
			Key:         "note",
			Label:       "Footer note",
			Kind:        SlotText,
			Description: "Optional line printed under the report, next to the row count.",
		},
	}
	for _, b := range t.Blocks {
		slots = append(slots, blockSlots(b)...)
	}
	return Meta{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Slots:       slots,
	}
}

func (t CustomTemplate) Document(data Data, cfg Config) Doc {
	doc := Doc{
		Title:       cfg.TextFor("title"),
		Footer:      footerText(data, cfg.TextFor("note")),
		GeneratedAt: data.GeneratedAt,
	}
	if len(t.Blocks) == 0 {
		doc.Blocks = []Block{{Note: noBlocksNote}}
		return doc
	}
	for _, b := range t.Blocks {
		doc.Blocks = append(doc.Blocks, blockDoc(b, data, cfg))
	}
	return doc
}

// customBody renders a Doc's own blocks generically, since a composed
// template's block count and arrangement isn't known ahead of time the way a
// starter template's fixed layout is. It walks the same Block/Tile/Table
// shapes the structural Document already produced, so Render can never
// disagree with Document about what a custom template shows.
var customBody = mustDocument(`
{{if .Blocks}}
{{range .Blocks}}
<section>
{{if .Title}}<h2 class="section-title">{{.Title}}</h2>{{end}}
{{if .Tiles}}
<div class="tiles">{{range .Tiles}}
<div class="tile"><p class="tile-label">{{.Label}}</p><p class="tile-value">{{.Value.Text}}</p></div>{{end}}
</div>
{{end}}
{{if .Table}}
<table>
<thead><tr>{{range .Table.Columns}}<th class="{{.Class}}">{{.Name}}</th>{{end}}</tr></thead>
<tbody>{{range .Table.Rows}}<tr>{{range .}}<td class="{{.Class}}">{{.Text}}</td>{{end}}</tr>{{end}}</tbody>
{{if .Table.Total}}<tfoot><tr>{{range .Table.Total}}<td class="{{.Class}}">{{.Text}}</td>{{end}}</tr></tfoot>{{end}}
</table>
{{end}}
{{if .Note}}<p class="empty">{{.Note}}</p>{{end}}
</section>
{{end}}
{{else}}<p class="empty">` + noBlocksNote + `</p>{{end}}
`)

type customView struct {
	page
	Blocks []Block
}

// Render builds the document structurally first, then prints it — so the page
// a user previews and the Document a PDF or spreadsheet is built from are
// always the same computation, never two.
func (t CustomTemplate) Render(data Data, cfg Config) ([]byte, error) {
	doc := t.Document(data, cfg)
	return renderDocument(customBody, customView{
		page:   page{Title: doc.Title, Subtitle: doc.Subtitle, Footer: doc.Footer},
		Blocks: doc.Blocks,
	})
}
