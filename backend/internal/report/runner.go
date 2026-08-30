package report

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/Max20050/docuwave/internal/datasource"
	"github.com/Max20050/docuwave/internal/llm"
	"github.com/Max20050/docuwave/internal/query"
	"github.com/Max20050/docuwave/internal/render"
	"github.com/Max20050/docuwave/internal/template"
)

// queryTimeout bounds a query run against the user's own data source.
const queryTimeout = 30 * time.Second

// ErrNotRunnable is returned for a report saved before query building replaced
// LLM-written SQL. Its query text is kept for the user to read, but nothing
// runs a stored query as text, so the report has to be rebuilt.
var ErrNotRunnable = errors.New("this report predates query building and has to be rebuilt")

// ErrQueryFailed means the data source didn't answer the report's query. The
// query is this server's own, so the source's message is the useful one and is
// wrapped in.
var ErrQueryFailed = errors.New("the data source could not answer this report's query")

// Runner is the report pipeline, end to end: compile the stored specification
// against the data source's schema, run it, project the template's document,
// and render the files the report is configured for.
//
// Every path that produces a report goes through here — the preview while a
// report is being built, the download in the app, and the scheduled and
// on-demand deliveries that come next — so what a user approves in the preview
// is what lands in their inbox.
type Runner struct {
	resolver  *datasource.Resolver
	schemas   *datasource.SchemaProvider
	templates template.Source
	// summaries generates the text an ai-summary block prints, from the
	// report owner's own configured LLM provider. It's nil in tests that never
	// exercise a template with such a block.
	summaries *llm.Generator
	// mappings is a REST source's api_field -> our_field mapping store. A
	// report against a REST source is compiled against the mapped our_field
	// names (see mappedRESTSchema), not the raw api_field names Introspect
	// reports — that's what the report builder's field picker offers.
	mappings *datasource.FieldMappingStore
}

func NewRunner(
	resolver *datasource.Resolver,
	schemas *datasource.SchemaProvider,
	templates template.Source,
	summaries *llm.Generator,
	mappings *datasource.FieldMappingStore,
) *Runner {
	return &Runner{resolver: resolver, schemas: schemas, templates: templates, summaries: summaries, mappings: mappings}
}

// runnable is a compiled query with everything needed to run it.
type runnable struct {
	connector datasource.Connector
	compiled  query.Compiled
	// language names the query language, for labelling the SQL in the UI.
	language string
}

// prepare resolves a data source, loads the schema read when it was connected,
// and compiles the specification against it. Every query in this package is
// built here and nowhere else.
func (r *Runner) prepare(
	ctx context.Context,
	userID, dataSourceID string,
	spec query.Spec,
) (runnable, error) {
	source, connector, err := r.resolver.Resolve(ctx, userID, dataSourceID)
	if err != nil {
		return runnable{}, err
	}

	dialect, err := query.DialectFor(source.Type)
	if err != nil {
		return runnable{}, err
	}

	schema, _, err := r.schemas.Schema(ctx, userID, dataSourceID)
	if err != nil {
		return runnable{}, err
	}

	if dialect == query.DialectREST {
		schema, err = r.mappedRESTSchema(ctx, dataSourceID)
		if err != nil {
			return runnable{}, err
		}
	}

	compiled, err := query.Compile(spec, schema, dialect, time.Now())
	if err != nil {
		return runnable{}, err
	}

	return runnable{connector: connector, compiled: compiled, language: connector.QueryLanguage()}, nil
}

// mappedRESTSchema builds the schema a REST source's spec is validated
// against: the deduped list of our_field names its stored field mapping
// points at, rather than the raw api_field names Introspect reports. That's
// what makes the report builder's field picker offer system-field names, and
// what makes selecting an unmapped field fail at compile time instead of
// silently returning nothing for it.
func (r *Runner) mappedRESTSchema(ctx context.Context, dataSourceID string) (datasource.Schema, error) {
	mapping, _, err := r.mappings.Get(ctx, dataSourceID)
	if err != nil {
		if !errors.Is(err, datasource.ErrFieldMappingNotFound) {
			return datasource.Schema{}, err
		}
		mapping = map[string]string{}
	}

	seen := make(map[string]bool, len(mapping))
	fields := make([]string, 0, len(mapping))
	for _, ourField := range mapping {
		if seen[ourField] {
			continue
		}
		seen[ourField] = true
		fields = append(fields, ourField)
	}
	sort.Strings(fields)

	return datasource.Schema{Fields: fields}, nil
}

