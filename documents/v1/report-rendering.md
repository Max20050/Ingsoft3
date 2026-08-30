# Report rendering — architecture

How a saved report becomes files: PDF, Excel (.xlsx) and CSV, produced in memory
and handed to the delivery layer.

## The pieces

| Piece | Where |
|---|---|
| Pipeline: query → rows → document → files | `backend/internal/report/runner.go` |
| Document model a template projects | `backend/internal/template/doc.go` |
| Format registry, artifacts, filenames | `backend/internal/render/render.go` |
| Per-format renderers | `render/pdf.go`, `render/xlsx.go`, `render/csv.go` |
| Configured formats | `reports.formats` (`TEXT[]`, at least one) |
| Download endpoint | `GET /api/reports/{id}/download?format=` |

## The pipeline

```
stored query spec ─▶ compile against the source's schema ─▶ RunQuery ─▶ rows
                                                                        │
     stored template_id + template_config ─▶ Registry.Document ─▶ template.Doc
                                                                        │
                          stored formats ─▶ render.RenderAll ─▶ []render.Artifact
```

`report.Runner` is the whole of it, and it's the only path that produces a
report:

```go
func (r *Runner) Render(ctx context.Context, rep Report) ([]render.Artifact, error)
func (r *Runner) RenderFormat(ctx context.Context, rep Report, f render.Format) (render.Artifact, error)
```

Email delivery and the scheduler call `Render`; the download endpoint calls
`RenderFormat`. Nothing renders a report anywhere else, so what a user previews
is what their recipients get.

An `Artifact` is bytes, a filename and a content type — never a path. A report
runs unattended on a schedule, on a server that doesn't need a writable disk,
and the delivery layer wants something to attach.

## Why a document model rather than the HTML

Issue #9's templates render a self-contained HTML page, and that page is what
the preview shows and what an email body carries. The file formats can't use it:

- A **spreadsheet** wants cells. A recipient sorts, filters and re-totals a
  report, so a value the document prints as `1,234.50` has to arrive as the
  number `1234.5` — parsing that back out of `<td>` markup would be inventing a
  browser's job.
- A **PDF** wants blocks to lay out on a page. Printing HTML faithfully means
  shipping one: a headless Chrome next to the API is a large, sandboxed,
  frequently patched dependency for a server whose job is to email a few pages
  of tables.

So a template describes its document a second time, structurally:

```go
type Documenter interface {
    Document(Data, Config) Doc
}
```

`Doc` is a title, a subtitle, a footer and blocks; a block is headline figures,
a table, or a note. A `Value` carries both the text the page prints and the
number behind it when there is one, which is what keeps a spreadsheet's totals
identical to the printed ones.

Both projections come out of one computation inside the template — the grouped
template buckets its rows once and hands the result to its HTML and to its
`Doc` — so a report's page, its PDF and its spreadsheet can't disagree.

`Documenter` is optional. A template that doesn't implement it still renders to
every format: `DocumentOf` falls back to printing the query's own output. So
implementing it is how a template controls its files, not a condition of having
any.

## Adding a format

1. Implement `render.Renderer` — `Extension`, `ContentType`, `Render(Doc)`.
2. Add it to `renderers` in `render.go`.

Nothing else changes. The registry decides what `ParseFormat` accepts, what the
UI offers, and what a report may be configured with, so a new format is a file
and a line.

## Formats per report

`reports.formats` is a non-empty `TEXT[]`. Duplicates collapse and the list is
stored in registry order, so a report reads the same however the client listed
its formats. A create request that omits `formats` gets PDF; one that sends an
empty list is asking for nothing and is refused.

Downloading is limited to the formats a report is configured for — the
configuration is what a client agreed to receive, and it's what the schedule
acts on.

## What each format looks like

**PDF** — A4, neutral ink, hairline rules, no series colour, exactly like the
HTML document: in a report the data is the only thing allowed to be loud. Tables
repeat their header on every page they run onto, numeric columns are
right-aligned, and the footer carries the row count, the generation time and the
page number. Text is drawn in the PDF core fonts, which cover Latin text
(cp1252); a script outside that range is transliterated where it can be.

**Excel** — one sheet, the document's blocks laid out down it: heading, tiles as
label/value pairs, tables with a bold header row and a ruled total row. Numbers
are written as numbers, and columns are sized to their content.

**CSV** — the same document flattened onto the one sheet a CSV has: heading,
then each block, separated by blank lines. Numbers are written unformatted,
because a CSV is read by another program more often than by a person, and
`1,234.5` is not a number to any of them. The file starts with a UTF-8 byte
order mark so that Excel doesn't mangle accents and currency symbols.

## Failures

A report is rendered unattended, so every failure is logged with the report's ID
as well as returned. The download endpoint maps them onto the answer the user
needs:

| Failure | Status |
|---|---|
| Report predates query building (`ErrNotRunnable`) | 409 — rebuild it |
| A format the report isn't configured for | 400 |
| The data source didn't answer (`ErrQueryFailed`) | 502 |
| A mapping the query no longer supports | 400 — it names the column |
| The data source or its connection is gone | 404 |

## Limits

- A report reads at most its specification's row limit (default 1000, max 5000).
  Reports are summaries; an unbounded query against someone's production
  database is never what they meant.
- A PDF table is sized to the page, so a very wide table truncates its cells —
  that report wants a spreadsheet.
- Table rows are one line high; long values are shortened with an ellipsis
  rather than wrapped.
