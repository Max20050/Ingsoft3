export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function parseErrorMessage(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as { error?: string };
    return body.error ?? response.statusText;
  } catch {
    return response.statusText;
  }
}

export async function register(email: string, password: string): Promise<{ token: string }> {
  const response = await fetch(`${API_URL}/api/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function login(email: string, password: string): Promise<{ token: string }> {
  const response = await fetch(`${API_URL}/api/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function authFetch(
  path: string,
  token: string,
  init: RequestInit = {},
): Promise<Response> {
  const response = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      ...init.headers,
      Authorization: `Bearer ${token}`,
    },
  });
  if (response.status === 401) {
    throw new ApiError(401, "session expired");
  }
  return response;
}

export type DataSourceType = "postgres" | "mysql" | "google_sheets" | "rest_api";

export type RestHeader = {
  key: string;
  value: string;
};

export type RestAuthType = "none" | "basic" | "bearer" | "api_key";

export type RestAuthInput =
  | { type: "none" }
  | { type: "basic"; username: string; password: string }
  | { type: "bearer"; token: string }
  | { type: "api_key"; headerName: string; headerValue: string };

export type DataSource = {
  id: string;
  name: string;
  type: DataSourceType;
  host?: string;
  port?: number;
  dbName?: string;
  username?: string;
  spreadsheetId?: string;
  spreadsheetName?: string;
  url?: string;
  method?: string;
  headers?: RestHeader[];
  authType?: RestAuthType;
  createdAt: string;
};

export type DataSourceInput = {
  name: string;
  type: DataSourceType;
  host: string;
  port: number;
  dbName: string;
  username: string;
  password: string;
};

export type RestApiDataSourceInput = {
  name: string;
  url: string;
  method: string;
  headers: RestHeader[];
  auth: RestAuthInput;
  body?: string;
};

export type GoogleSheetsSpreadsheet = {
  id: string;
  name: string;
};

export type GoogleSheetsDataSourceInput = {
  name: string;
  connectionId: string;
  spreadsheetId: string;
  spreadsheetName: string;
};

