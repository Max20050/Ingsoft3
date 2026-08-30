# Report templates — architecture

How a report's rows become a document, and where a future drag-and-drop builder
plugs in.

## The pieces

| Piece | Where |
|---|---|
| Interface, slots, mapping, validation | `backend/internal/template/template.go` |
| Built-in template registry | `backend/internal/template/registry.go` |
| `Source` interface, `RegistrySource`, `CompositeSource`, `RenderWith`/`DocumentWith`/`ValidateConfigWith` | `backend/internal/template/source.go` |
| Block catalog + `CustomTemplate` composition engine | `backend/internal/template/blocks.go` |
| Custom template persistence (per user) | `backend/internal/template/customstore.go` |
| Per-user archive overlay (built-in + custom) | `backend/internal/template/archive.go` |
| Shared document shell, CSS, value formatting | `backend/internal/template/document.go` |
| Starter templates | `tabular.go`, `grouped.go`, `kpi.go` |
| HTTP endpoints | `backend/internal/report/handlers.go` |
| Persisted selection + mapping | `reports.template_id`, `reports.template_config` |
| Custom template + archive tables | `custom_templates`, `template_archive_state` |
| Template picker, block builder, and field mapping UI | `frontend/app/ui/template-picker.tsx` |

## The interface

```go
type Template interface {
    Meta() Meta                          // id, name, description, slots
    Render(Data, Config) ([]byte, error) // a complete HTML document
}
```

- **`Meta`** describes the template *and* the slots it exposes. The UI builds its
  field-mapping controls entirely from `Meta`, so a template that declares its
  slots correctly needs no frontend work.
- **`Slot`** is one hole in the layout: `text` (typed text, e.g. a title),
  `column` (one query output column), or `columns` (an ordered list). `numeric`
  marks slots meant for measures; it's a hint the UI uses to offer likely
  columns first, not a constraint — whether a column holds numbers is only
  knowable once the query has run, and a spreadsheet's `"1,234.50"` counts.
- **`Config`** is the user's mapping of slot keys to columns and text. It is
  persisted as JSONB, so its JSON shape is a storage format.
- **`Data`** is the query output: columns, rows, and an optional generation time.

`Validate(template, config, queryColumns)` is the one gate. Pass the query's
columns to also check that every mapped column exists; pass `nil` when they
aren't known yet, which is the case when saving a report (saving must not run
the user's query).

## Why HTML

`Render` returns a self-contained HTML document — styles inlined, no scripts, no
external requests. One artifact then serves two consumers:

1. the layout preview in the app (an iframe with an empty `sandbox`),
2. anything that archives or emails a report body.

The file formats don't come from that HTML. A spreadsheet wants cells it can
sum and a PDF wants blocks to lay out on a page, so a template describes the
same document a second time, structurally, through an optional second method:

```go
type Documenter interface {
    Document(Data, Config) Doc  // title, subtitle, footer, blocks
}
```

Both projections come out of one computation inside the template, so a report's
preview, PDF and spreadsheet agree. A template that doesn't implement
`Documenter` is still downloadable in every format — the fallback prints the
query's own output. See `documents/v1/report-rendering.md`.

## The render pipeline

```
stored query ─▶ connector.RunQuery ─▶ template.Data
                                          │
              stored template_id ─▶ Registry.Get ─┐
              stored template_config ─────────────┴─▶ Validate ─▶ Render ─▶ []byte
```

Nothing in that path knows which template it's running. `Registry.Render` is the
whole of it:

```go
func (r *Registry) Render(id string, data Data, cfg Config) ([]byte, error)
```

## Adding a template

1. Add a type in `backend/internal/template/` implementing `Template`.
2. Declare its slots in `Meta`.
3. Build its body with `mustDocument(...)` and render with `renderDocument(...)`,
   so it prints inside the shared document shell.
4. Implement `Document` so its PDF and spreadsheet print the same arrangement
   its page does. Skipping this is allowed; the files then show the query's own
   output.
5. Add it to `Starters()`.

That's the whole checklist — no handler, storage, or frontend change. The UI
picks up the new template from `GET /api/report-templates` and generates its
mapping controls from the slots.

## The block-composed "Build my own design" template (implemented)

The drag-and-drop builder sketched below shipped as block *composition and
ordering* — drag-and-drop itself stayed out of scope; reordering uses up/down
buttons. It plugs in exactly as a **template source**, not a second rendering
path, per the plan this section used to describe as a future.

`backend/internal/template/source.go` introduces the interface the report
pipeline now depends on instead of the concrete `Registry`:

```go
type Source interface {
    List(ctx context.Context, userID string) ([]Meta, error)
    Get(ctx context.Context, userID, id string) (Template, error)
}
```

- `RegistrySource` adapts `*Registry` to `Source` by ignoring `userID` — the
  starters stay available to everyone.