// run executes a prepared query.
func (r runnable) run(ctx context.Context, limit int) (datasource.QueryResult, error) {
	return r.connector.RunQuery(ctx, r.compiled.Text, r.compiled.Args, limit)
}

// runLimit is how many rows a report run reads. The compiled query carries the
// specification's limit already; this is the connector's own cap, set to the
// same number so a source that ignores a limit still can't stream more than the
// report asked for.
func runLimit(spec query.Spec) int {
	if spec.Limit > 0 {
		return spec.Limit
	}
	return query.DefaultRowLimit
}

// Document runs a report's query and projects the document its template prints.
// It's the half of the pipeline every output format shares.
func (r *Runner) Document(ctx context.Context, rep Report) (template.Doc, error) {
	if rep.QuerySpec.IsZero() {
		return template.Doc{}, ErrNotRunnable
	}

	prepared, err := r.prepare(ctx, rep.UserID, rep.DataSourceID, rep.QuerySpec)
	if err != nil {
		log.Printf("report %s: preparing its query failed: %v", rep.ID, err)
		return template.Doc{}, err
	}

	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	result, err := prepared.run(queryCtx, runLimit(rep.QuerySpec))
	if err != nil {
		// A report is rendered long after it was configured, unattended, so a
		// failure here is logged as well as returned: the schedule that hit it
		// won't be watching.
		log.Printf("report %s: query failed: %v", rep.ID, err)
		return template.Doc{}, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}

	data := template.Data{Columns: result.Columns, Rows: result.Rows, GeneratedAt: time.Now()}

	t, err := r.templates.Get(ctx, rep.UserID, rep.TemplateID)
	if err != nil {
		log.Printf("report %s: template %s failed: %v", rep.ID, rep.TemplateID, err)
		return template.Doc{}, err
	}
	if err := template.Validate(t, rep.TemplateConfig, data.Columns); err != nil {
		log.Printf("report %s: template %s failed: %v", rep.ID, rep.TemplateID, err)
		return template.Doc{}, err
	}

	// Every ai-summary block's text is generated fresh for this run — a
	// report is rendered again for every recipient and every schedule, and
	// the data behind it can have changed since the last time. A block that
	// fails to generate doesn't fail the report: its own spot in the document
	// says so instead, and everything else still renders.
	cfg := rep.TemplateConfig
	for _, block := range template.AISummaryBlocks(t) {
		cfg = template.WithAISummaryText(cfg, block, r.aiSummaryText(ctx, rep, data, block))
	}

	return template.DocumentOf(t, data, cfg), nil
}

// aiSummaryText produces the text one ai-summary block prints: the model's
// answer to the block's prompt, given the report's own selected columns and
// whatever additional queries the block defines. A failure anywhere in that
// — no provider configured, the provider's API rejected the call, an extra
// query failed — comes back as text explaining what went wrong, rather than
// an error, so it never takes the rest of the report down with it.
func (r *Runner) aiSummaryText(ctx context.Context, rep Report, data template.Data, block template.BlockDef) string {
	if r.summaries == nil {
		return "AI summaries are not configured on this server."
	}

	prompt := template.AISummaryPrompt(block, rep.TemplateConfig)
	if prompt == "" {
		return "This block has no prompt configured."
	}

	contextText, err := r.aiSummaryContext(ctx, rep, data, block)
	if err != nil {
		log.Printf("report %s: ai-summary block %q: gathering context failed: %v", rep.ID, block.Title, err)
		return fmt.Sprintf("Could not gather this summary's data: %v", err)
	}

	full := prompt
	if contextText != "" {
		full = fmt.Sprintf("%s\n\nData:\n%s", prompt, contextText)
	}

	text, err := r.summaries.GenerateSummary(ctx, rep.UserID, full)
	if err != nil {
		log.Printf("report %s: ai-summary block %q: generation failed: %v", rep.ID, block.Title, err)
		return fmt.Sprintf("Could not generate this summary: %v", err)
	}
	return text
}

