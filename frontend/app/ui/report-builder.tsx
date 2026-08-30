"use client";

import { useEffect, useState } from "react";
import {
  AGGREGATES,
  OPERATORS,
  REPORT_FORMATS,
  archiveReportTemplate,
  createCustomTemplate,
  emptyQuerySpec,
  getDataSourceSchema,
  getFieldMapping,
  getLLMConfig,
  listArchivedReportTemplates,
  listReportTemplates,
  previewReport,
  previewReportTemplate,
  restoreReportTemplate,
  updateCustomTemplate,
  type Aggregate,
  type DataSource,
  type DataSourceSchema,
  type FieldMapping,
  type LLMConfig,
  type Operator,
  type OperatorArity,
  type PlaceholderFilter,
  type QueryField,
  type QueryFilter,
  type QueryPreview,
  type QuerySort,
  type QuerySpec,
  type ReportFormat,
  type ReportInput,
  type ReportTemplate,
  type SchemaColumn,
  type TemplateConfig,
} from "@/lib/api";
import { QueryPreviewTable } from "@/app/ui/query-preview-table";
import { DataSourceFieldMappingPanel } from "@/app/ui/data-source-field-mapping";
import {
  TemplatePicker,
  defaultTemplateConfig,
  numericColumns,
  type CustomTemplateDraft,
} from "@/app/ui/template-picker";

const inputClass = "rounded border border-black/[.1] px-3 py-2 dark:border-white/[.15] dark:bg-black";
const smallInputClass = `${inputClass} py-1 text-sm`;
const removeButtonClass =
  "rounded border border-black/[.1] px-2 py-1 text-xs transition-colors hover:bg-black/[.04] dark:border-white/[.15] dark:hover:bg-[#1a1a1a]";

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

function sameColumns(left: string[], right: string[]): boolean {
  return left.length === right.length && left.every((column, index) => column === right[index]);
}

function hasAISummaryBlock(blocks: { kind: string }[] | undefined): boolean {
  return (blocks ?? []).some((b) => b.kind === "ai-summary");
}

// columnMetaFor reads the columns a data source's schema makes available,
// each with its declared type kept alongside its name for UI that shows a
// type hint (Google Sheets and REST fields have none): a table's own columns
// for SQL sources, the sheet's header fields for Google Sheets (which report
// an empty table), and a REST source's currently-mapped system fields.
//
// For a REST source, the offered columns aren't the raw api_field names
// Introspect reports — they're the distinct our_field (system field) names
// the source's stored mapping currently points at, matching what the backend
// compiles a REST spec's fields against (see report.Runner.mappedRESTSchema).
// `name` stays the raw system-field key FieldPicker/FieldOrderPanel and the
// backend expect in QueryField.column; only the label shown is prettified via
// the mapping response's systemFields catalog.
function columnMetaFor(
  schema: DataSourceSchema | null,
  table: string,
  fieldMapping: FieldMapping | null,
): SchemaColumn[] {
  if (!schema) return [];
  if (schema.type === "google_sheets") return (schema.fields ?? []).map((name) => ({ name, type: "" }));
  if (schema.type === "rest_api") {
    if (!fieldMapping) return [];
    const labelFor = new Map(fieldMapping.systemFields.map((f) => [f.key, f.label]));
    const seen = new Set<string>();
    const columns: SchemaColumn[] = [];
    for (const ourField of Object.values(fieldMapping.mapping)) {
      if (seen.has(ourField)) continue;
      seen.add(ourField);
      columns.push({ name: ourField, type: labelFor.get(ourField) ?? "" });
    }
    return columns;
  }
  return schema.tables?.find((candidate) => candidate.name === table)?.columns ?? [];
}

function operatorArity(operator: Operator): OperatorArity {
  return OPERATORS.find((candidate) => candidate.value === operator)?.arity ?? "one";
}

// FormatPicker chooses the files the report is delivered as. A report needs at
// least one, so the last one checked can't be unchecked — the save button would
// only refuse it a step later.
function FormatPicker({
  formats,
  onChange,
}: {
  formats: ReportFormat[];
  onChange: (formats: ReportFormat[]) => void;
}) {
  function toggle(format: ReportFormat, checked: boolean) {
    onChange(
      checked
        ? REPORT_FORMATS.map((option) => option.value).filter(
            (value) => value === format || formats.includes(value),
          )
        : formats.filter((value) => value !== format),
    );
  }

  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="text-sm font-medium">Output formats</legend>
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Every format you pick is generated each time the report runs.
      </p>
      {REPORT_FORMATS.map((option) => {
        const checked = formats.includes(option.value);
        return (
          <label key={option.value} className="flex items-start gap-2 text-sm">
            <input
              type="checkbox"
              checked={checked}
              disabled={checked && formats.length === 1}
              onChange={(e) => toggle(option.value, e.target.checked)}
              className="mt-1"
            />
            <span>
              <span className="font-medium">{option.label}</span>{" "}
              <span className="text-zinc-600 dark:text-zinc-400">{option.description}</span>
            </span>
          </label>
        );
      })}
    </fieldset>
  );
}