- `CustomStore` (`customstore.go`) persists a user's own block-composed
  templates in the `custom_templates` table — id, owning user id, name,
  description, an ordered `blocks` JSONB array, timestamps. It's a
  `CustomSource`: `List`/`Get`, both scoped to the owning user, so one
  account never resolves another's design.
- `ArchiveStore` (`archive.go`) is a single, unified per-user overlay over the
  `template_archive_state` table, keyed by `(user_id, template_id)`. It
  applies to built-in and custom template IDs alike, since archiving a
  built-in can't be a column on a row the user doesn't own. Archiving never
  deletes or blocks a template — `Get` ignores archive state entirely, so a
  report that already references an archived template keeps rendering; only
  `List`'s default listing excludes what the user archived.
- `CompositeSource` (`source.go`) merges `Registry` + `CustomStore` +
  `ArchiveStore` into the one `Source` the pipeline holds. `report.Runner` and
  `report.Handlers` now depend on `template.Source`, not `*template.Registry`
  directly; `report.Runner.Document` passes the report's own `UserID` through
  (a report already carries who it belongs to) so a custom template resolves
  correctly no matter who's viewing it.
- `RenderWith`, `DocumentWith`, `ValidateConfigWith` in `source.go` are the
  `Source`-shaped equivalents of `Registry.Render`/`Document`/`ValidateConfig`
  — same gate (`Validate` before running), just resolving through a `Source`
  and a `userID` instead of a fixed registry.

What did **not** change, as planned: `Template`, `Meta`, `Slot`, `Config`,
`Data`, `Validate`, the stored `template_id` + `template_config` columns, and
the frontend's slot-driven mapping UI — a block-composed template declares its
slots through the same `Meta.Slots` shape any starter does.

### The block catalog

`backend/internal/template/blocks.go` defines the catalog a design is composed
from — `table`, `grouped-table`, `kpi-tiles`, `text` — and `CustomTemplate`,
the `Template`+`Documenter` that composes an ordered `[]BlockDef` into a `Doc`
the same way `tabular.go`/`grouped.go`/`kpi.go` build theirs, reusing the same
underlying helpers (`data.resolve`, `tableFrom`, `sum`, `emptyNote`) so a
composed block's empty/partial-mapping/non-numeric behavior matches the
starters exactly.

- A block's `Title` doubles as its user-facing label; a stable `ID` (assigned
  once, kept across reorders and edits via `PrepareBlocks`) namespaces its
  slot keys (`"<blockID>:columns"`, `"<blockID>:group"`, `"<blockID>:metrics"`)
  so two blocks of the same kind — even with colliding titles — never collide
  in the mapping, and the mapping UI can still show each labelled with its
  block's title.
- `CustomTemplate.Render` calls `CustomTemplate.Document` and prints the
  result generically (`customBody`, a single template that walks whatever
  `Blocks` came out) — so the HTML page and the structural projection a PDF or
  spreadsheet builds from are literally the same computation, not two that
  have to be kept in sync by hand.
- Adding a fifth block kind (a chart, an image) means extending
  `isKnownBlockKind`, `blockSlots`, and `blockDoc`'s switches — the
  composition engine, the ordering UI, and the rendering pipeline don't
  change.

## API

| Endpoint | Purpose |
|---|---|
| `GET /api/report-templates` | Every built-in plus the user's own custom templates, minus what they've archived, each with `slots`, `owned`, `archived` — the picker and mapping UI are built from this |
| `GET /api/report-templates/archived` | The templates (built-in or custom) this user has archived, for the picker's collapsible "Show archived" section |
| `POST /api/report-templates` | Saves a block composition as a named, reusable custom template |
| `PUT /api/report-templates/{id}` | Reworks a saved custom template's blocks — a live reference, so this changes every report using it, going forward |
| `POST /api/report-templates/{id}/archive` | Hides a template (built-in or custom) from this user's default listing |
| `POST /api/report-templates/{id}/restore` | Un-hides it |
| `POST /api/reports/preview-template` | Runs the query, renders the chosen template with the real rows, returns `{ "html": ... }` |
| `POST /api/reports` | Saves `templateId` + `templateConfig` with the rest of the report |
| `GET /api/reports/{id}/download?format=` | Runs the report and returns one of its files |

`POST /api/reports` validates the mapping against the template's slots but not
against the query's columns; the render path re-validates against the rows it
actually receives, and reports a stale mapping by name.

## Notes on rendering behaviour

- **Escaping.** Report content is a user's own data, so it is data, not markup:
  `html/template` escapes every value and title.
- **Totals** ignore values that aren't numbers, so one stray label in a column
  doesn't void a total. Text a source formatted itself (`"$1,234.50"`) is parsed.
- **Alignment** follows the data: a column whose every value reads as a number is
  right-aligned with tabular figures; one label in it makes the column text.
- **A partial mapping still renders.** The preview has to show something while
  the user is still filling slots in, so a template with unmapped slots prints
  what it has and says what's missing.
- **Empty results are a normal outcome** — the document says so rather than
  failing.