// aiSummaryContext assembles the plain-text data an ai-summary block sends
// the model: the report's own already-filtered rows, cut down to the columns
// the block chose, followed by the block's additional queries — run against
// the same data source, never shown in the document itself.
func (r *Runner) aiSummaryContext(ctx context.Context, rep Report, data template.Data, block template.BlockDef) (string, error) {
	var sections []string

	if columns := template.AISummaryColumns(block, rep.TemplateConfig); len(columns) > 0 {
		sections = append(sections, formatColumnsAsText("This report's own data", columns, data))
	}

	for _, extra := range block.Queries {
		prepared, err := r.prepare(ctx, rep.UserID, rep.DataSourceID, extra.Spec)
		if err != nil {
			return "", fmt.Errorf("query %q: %w", extra.Title, err)
		}

		queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
		result, err := prepared.run(queryCtx, runLimit(extra.Spec))
		cancel()
		if err != nil {
			return "", fmt.Errorf("query %q: %w", extra.Title, err)
		}

		title := extra.Title
		if title == "" {
			title = "Additional data"
		}
		sections = append(sections, formatColumnsAsText(title, result.Columns, template.Data{
			Columns: result.Columns, Rows: result.Rows,
		}))
	}

	return strings.Join(sections, "\n\n"), nil
}

// formatColumnsAsText prints a subset of a query's columns as a small text
// table — plain rows a model can read, not a document a person is meant to.
// Columns the query didn't return are skipped rather than failing: a stale
// mapping shouldn't keep the rest of the summary from generating.
func formatColumnsAsText(title string, columns []string, data template.Data) string {
	index := make(map[string]int, len(data.Columns))
	for i, name := range data.Columns {
		index[name] = i
	}
	kept := make([]string, 0, len(columns))
	for _, name := range columns {
		if _, ok := index[name]; ok {
			kept = append(kept, name)
		}
	}
	if len(kept) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s):\n", title, strings.Join(kept, ", "))
	for _, row := range data.Rows {
		values := make([]string, len(kept))
		for i, name := range kept {
			if col := index[name]; col < len(row) {
				values[i] = fmt.Sprint(row[col])
			}
		}
		fmt.Fprintf(&b, "- %s\n", strings.Join(values, " | "))
	}
	return b.String()
}

// Render produces every file a report is configured for, in memory, ready for
// the delivery layer to attach.
func (r *Runner) Render(ctx context.Context, rep Report) ([]render.Artifact, error) {
	doc, err := r.Document(ctx, rep)
	if err != nil {
		return nil, err
	}

	artifacts, err := render.RenderAll(doc, rep.Formats, rep.Name)
	if err != nil {
		log.Printf("report %s: rendering failed: %v", rep.ID, err)
		return nil, err
	}
	return artifacts, nil
}

// RenderFormat produces one of the report's files. The format has to be one the
// report is configured for: the configuration is what a client agreed to
// receive, and it's also what the preview and the schedule act on.
func (r *Runner) RenderFormat(
	ctx context.Context,
	rep Report,
	format render.Format,
) (render.Artifact, error) {
	if !render.Includes(rep.Formats, format) {
		return render.Artifact{}, fmt.Errorf("%w: this report is not configured for %s, only %v",
			render.ErrUnknownFormat, format, render.Strings(rep.Formats))
	}

	doc, err := r.Document(ctx, rep)
	if err != nil {
		return render.Artifact{}, err
	}

	artifact, err := render.Render(doc, format, rep.Name)
	if err != nil {
		log.Printf("report %s: rendering %s failed: %v", rep.ID, format, err)
		return render.Artifact{}, err
	}
	return artifact, nil
}