// columnBadge shortens a schema column's declared type down to the same kind
// of one-glyph hint the field picker shows next to each checkbox.
function columnBadge(type: string | undefined): string {
  const t = (type ?? "").toLowerCase();
  if (/int|numeric|decimal|float|double|serial/.test(t)) return "123";
  if (/date|time/.test(t)) return "\u{1F4C5}";
  if (/bool/.test(t)) return "✓";
  return "A";
}

// FieldPicker lists every column a table (or sheet) offers as a checkbox: on
// for columns already in the query's field list, off otherwise. Toggling adds
// or removes the column from that list without disturbing the position of the
// fields that stay — reordering is FieldOrderPanel's job, not this one's.
function FieldPicker({
  tableLabel,
  columns,
  fields,
  onChange,
}: {
  tableLabel: string;
  columns: SchemaColumn[];
  fields: QueryField[];
  onChange: (fields: QueryField[]) => void;
}) {
  function toggle(column: string, checked: boolean) {
    if (checked) {
      if (fields.some((f) => f.column === column)) return;
      onChange([...fields, { column, aggregate: "" }]);
    } else {
      onChange(fields.filter((f) => f.column !== column));
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <div>
        <h3 className="text-sm font-medium">Choose what to include</h3>
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          Pick the columns you want in the report.
        </p>
      </div>
      <div className="flex flex-col gap-1">
        <p className="text-xs font-medium uppercase tracking-wide text-zinc-500 dark:text-zinc-400">
          Fields available in: {tableLabel}
        </p>
        <div className="flex flex-col divide-y divide-black/[.06] rounded border border-black/[.1] dark:divide-white/[.08] dark:border-white/[.15]">
          {columns.map((column) => {
            const checked = fields.some((f) => f.column === column.name);
            return (
              <label
                key={column.name}
                className="flex cursor-pointer items-center gap-3 px-3 py-2 text-sm transition-colors hover:bg-black/[.03] dark:hover:bg-[#1a1a1a]"
              >
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={(e) => toggle(column.name, e.target.checked)}
                />
                <span className="flex h-5 w-6 shrink-0 items-center justify-center rounded bg-black/[.06] text-[10px] font-medium dark:bg-white/[.1]">
                  {columnBadge(column.type)}
                </span>
                <span className="font-medium">{column.name}</span>
                {column.type && (
                  <span className="text-xs text-zinc-500 dark:text-zinc-400">{column.type}</span>
                )}
              </label>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// FieldOrderPanel is the "your report" side of field selection: the columns
// already picked, in the order they'll appear, reorderable by native
// drag-and-drop and each still able to carry an aggregate.
function FieldOrderPanel({
  fields,
  onChange,
}: {
  fields: QueryField[];
  onChange: (fields: QueryField[]) => void;
}) {
  const [dragIndex, setDragIndex] = useState<number | null>(null);

  function update(index: number, field: QueryField) {
    onChange(fields.map((f, i) => (i === index ? field : f)));
  }
  function remove(index: number) {
    onChange(fields.filter((_, i) => i !== index));
  }
  function moveTo(from: number, to: number) {
    if (from === to) return;
    const next = [...fields];
    const [moved] = next.splice(from, 1);
    next.splice(to, 0, moved);
    onChange(next);
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-medium">Your report</h3>
          <p className="text-sm text-zinc-600 dark:text-zinc-400">Drag to reorder.</p>
        </div>
        {fields.length > 0 && (
          <button type="button" onClick={() => onChange([])} className="text-xs text-red-600 hover:underline">
            Clear all
          </button>
        )}
      </div>
      {fields.length === 0 ? (
        <p className="rounded border border-dashed border-black/[.1] px-3 py-6 text-center text-sm text-zinc-500 dark:border-white/[.15] dark:text-zinc-400">
          Check fields on the left to add them here.
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {fields.map((field, index) => (
            <li
              key={`${field.column}-${index}`}
              draggable
              onDragStart={() => setDragIndex(index)}
              onDragOver={(e) => e.preventDefault()}
              onDrop={(e) => {
                e.preventDefault();
                if (dragIndex !== null) moveTo(dragIndex, index);
                setDragIndex(null);
              }}
              onDragEnd={() => setDragIndex(null)}
              className={`flex items-center gap-2 rounded border border-black/[.1] bg-black/[.02] px-2 py-2 dark:border-white/[.15] dark:bg-white/[.03] ${
                dragIndex === index ? "opacity-50" : ""
              }`}
            >
              <span className="cursor-grab select-none text-zinc-400" aria-hidden>
                ⋮⋮
              </span>
              <span className="min-w-0 flex-1 truncate text-sm font-medium">
                {field.aggregate === "count" ? "(all rows)" : field.column}
              </span>
              <select
                value={field.aggregate ?? ""}
                onChange={(e) => {
                  const aggregate = e.target.value as Aggregate;
                  update(index, {
                    ...field,
                    aggregate,
                    column: aggregate === "count" ? "" : field.column,
                  });
                }}
                className={smallInputClass}
              >
                {AGGREGATES.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
              <button type="button" onClick={() => remove(index)} className={removeButtonClass}>
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

// filterValueInputs renders the value control(s) a filter's operator needs:
// none, one, two (between), or a comma-separated list (in).
function FilterValueInputs({ filter, onChange }: { filter: QueryFilter; onChange: (filter: QueryFilter) => void }) {
  const arity = operatorArity(filter.operator);
  if (arity === "none") return null;
  if (arity === "pair") {
    const values = Array.isArray(filter.values) ? filter.values : ["", ""];
    return (
      <>
        <input
          value={String(values[0] ?? "")}
          onChange={(e) => onChange({ ...filter, values: [e.target.value, values[1] ?? ""] })}
          className={smallInputClass}
          placeholder="from"
        />
        <input
          value={String(values[1] ?? "")}
          onChange={(e) => onChange({ ...filter, values: [values[0] ?? "", e.target.value] })}
          className={smallInputClass}
          placeholder="to"
        />
      </>
    );
  }
  if (arity === "many") {
    const text = Array.isArray(filter.values) ? filter.values.join(", ") : "";
    return (
      <input
        value={text}
        onChange={(e) =>
          onChange({
            ...filter,
            values: e.target.value.split(",").map((v) => v.trim()).filter((v) => v !== ""),
          })
        }
        className={smallInputClass}
        placeholder="a, b, c"
      />
    );
  }
  if (arity === "count") {
    return (
      <input
        type="number"
        min={1}
        value={typeof filter.value === "number" ? filter.value : ""}
        onChange={(e) => onChange({ ...filter, value: Number(e.target.value) })}
        className={smallInputClass}
        placeholder="days"
      />
    );
  }
  return (
    <input
      value={typeof filter.value === "string" || typeof filter.value === "number" ? String(filter.value) : ""}
      onChange={(e) => onChange({ ...filter, value: e.target.value })}
      className={smallInputClass}
      placeholder="value"
    />
  );
}

// FilterEditor builds the Filters part of a query specification.
function FilterEditor({
  columns,
  filters,
  onChange,
}: {
  columns: string[];
  filters: QueryFilter[];
  onChange: (filters: QueryFilter[]) => void;
}) {
  function update(index: number, filter: QueryFilter) {
    onChange(filters.map((f, i) => (i === index ? filter : f)));
  }
  function remove(index: number) {
    onChange(filters.filter((_, i) => i !== index));
  }
  function add() {
    onChange([...filters, { column: columns[0] ?? "", operator: "eq" }]);
  }

  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="text-sm font-medium">Filters</legend>
      {filters.map((filter, index) => (
        <div key={index} className="flex flex-wrap items-center gap-2">
          <select
            value={filter.column}
            onChange={(e) => update(index, { ...filter, column: e.target.value })}
            className={smallInputClass}
          >
            {columns.map((column) => (
              <option key={column} value={column}>
                {column}
              </option>
            ))}
          </select>
          <select
            value={filter.operator}
            onChange={(e) => update(index, { ...filter, operator: e.target.value as Operator })}
            className={smallInputClass}
          >
            {OPERATORS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <FilterValueInputs filter={filter} onChange={(next) => update(index, next)} />
          <button type="button" onClick={() => remove(index)} className={removeButtonClass}>
            Remove
          </button>
        </div>
      ))}
      <div>
        <button type="button" onClick={add} disabled={columns.length === 0} className={removeButtonClass}>
          Add filter
        </button>
      </div>
    </fieldset>
  );
}

// SortEditor builds the Sorts part of a query specification.
function SortEditor({
  columns,
  sorts,
  onChange,
}: {
  columns: string[];
  sorts: QuerySort[];
  onChange: (sorts: QuerySort[]) => void;
}) {
  function update(index: number, sort: QuerySort) {
    onChange(sorts.map((s, i) => (i === index ? sort : s)));
  }
  function remove(index: number) {
    onChange(sorts.filter((_, i) => i !== index));
  }
  function add() {
    onChange([...sorts, { column: columns[0] ?? "", descending: false }]);
  }

  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="text-sm font-medium">Sort</legend>
      {sorts.map((sort, index) => (
        <div key={index} className="flex items-center gap-2">
          <select
            value={sort.column}
            onChange={(e) => update(index, { ...sort, column: e.target.value })}
            className={smallInputClass}
          >
            {columns.map((column) => (
              <option key={column} value={column}>
                {column}
              </option>
            ))}
          </select>
          <label className="flex items-center gap-1 text-xs">
            <input
              type="checkbox"
              checked={sort.descending ?? false}
              onChange={(e) => update(index, { ...sort, descending: e.target.checked })}
            />
            Descending
          </label>
          <button type="button" onClick={() => remove(index)} className={removeButtonClass}>
            Remove
          </button>
        </div>
      ))}
      <div>
        <button type="button" onClick={add} disabled={columns.length === 0} className={removeButtonClass}>
          Add sort
        </button>
      </div>
    </fieldset>
  );
}

const RECIPIENT_FIELD_SUGGESTIONS = ["email", "name"];

// PlaceholderFilterEditor builds a report's Inputs: filters whose value isn't
// typed now, but is resolved from the recipient the report is eventually sent
// to. They don't affect preview — the server keeps them out of query
// compilation until a future delivery step resolves them.
function PlaceholderFilterEditor({
  columns,
  filters,
  onChange,
}: {
  columns: string[];
  filters: PlaceholderFilter[];
  onChange: (filters: PlaceholderFilter[]) => void;
}) {
  function update(index: number, filter: PlaceholderFilter) {
    onChange(filters.map((f, i) => (i === index ? filter : f)));
  }
  function remove(index: number) {
    onChange(filters.filter((_, i) => i !== index));
  }
  function add() {
    onChange([...filters, { column: columns[0] ?? "", operator: "eq", recipientField: "" }]);
  }

  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="text-sm font-medium">Inputs</legend>
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Filters filled in from the recipient a report is sent to, once that&apos;s set up — not from a
        value you type now.
      </p>
      {filters.map((filter, index) => (
        <div key={index} className="flex flex-wrap items-center gap-2">
          <select
            value={filter.column}
            onChange={(e) => update(index, { ...filter, column: e.target.value })}
            className={smallInputClass}
          >
            {columns.map((column) => (
              <option key={column} value={column}>
                {column}
              </option>
            ))}
          </select>
          <select
            value={filter.operator}
            onChange={(e) => update(index, { ...filter, operator: e.target.value as Operator })}
            className={smallInputClass}
          >
            {OPERATORS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
          <input
            value={filter.recipientField}
            onChange={(e) => update(index, { ...filter, recipientField: e.target.value })}
            list="recipient-field-suggestions"
            placeholder="recipient field (e.g. region)"
            className={smallInputClass}
          />
          <button type="button" onClick={() => remove(index)} className={removeButtonClass}>
            Remove
          </button>
        </div>
      ))}
      <datalist id="recipient-field-suggestions">
        {RECIPIENT_FIELD_SUGGESTIONS.map((field) => (
          <option key={field} value={field} />
        ))}
      </datalist>
      <div>
        <button type="button" onClick={add} disabled={columns.length === 0} className={removeButtonClass}>
          Add input
        </button>
      </div>
    </fieldset>
  );
}

const STEPS = [
  { n: 1, label: "Source" },
  { n: 2, label: "Data" },
  { n: 3, label: "Design" },
  { n: 4, label: "Publish" },
] as const;

// StepIndicator shows where the user is in the wizard and lets them jump back
// to any step already reached — but not ahead of maxUnlocked, which the
// wizard derives from what's actually been completed so far.
function StepIndicator({
  currentStep,
  maxUnlocked,
  onSelect,
}: {
  currentStep: number;
  maxUnlocked: number;
  onSelect: (step: number) => void;
}) {
  return (
    <ol className="flex flex-wrap items-center gap-2 text-sm">
      {STEPS.map((step, index) => {
        const unlocked = step.n <= maxUnlocked;
        const active = step.n === currentStep;
        const completed = step.n < currentStep;
        return (
          <li key={step.n} className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => onSelect(step.n)}
              disabled={!unlocked}
              className={`flex items-center gap-2 rounded-full px-3 py-1.5 transition-colors disabled:cursor-not-allowed ${
                active
                  ? "bg-foreground text-background"
                  : unlocked
                    ? "border border-black/[.1] hover:bg-black/[.04] dark:border-white/[.15] dark:hover:bg-[#1a1a1a]"
                    : "border border-black/[.1] text-zinc-400 dark:border-white/[.15] dark:text-zinc-600"
              }`}
            >
              <span
                className={`flex h-5 w-5 items-center justify-center rounded-full text-xs ${
                  active
                    ? "bg-background text-foreground"
                    : completed
                      ? "bg-foreground text-background"
                      : "bg-black/[.06] dark:bg-white/[.1]"
                }`}
              >
                {step.n}
              </span>
              {step.label}
            </button>
            {index < STEPS.length - 1 && <span className="h-px w-6 bg-black/[.1] dark:bg-white/[.15]" />}
          </li>
        );
      })}
    </ol>
  );
}

// ReportBuilder walks the user from a data source to a saved report: pick the
// table and the fields/filters/sort that make up its query specification,
// preview the rows it returns, pick the template it prints in and map the
// columns onto that template's slots, then save. The specification is what's
// stored and recompiled on every run — no query text ever leaves the browser.
export function ReportBuilder({
  token,
  sources,
  onCreate,
}: {
  token: string;
  sources: DataSource[];
  onCreate: (input: ReportInput) => Promise<void>;
}) {
  const [dataSourceId, setDataSourceId] = useState(sources[0]?.id ?? "");
  const [schema, setSchema] = useState<DataSourceSchema | null>(null);
  const [schemaError, setSchemaError] = useState<string | null>(null);
  // fieldMapping is only meaningful (and only fetched) for a rest_api source:
  // it's what turns raw api_field names into the system-field columns the
  // Data step offers, per columnMetaFor.
  const [fieldMapping, setFieldMapping] = useState<FieldMapping | null>(null);
  const [fieldMappingError, setFieldMappingError] = useState<string | null>(null);
  const [spec, setSpec] = useState<QuerySpec>(emptyQuerySpec());
  const [prompt, setPrompt] = useState("");
  const [preview, setPreview] = useState<QueryPreview | null>(null);
  const [name, setName] = useState("");

  // A report is a PDF unless the user says otherwise, which is what a report
  // was before it could be anything else.
  const [formats, setFormats] = useState<ReportFormat[]>(["pdf"]);

  const [templates, setTemplates] = useState<ReportTemplate[]>([]);
  const [archivedTemplates, setArchivedTemplates] = useState<ReportTemplate[]>([]);
  const [templateId, setTemplateId] = useState("");
  const [templateConfig, setTemplateConfig] = useState<TemplateConfig>({});
  const [layout, setLayout] = useState<string | null>(null);
  // customDraft holds the block composition the user is building or
  // reworking; it's null the rest of the time, when the picker shows the
  // normal template list and field mapping.
  const [customDraft, setCustomDraft] = useState<CustomTemplateDraft | null>(null);

  // llmConfig is loaded once to warn about ai-summary blocks before save
  // rather than letting the request fail — null means "none configured",
  // not "still loading" (loading and none look the same to this warning).
  const [llmConfig, setLlmConfig] = useState<LLMConfig | null>(null);

  const [templatesError, setTemplatesError] = useState<string | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [layoutError, setLayoutError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const [rendering, setRendering] = useState(false);
  const [saving, setSaving] = useState(false);

  // currentStep drives which of the wizard's four stages is on screen. It's
  // clamped below whenever maxUnlocked drops behind it — e.g. changing the
  // data source invalidates everything downstream of step 1.
  const [currentStep, setCurrentStep] = useState(1);

  // loadTemplates reloads both listings — the picker's default cards and its
  // "Show archived" section — so a save, archive, or restore is reflected
  // immediately without a page refresh.
  async function loadTemplates() {
    const [active, archived] = await Promise.all([
      listReportTemplates(token),
      listArchivedReportTemplates(token),
    ]);
    setTemplates(active);
    setArchivedTemplates(archived);
  }

  useEffect(() => {
    Promise.all([listReportTemplates(token), listArchivedReportTemplates(token)])
      .then(([active, archived]) => {
        setTemplates(active);
        setArchivedTemplates(archived);
      })
      .catch((err) => setTemplatesError(errorMessage(err, "Failed to load report templates")));
  }, [token]);

  useEffect(() => {
    getLLMConfig(token)
      .then(setLlmConfig)
      .catch(() => setLlmConfig(null));
  }, [token]);

  useEffect(() => {
    if (!dataSourceId) return;
    let active = true;
    getDataSourceSchema(token, dataSourceId)
      .then((result) => {
        if (active) setSchema(result);
      })
      .catch((err) => {
        if (active) setSchemaError(errorMessage(err, "Failed to read the data source's schema"));
      });
    return () => {
      active = false;
    };
  }, [token, dataSourceId]);

  // The field mapping is only relevant once the schema confirms this is a
  // rest_api source — fetching it earlier would race the schema fetch and
  // fetching it for other source types would be wasted work. handleSourceChange
  // already clears fieldMapping on every source change, so there's nothing to
  // reset here when the type turns out not to be rest_api.
  useEffect(() => {
    if (!dataSourceId || schema?.type !== "rest_api") return;
    let active = true;
    getFieldMapping(token, dataSourceId)
      .then((result) => {
        if (active) setFieldMapping(result);
      })
      .catch((err) => {
        if (active) setFieldMappingError(errorMessage(err, "Failed to load the field mapping"));
      });
    return () => {
      active = false;
    };
  }, [token, dataSourceId, schema?.type]);

  const columnMeta = columnMetaFor(schema, spec.table ?? "", fieldMapping);
  const columns = columnMeta.map((c) => c.name);

  // needsAIProvider blocks saving before the request would fail: whichever
  // template is in play — the design being composed, or an already-saved one
  // just picked — is checked for an ai-summary block, which needs its owner
  // to have an LLM provider configured.
  const selectedTemplate = templates.find((t) => t.id === templateId);
  const blocksInUse = customDraft ? customDraft.blocks : selectedTemplate?.blocks;
  const needsAIProvider = llmConfig === null && hasAISummaryBlock(blocksInUse);

  // Each step's validity gates the next: a table must be chosen before
  // querying it, a preview must succeed before picking a template, and a
  // template must be picked before publishing. maxUnlocked is the furthest
  // step reachable given what's true right now.
  const step1Valid =
    schema !== null &&
    (schema.type === "google_sheets" || schema.type === "rest_api" || (spec.table ?? "") !== "");
  const step2Valid = step1Valid && preview !== null;
  const step3Valid = step2Valid && templateId !== "";
  const maxUnlocked = step3Valid ? 4 : step2Valid ? 3 : step1Valid ? 2 : 1;

  // currentStep can point past maxUnlocked right after something upstream
  // changes (e.g. picking a new data source invalidates the preview) and the
  // render that reflects it hasn't happened yet. Deriving the displayed step
  // here, rather than syncing it back into state via an effect, keeps that
  // window from ever being rendered.
  const step = Math.min(currentStep, maxUnlocked);

  // A mapping only means something against a known set of columns, so it's
  // rebuilt whenever the preview's columns change.
  function applyTemplate(id: string, previewColumns: string[], rows: unknown[][]) {
    const template = templates.find((candidate) => candidate.id === id);
    setTemplateId(id);
    setTemplateConfig(
      template
        ? defaultTemplateConfig(template, previewColumns, numericColumns(previewColumns, rows), name)
        : {},
    );
    setLayout(null);
    setLayoutError(null);
    setCustomDraft(null);
  }

  function resetTemplateState() {
    setTemplateId("");
    setTemplateConfig({});
    setLayout(null);
    setLayoutError(null);
    setCustomDraft(null);
  }

  function openNewCustomTemplate() {
    setCustomDraft({ editingId: null, name: "", description: "", blocks: [], saving: false, error: null });
  }

  function openEditCustomTemplate(template: ReportTemplate) {
    setCustomDraft({
      editingId: template.id,
      name: template.name,
      description: template.description,
      blocks: template.blocks ?? [],
      saving: false,
      error: null,
    });
  }

  // saveCustomTemplate persists the design being built or reworked, then hands
  // off to the same slot-mapping flow every template goes through — saving
  // returns the design's declared slots, so nothing template-specific is
  // needed to pick those up.
  async function saveCustomTemplate() {
    if (!customDraft) return;
    setCustomDraft({ ...customDraft, saving: true, error: null });
    try {
      const input = { name: customDraft.name, description: customDraft.description, blocks: customDraft.blocks };
      const saved = customDraft.editingId === null
        ? await createCustomTemplate(token, input)
        : await updateCustomTemplate(token, customDraft.editingId, input);

      await loadTemplates();
      setCustomDraft(null);
      // Built directly from the save response rather than through
      // applyTemplate: the just-awaited loadTemplates() hasn't re-rendered
      // this closure's `templates` yet, so looking the new template up there
      // would still miss it.
      const previewColumns = preview?.columns ?? [];
      const previewRows = preview?.rows ?? [];
      setTemplateId(saved.id);
      setTemplateConfig(
        defaultTemplateConfig(
          { id: saved.id, name: saved.name, description: saved.description, slots: saved.slots, owned: true, archived: false },
          previewColumns,
          numericColumns(previewColumns, previewRows),
          name,
        ),
      );
      setLayout(null);
      setLayoutError(null);
    } catch (err) {
      setCustomDraft({ ...customDraft, saving: false, error: errorMessage(err, "Failed to save the design") });
    }
  }

  async function handleArchive(id: string) {
    setTemplatesError(null);
    try {
      await archiveReportTemplate(token, id);
      if (id === templateId) resetTemplateState();
      await loadTemplates();
    } catch (err) {
      setTemplatesError(errorMessage(err, "Failed to archive the template"));
    }
  }

  async function handleRestore(id: string) {
    setTemplatesError(null);
    try {
      await restoreReportTemplate(token, id);
      await loadTemplates();
    } catch (err) {
      setTemplatesError(errorMessage(err, "Failed to restore the template"));
    }
  }

  // A specification built against one source means nothing against another.
  // The schema is reset here, at the change itself, rather than in the effect
  // that fetches the new one — dataSourceId only ever changes through this
  // function, so the effect doesn't need to (and, as a set-state-in-effect
  // call, shouldn't) reset it again on the render that follows.
  function handleSourceChange(id: string) {
    setDataSourceId(id);
    setSchema(null);
    setSchemaError(null);
    setFieldMapping(null);
    setFieldMappingError(null);
    setSpec(emptyQuerySpec());
    setPreview(null);
    setPreviewError(null);
    setSaveError(null);
    setSaved(false);
    resetTemplateState();
  }

  async function handlePreview() {
    setPreviewError(null);
    setPreviewing(true);
    try {
      const result = await previewReport(token, dataSourceId, spec);
      setPreview(result);
      setLayout(null);
      // Changing the spec can change what the mapping refers to, so the
      // template is re-applied unless the columns came back identical.
      if (templateId === "" || preview === null || !sameColumns(preview.columns, result.columns)) {
        applyTemplate(templateId === "" ? (templates[0]?.id ?? "") : templateId,
          result.columns, result.rows);
      }
    } catch (err) {
      setPreview(null);
      setPreviewError(errorMessage(err, "Failed to run the query"));
    } finally {
      setPreviewing(false);
    }
  }

  async function handlePreviewLayout() {
    setLayoutError(null);
    setRendering(true);
    try {
      setLayout(await previewReportTemplate(token, { dataSourceId, querySpec: spec, templateId, templateConfig }));
    } catch (err) {
      setLayout(null);
      setLayoutError(errorMessage(err, "Failed to render the template"));
    } finally {
      setRendering(false);
    }
  }

  async function handleSave() {
    setSaveError(null);
    setSaving(true);
    try {
      await onCreate({ name, dataSourceId, prompt, querySpec: spec, templateId, templateConfig, formats });
      setName("");
      setPrompt("");
      setSpec(emptyQuerySpec());
      setPreview(null);
      setFormats(["pdf"]);
      resetTemplateState();
      setSaved(true);
      setCurrentStep(1);
    } catch (err) {
      setSaveError(errorMessage(err, "Failed to save the report"));
    } finally {
      setSaving(false);
    }
  }

  if (sources.length === 0) {
    return (
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Connect a data source before creating a report.
      </p>
    );
  }

  const canPreview =
    spec.fields.length > 0 &&
    (schema?.type === "google_sheets" || schema?.type === "rest_api" || (spec.table ?? "") !== "");

  return (
    <div className="flex w-full flex-col gap-4">
      <StepIndicator currentStep={step} maxUnlocked={maxUnlocked} onSelect={setCurrentStep} />
      {saved && <p className="text-sm text-green-600">Report saved</p>}

      {step === 1 && (
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1">
            <label htmlFor="report-source" className="text-sm font-medium">
              Data source
            </label>
            <select
              id="report-source"
              value={dataSourceId}
              onChange={(e) => handleSourceChange(e.target.value)}
              className={inputClass}
            >
              {sources.map((source) => (
                <option key={source.id} value={source.id}>
                  {source.name}
                </option>
              ))}
            </select>
          </div>

          <div className="flex flex-col gap-1">
            <label htmlFor="report-prompt" className="text-sm font-medium">
              Description (optional)
            </label>
            <textarea
              id="report-prompt"
              rows={2}
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="Sum of sales by region for last month"
              className={inputClass}
            />
          </div>

          {schemaError && <p className="text-sm text-red-600">{schemaError}</p>}

          {schema && schema.type !== "google_sheets" && schema.type !== "rest_api" && (
            <div className="flex flex-col gap-1">
              <label htmlFor="report-table" className="text-sm font-medium">
                Table
              </label>
              <select
                id="report-table"
                value={spec.table ?? ""}
                onChange={(e) => setSpec({ ...emptyQuerySpec(), table: e.target.value })}
                className={inputClass}
              >
                <option value="">Select a table…</option>
                {(schema.tables ?? []).map((table) => (
                  <option key={table.name} value={table.name}>
                    {table.name}
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>
      )}

      {step === 2 && schema && (
        <div className="flex flex-col gap-4">
          {schema.type === "rest_api" && (
            <div className="flex flex-col gap-2 rounded border border-black/[.1] p-4 dark:border-white/[.15]">
              <div>
                <h3 className="text-sm font-medium">Map API fields to system fields</h3>
                <p className="text-sm text-zinc-600 dark:text-zinc-400">
                  Connect this source&apos;s detected fields to DocuWave&apos;s system fields — only mapped
                  fields can be picked below.
                </p>
              </div>
              <DataSourceFieldMappingPanel
                key={dataSourceId}
                token={token}
                dataSourceId={dataSourceId}
                onMappingChange={(mapping) =>
                  setFieldMapping((prev) => (prev ? { ...prev, mapping } : prev))
                }
              />
              {fieldMappingError && <p className="text-sm text-red-600">{fieldMappingError}</p>}
            </div>
          )}

          {columns.length > 0 && (
            <>
              <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                <FieldPicker
                  tableLabel={schema.type === "google_sheets" ? "sheet" : spec.table ?? ""}
                  columns={columnMeta}
                  fields={spec.fields}
                  onChange={(fields) => setSpec({ ...spec, fields })}
                />
                <FieldOrderPanel fields={spec.fields} onChange={(fields) => setSpec({ ...spec, fields })} />
              </div>
              <FilterEditor
                columns={columns}
                filters={spec.filters ?? []}
                onChange={(filters) => setSpec({ ...spec, filters })}
              />
              <SortEditor columns={columns} sorts={spec.sorts ?? []} onChange={(sorts) => setSpec({ ...spec, sorts })} />
              <PlaceholderFilterEditor
                columns={columns}
                filters={spec.placeholderFilters ?? []}
                onChange={(placeholderFilters) => setSpec({ ...spec, placeholderFilters })}
              />
              <div className="flex flex-col gap-1">
                <label htmlFor="report-limit" className="text-sm font-medium">
                  Row limit
                </label>
                <input
                  id="report-limit"
                  type="number"
                  min={1}
                  value={spec.limit ?? ""}
                  onChange={(e) => setSpec({ ...spec, limit: e.target.value ? Number(e.target.value) : undefined })}
                  placeholder="1000 (default)"
                  className={`${inputClass} w-40`}
                />
              </div>
            </>
          )}

          <div>
            <button
              type="button"
              onClick={handlePreview}
              disabled={previewing || !canPreview}
              className="rounded-full bg-foreground px-5 py-2 text-background transition-colors hover:bg-[#383838] disabled:opacity-50 dark:hover:bg-[#ccc]"
            >
              {previewing ? "Running…" : "Run preview"}
            </button>
          </div>
          {previewError && <p className="text-sm text-red-600">{previewError}</p>}
          {preview && (
            <>
              <QueryPreviewTable preview={preview} />
              <p className="font-mono text-xs text-zinc-500 dark:text-zinc-400">{preview.sql}</p>
            </>
          )}
        </div>
      )}

      {step === 3 && schema && (
        <div className="flex flex-col gap-4">
          {templatesError && <p className="text-sm text-red-600">{templatesError}</p>}

          {preview && (
            <TemplatePicker
              templates={templates}
              archivedTemplates={archivedTemplates}
              columns={preview.columns}
              rows={preview.rows}
              templateId={templateId}
              config={templateConfig}
              onSelect={(id) => applyTemplate(id, preview.columns, preview.rows)}
              onConfigChange={setTemplateConfig}
              customDraft={customDraft}
              onOpenNewCustomTemplate={openNewCustomTemplate}
              onOpenEditCustomTemplate={openEditCustomTemplate}
              onCustomDraftChange={setCustomDraft}
              onSaveCustomTemplate={saveCustomTemplate}
              onCancelCustomTemplate={() => setCustomDraft(null)}
              onArchive={handleArchive}
              onRestore={handleRestore}
              token={token}
              schema={schema}
              dataSourceId={dataSourceId}
              querySpec={spec}
            />
          )}

          {needsAIProvider && (
            <p className="text-sm text-amber-600 dark:text-amber-500">
              This template uses an AI summary block. Configure an LLM provider in settings before
              saving this report.
            </p>
          )}

          <div>
            <button
              type="button"
              onClick={handlePreviewLayout}
              disabled={rendering || templateId === ""}
              className="rounded-full border border-black/[.08] px-5 py-2 transition-colors hover:bg-black/[.04] disabled:opacity-50 dark:border-white/[.145] dark:hover:bg-[#1a1a1a]"
            >
              {rendering ? "Rendering…" : "Preview layout"}
            </button>
          </div>
          {layoutError && <p className="text-sm text-red-600">{layoutError}</p>}
          {layout !== null && (
            <iframe
              title="Report layout preview"
              srcDoc={layout}
              // The document is the user's own data rendered by the server;
              // an empty sandbox keeps it from doing anything but display.
              sandbox=""
              className="h-96 w-full rounded border border-black/[.1] dark:border-white/[.15]"
            />
          )}
        </div>
      )}

      {step === 4 && (
        <div className="flex flex-col gap-4">
          <FormatPicker formats={formats} onChange={setFormats} />

          <div className="flex flex-col gap-1">
            <label htmlFor="report-name" className="text-sm font-medium">
              Report name
            </label>
            <input
              id="report-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className={inputClass}
            />
          </div>
          <div>
            <button
              type="button"
              onClick={handleSave}
              disabled={
                saving ||
                name.trim() === "" ||
                preview === null ||
                templateId === "" ||
                formats.length === 0 ||
                needsAIProvider
              }
              className="rounded-full bg-foreground px-5 py-2 text-background transition-colors hover:bg-[#383838] disabled:opacity-50 dark:hover:bg-[#ccc]"
            >
              {saving ? "Saving…" : "Save report"}
            </button>
          </div>
          {saveError && <p className="text-sm text-red-600">{saveError}</p>}
        </div>
      )}

      <div className="flex justify-between border-t border-black/[.1] pt-4 dark:border-white/[.15]">
        <button
          type="button"
          onClick={() => setCurrentStep((s) => Math.max(1, s - 1))}
          disabled={step === 1}
          className={`${removeButtonClass} disabled:opacity-50`}
        >
          Back
        </button>
        {step < 4 && (
          <button
            type="button"
            onClick={() => setCurrentStep((s) => Math.min(4, s + 1))}
            disabled={step >= maxUnlocked}
            className="rounded-full bg-foreground px-5 py-2 text-background transition-colors hover:bg-[#383838] disabled:opacity-50 dark:hover:bg-[#ccc]"
          >
            Next
          </button>
        )}
      </div>
    </div>
  );
}