export async function listDataSources(token: string): Promise<DataSource[]> {
  const response = await authFetch("/api/datasources", token);
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function testDataSource(token: string, input: DataSourceInput): Promise<void> {
  const response = await authFetch("/api/datasources/test", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}

export async function createDataSource(token: string, input: DataSourceInput): Promise<DataSource> {
  const response = await authFetch("/api/datasources", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function deleteDataSource(token: string, id: string): Promise<void> {
  const response = await authFetch(`/api/datasources/${id}`, token, { method: "DELETE" });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}

export type SchemaColumn = {
  name: string;
  type: string;
};

export type SchemaTable = {
  name: string;
  columns: SchemaColumn[];
};

// SQL sources report `tables`; Google Sheets and REST API sources report
// `fields` (the header row, or the detected response fields).
export type DataSourceSchema = {
  dataSourceId: string;
  type: DataSourceType;
  tables?: SchemaTable[];
  fields?: string[];
  // fieldTypes is a simple inferred type (string/number/boolean/array/object)
  // per field, keyed by field name. Only REST API sources fill this in.
  fieldTypes?: Record<string, string>;
};

export async function getDataSourceSchema(
  token: string,
  id: string,
): Promise<DataSourceSchema> {
  const response = await authFetch(`/api/datasources/${id}/schema`, token);
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function refreshDataSourceSchema(
  token: string,
  id: string,
): Promise<DataSourceSchema> {
  const response = await authFetch(`/api/datasources/${id}/schema/refresh`, token, {
    method: "POST",
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

// SystemField is one of DocuWave's predefined fields a data source's
// detected fields can be mapped onto. Mirrors backend/internal/datasource's
// SystemFields.
export type SystemField = {
  key: string;
  label: string;
};

export type FieldMapping = {
  dataSourceId: string;
  // Keyed by the detected api_field, valued by the system field's key.
  mapping: Record<string, string>;
  systemFields: SystemField[];
  updatedAt?: string;
};

export async function getFieldMapping(token: string, id: string): Promise<FieldMapping> {
  const response = await authFetch(`/api/datasources/${id}/field-mapping`, token);
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function saveFieldMapping(
  token: string,
  id: string,
  mapping: Record<string, string>,
): Promise<FieldMapping> {
  const response = await authFetch(`/api/datasources/${id}/field-mapping`, token, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mapping }),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export function googleSheetsConnectUrl(token: string): string {
  return `${API_URL}/api/datasources/google-sheets/login?token=${encodeURIComponent(token)}`;
}

export async function listGoogleSheetsSpreadsheets(
  token: string,
  connectionId: string,
): Promise<GoogleSheetsSpreadsheet[]> {
  const response = await authFetch(
    `/api/datasources/google-sheets/connections/${connectionId}/spreadsheets`,
    token,
  );
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function createGoogleSheetsDataSource(
  token: string,
  input: GoogleSheetsDataSourceInput,
): Promise<DataSource> {
  const response = await authFetch("/api/datasources/google-sheets", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function testRestApiDataSource(token: string, input: RestApiDataSourceInput): Promise<void> {
  const response = await authFetch("/api/datasources/rest-api/test", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}

export async function createRestApiDataSource(
  token: string,
  input: RestApiDataSourceInput,
): Promise<DataSource> {
  const response = await authFetch("/api/datasources/rest-api", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export type LLMProviderType = "claude" | "openai" | "openrouter";

export type LLMConfig = {
  id: string;
  provider: LLMProviderType;
  createdAt: string;
  updatedAt: string;
};

export type LLMConfigInput = {
  provider: LLMProviderType;
  apiKey: string;
};

export async function getLLMConfig(token: string): Promise<LLMConfig | null> {
  const response = await authFetch("/api/llm-config", token);
  if (response.status === 404) {
    return null;
  }
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function saveLLMConfig(token: string, input: LLMConfigInput): Promise<LLMConfig> {
  const response = await authFetch("/api/llm-config", token, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function deleteLLMConfig(token: string): Promise<void> {
  const response = await authFetch("/api/llm-config", token, { method: "DELETE" });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}

// A slot is a hole in a template's layout the user fills in: "text" takes typed
// text, "column" a single query output column, "columns" an ordered list.
export type TemplateSlotKind = "text" | "column" | "columns";

export type TemplateSlot = {
  key: string;
  label: string;
  kind: TemplateSlotKind;
  description: string;
  required: boolean;
  // numeric marks a slot meant for columns of numbers, such as the measures a
  // template totals. It's a hint for field mapping, not a rule.
  numeric: boolean;
};

// A custom template is composed from this small, extensible block catalog.
// Mirrors backend/internal/template/blocks.go's BlockKind.
export type CustomBlockKind = "table" | "grouped-table" | "kpi-tiles" | "text" | "ai-summary";

export const CUSTOM_BLOCK_KINDS: { value: CustomBlockKind; label: string; description: string }[] = [
  { value: "table", label: "Table", description: "The columns you pick, in order." },
  {
    value: "grouped-table",
    label: "Grouped table",
    description: "One row per group, with the columns you pick totalled.",
  },
  { value: "kpi-tiles", label: "KPI tiles", description: "Headline totals for the columns you pick." },
  { value: "text", label: "Text / note", description: "A freeform title and note. No data binding." },
  {
    value: "ai-summary",
    label: "AI summary",
    description: "Text an AI model writes from a prompt you give it and the data you choose to share.",
  },
];

// AISummaryQuery is one additional query an "ai-summary" block runs purely to
// gather context for its prompt — never shown in the document, only sent to
// the model. Mirrors backend/internal/template/blocks.go's AISummaryQuery.
export type AISummaryQuery = {
  title: string;
  spec: QuerySpec;
};

// CustomBlock is one block in a composed design: its type, its title (which
// also identifies it among blocks of the same type), and — for a text block —
// the freeform note it prints. A block's data-bound slots (which columns feed
// it, its ai-summary prompt) aren't part of the definition: they're mapped
// per report, the same as any template's slots. queries is the one exception,
// since an ai-summary block's additional queries aren't a data mapping onto
// the report's own query — they're queries of their own, part of the design.
export type CustomBlock = {
  id: string;
  kind: CustomBlockKind;
  title: string;
  note?: string;
  queries?: AISummaryQuery[];
};

export type ReportTemplate = {
  id: string;
  name: string;
  description: string;
  slots: TemplateSlot[];
  // owned marks a template as one of the current user's own custom designs —
  // the picker shows these with a "Mine" badge.
  owned: boolean;
  // archived is only ever true in the archived-templates listing; the default
  // listing never includes what a user has archived.
  archived: boolean;
  // blocks is only present for a template that's owned: it's what a "Rework
  // this design" editor needs to reopen it. Built-ins and other users'
  // templates never carry it.
  blocks?: CustomBlock[];
};

export type CustomTemplateInput = {
  name: string;
  description: string;
  blocks: CustomBlock[];
};

// CustomTemplate is the full response from saving a custom template: the
// design as stored, plus the slots it now declares — the same slot shape any
// template exposes, ready to drive the existing mapping UI immediately.
export type CustomTemplate = {
  id: string;
  name: string;
  description: string;
  blocks: CustomBlock[];
  slots: TemplateSlot[];
  createdAt: string;
  updatedAt: string;
};

// TemplateConfig is the user's slot mapping, keyed by slot. It is saved with the
// report, so the shape matches what the backend stores.
export type TemplateConfig = {
  columns?: Record<string, string[]>;
  text?: Record<string, string>;
};

// A report is delivered as one or more files. The options come from the
// server's renderer registry, so this list is the whole of what it can produce.
export type ReportFormat = "pdf" | "xlsx" | "csv";

export const REPORT_FORMATS: { value: ReportFormat; label: string; description: string }[] = [
  { value: "pdf", label: "PDF", description: "The report as a document, laid out for printing." },
  { value: "xlsx", label: "Excel", description: "A worksheet whose numbers are still numbers." },
  { value: "csv", label: "CSV", description: "Plain rows, for loading into another tool." },
];

export type Report = {
  id: string;
  dataSourceId: string;
  dataSourceName: string;
  name: string;
  prompt: string;
  // query is the SQL the server compiled from querySpec, shown for the user to
  // read. It is never sent back — the spec is the source of truth.
  query: string;
  querySpec: QuerySpec;
  templateId: string;
  templateConfig: TemplateConfig;
  formats: ReportFormat[];
  createdAt: string;
  updatedAt: string;
};

// Aggregate is how a field's values are collapsed across a group. Mirrors
// backend/internal/query/query.go's Aggregate.
export type Aggregate = "" | "sum" | "avg" | "min" | "max" | "count";

export const AGGREGATES: { value: Aggregate; label: string }[] = [
  { value: "", label: "None" },
  { value: "sum", label: "Sum" },
  { value: "avg", label: "Average" },
  { value: "min", label: "Min" },
  { value: "max", label: "Max" },
  { value: "count", label: "Count" },
];

// Operator is a filter comparison. Mirrors query.go's Operator.
export type Operator =
  | "eq"
  | "neq"
  | "gt"
  | "gte"
  | "lt"
  | "lte"
  | "contains"
  | "in"
  | "between"
  | "is_null"
  | "is_not_null"
  | "last_days"
  | "this_month"
  | "last_month";

// OperatorArity says how many values an operator's UI needs to collect. Mirrors
// query.go's operatorArities whitelist — kept in sync by hand, since the server
// is the one place that actually enforces it.
export type OperatorArity = "one" | "many" | "pair" | "none" | "count";

export const OPERATORS: { value: Operator; label: string; arity: OperatorArity }[] = [
  { value: "eq", label: "Equals", arity: "one" },
  { value: "neq", label: "Not equals", arity: "one" },
  { value: "gt", label: "Greater than", arity: "one" },
  { value: "gte", label: "Greater than or equal", arity: "one" },
  { value: "lt", label: "Less than", arity: "one" },
  { value: "lte", label: "Less than or equal", arity: "one" },
  { value: "contains", label: "Contains", arity: "one" },
  { value: "in", label: "In (comma separated)", arity: "many" },
  { value: "between", label: "Between", arity: "pair" },
  { value: "is_null", label: "Is empty", arity: "none" },
  { value: "is_not_null", label: "Is not empty", arity: "none" },
  { value: "last_days", label: "In the last N days", arity: "count" },
  { value: "this_month", label: "This month", arity: "none" },
  { value: "last_month", label: "Last month", arity: "none" },
];

export type QueryField = {
  column: string;
  aggregate?: Aggregate;
};

export type QueryFilter = {
  column: string;
  operator: Operator;
  value?: unknown;
  values?: unknown[];
};

export type QuerySort = {
  column: string;
  aggregate?: Aggregate;
  descending?: boolean;
};

// PlaceholderFilter is a filter whose value isn't known yet: it names a
// recipient attribute ("email", "name", or a key into their free-form
// attributes) the value will come from once the report is sent to a specific
// recipient. It never affects preview — the server keeps it out of query
// compilation entirely until a future delivery step resolves it.
export type PlaceholderFilter = {
  column: string;
  operator: Operator;
  recipientField: string;
};

// QuerySpec is a report's query: what to read, from where, and in what shape.
// Mirrors backend/internal/query/query.go's Spec. It never carries query text —
// every identifier in it is checked against the data source's stored schema
// when the server compiles it.
export type QuerySpec = {
  table?: string;
  fields: QueryField[];
  filters?: QueryFilter[];
  placeholderFilters?: PlaceholderFilter[];
  sorts?: QuerySort[];
  limit?: number;
};

export function emptyQuerySpec(): QuerySpec {
  return { table: "", fields: [], filters: [], placeholderFilters: [], sorts: [] };
}

// Cell values come straight from the data source, so anything JSON can hold.
export type QueryPreview = {
  columns: string[];
  rows: unknown[][];
  truncated: boolean;
  // sql is what the spec compiled to, shown so the user can see what their
  // report actually reads.
  sql: string;
  language: string;
};

export type ReportInput = {
  name: string;
  dataSourceId: string;
  prompt: string;
  querySpec: QuerySpec;
  templateId: string;
  templateConfig: TemplateConfig;
  formats: ReportFormat[];
};

export async function listReportTemplates(token: string): Promise<ReportTemplate[]> {
  const response = await authFetch("/api/report-templates", token);
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

// listArchivedReportTemplates returns the templates — built-in or custom —
// this user has archived, for the picker's "Show archived" section.
export async function listArchivedReportTemplates(token: string): Promise<ReportTemplate[]> {
  const response = await authFetch("/api/report-templates/archived", token);
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

// createCustomTemplate saves a block composition as a named, reusable
// template, which then shows up in the picker like any other.
export async function createCustomTemplate(
  token: string,
  input: CustomTemplateInput,
): Promise<CustomTemplate> {
  const response = await authFetch("/api/report-templates", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

// updateCustomTemplate reworks a saved custom template's blocks. It's a live
// reference: every report already using it renders through the new design
// going forward.
export async function updateCustomTemplate(
  token: string,
  id: string,
  input: CustomTemplateInput,
): Promise<CustomTemplate> {
  const response = await authFetch(`/api/report-templates/${id}`, token, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

// archiveReportTemplate hides a template (built-in or custom) from this
// user's default picker listing, without affecting any other account or any
// report that already references it.
export async function archiveReportTemplate(token: string, id: string): Promise<void> {
  const response = await authFetch(`/api/report-templates/${id}/archive`, token, { method: "POST" });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}

// restoreReportTemplate un-archives a template for this user.
export async function restoreReportTemplate(token: string, id: string): Promise<void> {
  const response = await authFetch(`/api/report-templates/${id}/restore`, token, { method: "POST" });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}

// previewReportTemplate renders the template on the server with the rows the
// query actually returns, so the preview is the document the report will be.
export async function previewReportTemplate(
  token: string,
  input: {
    dataSourceId: string;
    querySpec: QuerySpec;
    templateId: string;
    templateConfig: TemplateConfig;
  },
): Promise<string> {
  const response = await authFetch("/api/reports/preview-template", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  const body = (await response.json()) as { html: string };
  return body.html;
}

// previewAISummary is the "Probar resumen" call: it runs the report's query
// (and the block's own additional queries) and asks the model for real,
// right now — the only place a model is called before a report is saved.
export async function previewAISummary(
  token: string,
  input: {
    dataSourceId: string;
    querySpec: QuerySpec;
    columns: string[];
    prompt: string;
    queries: AISummaryQuery[];
  },
): Promise<string> {
  const response = await authFetch("/api/reports/preview-ai-summary", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  const body = (await response.json()) as { text: string };
  return body.text;
}

// previewReport compiles the specification on the server and runs it
// read-only, so the user can see what the report will contain before saving it.
export async function previewReport(
  token: string,
  dataSourceId: string,
  querySpec: QuerySpec,
): Promise<QueryPreview> {
  const response = await authFetch("/api/reports/preview", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ dataSourceId, querySpec }),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function listReports(token: string): Promise<Report[]> {
  const response = await authFetch("/api/reports", token);
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function createReport(token: string, input: ReportInput): Promise<Report> {
  const response = await authFetch("/api/reports", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

// downloadReport runs the report on the server and saves the file it returns.
// The request carries the session token, so the file arrives as a response body
// rather than at a URL the browser could follow on its own.
export async function downloadReport(
  token: string,
  id: string,
  format: ReportFormat,
): Promise<{ blob: Blob; filename: string }> {
  const response = await authFetch(`/api/reports/${id}/download?format=${format}`, token);
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return {
    blob: await response.blob(),
    filename: filenameFrom(response.headers.get("Content-Disposition"), `${id}.${format}`),
  };
}

// filenameFrom reads the name the server chose for the file. The header is
// exposed to scripts by the API's CORS configuration; if it isn't there, the
// caller's fallback is used rather than guessing.
function filenameFrom(disposition: string | null, fallback: string): string {
  const quoted = disposition?.match(/filename="([^"]+)"/);
  return quoted?.[1] ?? fallback;
}

export async function deleteReport(token: string, id: string): Promise<void> {
  const response = await authFetch(`/api/reports/${id}`, token, { method: "DELETE" });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}

// A recipient's attributes are free-form values a report's inputs can be
// filled from at send time (e.g. "region"), so anything JSON can hold.
export type Recipient = {
  id: string;
  email: string;
  name: string;
  attributes: Record<string, unknown>;
  createdAt: string;
};

export type RecipientInput = {
  email: string;
  name: string;
  attributes: Record<string, unknown>;
};

export type RecipientGroup = {
  id: string;
  name: string;
  createdAt: string;
};

export async function listRecipients(token: string): Promise<Recipient[]> {
  const response = await authFetch("/api/recipients", token);
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function createRecipient(token: string, input: RecipientInput): Promise<Recipient> {
  const response = await authFetch("/api/recipients", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function deleteRecipient(token: string, id: string): Promise<void> {
  const response = await authFetch(`/api/recipients/${id}`, token, { method: "DELETE" });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}

export async function listRecipientGroups(token: string): Promise<RecipientGroup[]> {
  const response = await authFetch("/api/recipient-groups", token);
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function createRecipientGroup(token: string, name: string): Promise<RecipientGroup> {
  const response = await authFetch("/api/recipient-groups", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function deleteRecipientGroup(token: string, id: string): Promise<void> {
  const response = await authFetch(`/api/recipient-groups/${id}`, token, { method: "DELETE" });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}

export async function listGroupMembers(token: string, groupId: string): Promise<Recipient[]> {
  const response = await authFetch(`/api/recipient-groups/${groupId}/members`, token);
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
  return response.json();
}

export async function addGroupMember(token: string, groupId: string, recipientId: string): Promise<void> {
  const response = await authFetch(`/api/recipient-groups/${groupId}/members`, token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ recipientId }),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}

export async function removeGroupMember(token: string, groupId: string, recipientId: string): Promise<void> {
  const response = await authFetch(`/api/recipient-groups/${groupId}/members/${recipientId}`, token, {
    method: "DELETE",
  });
  if (!response.ok) {
    throw new ApiError(response.status, await parseErrorMessage(response));
  }
}
