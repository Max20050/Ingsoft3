package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Max20050/docuwave/internal/auth"
	"github.com/Max20050/docuwave/internal/datasource"
	"github.com/Max20050/docuwave/internal/query"
	"github.com/Max20050/docuwave/internal/render"
	"github.com/Max20050/docuwave/internal/template"
)

func TestHandlersRequireAuthentication(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil, nil, nil, nil, false)

	cases := map[string]struct {
		handler http.HandlerFunc
		request *http.Request
	}{
		"preview":                 {handlers.Preview, httptest.NewRequest(http.MethodPost, "/api/reports/preview", nil)},
		"list templates":          {handlers.ListTemplates, httptest.NewRequest(http.MethodGet, "/api/report-templates", nil)},
		"list archived templates": {handlers.ListArchivedTemplates, httptest.NewRequest(http.MethodGet, "/api/report-templates/archived", nil)},
		"create custom template":  {handlers.CreateCustomTemplate, httptest.NewRequest(http.MethodPost, "/api/report-templates", nil)},
		"update custom template":  {handlers.UpdateCustomTemplate, httptest.NewRequest(http.MethodPut, "/api/report-templates/c-1", nil)},
		"archive template":        {handlers.ArchiveTemplate, httptest.NewRequest(http.MethodPost, "/api/report-templates/tabular/archive", nil)},
		"restore template":        {handlers.RestoreTemplate, httptest.NewRequest(http.MethodPost, "/api/report-templates/tabular/restore", nil)},
		"preview template":        {handlers.PreviewTemplate, httptest.NewRequest(http.MethodPost, "/api/reports/preview-template", nil)},
		"preview ai summary":      {handlers.PreviewAISummary, httptest.NewRequest(http.MethodPost, "/api/reports/preview-ai-summary", nil)},
		"create":                  {handlers.Create, httptest.NewRequest(http.MethodPost, "/api/reports", nil)},
		"list":                    {handlers.List, httptest.NewRequest(http.MethodGet, "/api/reports", nil)},
		"download":                {handlers.Download, httptest.NewRequest(http.MethodGet, "/api/reports/r-1/download", nil)},
		"delete":                  {handlers.Delete, httptest.NewRequest(http.MethodDelete, "/api/reports/r-1", nil)},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tc.handler(recorder, tc.request)

			if recorder.Code != http.StatusUnauthorized {
				t.Errorf("got status %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestMissingField(t *testing.T) {
	complete := createReportRequest{
		Name:         "Monthly sales",
		DataSourceID: "ds-1",
		TemplateID:   "tabular",
	}
	if got := missingField(complete); got != "" {
		t.Errorf("got %q for a complete request, want no missing field", got)
	}

	tests := map[string]createReportRequest{
		"name":         {DataSourceID: "ds-1", TemplateID: "tabular"},
		"dataSourceId": {Name: "n", TemplateID: "tabular"},
		"templateId":   {Name: "n", DataSourceID: "ds-1"},
	}
	for want, req := range tests {
		t.Run(want, func(t *testing.T) {
			if got := missingField(req); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}

	// A description is the user's own note about the report, not an input to
	// building it, so a report without one is fine.
	if got := missingField(createReportRequest{Name: "n", DataSourceID: "ds-1", TemplateID: "tabular"}); got != "" {
		t.Errorf("got %q for a report with no description, want no missing field", got)
	}
}

// The user needs to know whether to fix their settings, retry, or rewrite
// their description, so each generation failure gets its own status.
// A query that the schema can't answer is the user's to fix, and the message has
// to say which part — usually a column that's gone, or a stale schema.
func TestWriteQueryError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "a specification the schema doesn't support",
			err:        fmt.Errorf(`%w: "profit" is not a column`, query.ErrInvalidSpec),
			wantStatus: http.StatusBadRequest,
			wantBody:   "profit",
		},
		{
			name:       "a source with no query compiler",
			err:        fmt.Errorf("%w: mssql", query.ErrUnsupportedSource),
			wantStatus: http.StatusBadRequest,
			wantBody:   "mssql",
		},
		{
			name:       "the source could not be read",
			err:        fmt.Errorf("%w: connection refused", datasource.ErrIntrospectFailed),
			wantStatus: http.StatusBadGateway,
			wantBody:   "connection refused",
		},
		{
			name:       "a data source that isn't there",
			err:        datasource.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantBody:   "data source not found",
		},
		{
			name:       "anything else is a server fault",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "failed to prepare",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeQueryError(recorder, tt.err)

			if recorder.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", recorder.Code, tt.wantStatus)
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Errorf("body %q does not mention %q", recorder.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestWriteResolveError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"missing data source", datasource.ErrNotFound, http.StatusNotFound},
		{"missing sheets connection", datasource.ErrConnectionNotFound, http.StatusNotFound},
		{"anything else", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeResolveError(recorder, tt.err)

			if recorder.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}

func TestWriteTemplateError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"template that isn't registered", template.ErrUnknownTemplate, http.StatusBadRequest},
		{"mapping the template can't use", template.ErrInvalidConfig, http.StatusBadRequest},
		{"anything else is a server fault", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeTemplateError(recorder, tt.err)

			if recorder.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}

// The UI builds its template picker and its field mapping controls from this
// response, so it has to carry every slot.
func TestListTemplates(t *testing.T) {
	handlers := NewHandlers(nil, nil, template.NewRegistrySource(template.NewRegistry(template.Starters()...)), nil, nil, nil, false)

	recorder := httptest.NewRecorder()
	handlers.ListTemplates(recorder, authenticatedRequest(t, http.MethodGet, "/api/report-templates", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", recorder.Code, http.StatusOK)
	}

	var metas []template.Meta
	if err := json.NewDecoder(recorder.Body).Decode(&metas); err != nil {
		t.Fatalf("decode returned error: %v", err)
	}
	if len(metas) != len(template.Starters()) {
		t.Fatalf("got %d templates, want %d", len(metas), len(template.Starters()))
	}
	for _, meta := range metas {
		if meta.ID == "" || meta.Name == "" || len(meta.Slots) == 0 {
			t.Errorf("template %+v is not usable by the UI", meta)
		}
	}
}

// A custom template needs a name — the same rule reports themselves already
// follow — checked before it ever reaches the store.
func TestCreateCustomTemplateRequiresAName(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil, nil, nil, nil, false)

	body, err := json.Marshal(customTemplateRequest{Description: "no name"})
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	handlers.CreateCustomTemplate(recorder,
		authenticatedRequest(t, http.MethodPost, "/api/report-templates", bytes.NewReader(body)))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "name") {
		t.Errorf("body %q does not name the missing field", recorder.Body.String())
	}
}

// A block naming a kind outside the catalog is rejected before it's saved,
// for both creating and updating a custom template.
func TestCustomTemplateRejectsUnknownBlockKind(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil, nil, nil, nil, false)
	req := customTemplateRequest{
		Name:   "Bad design",
		Blocks: []template.BlockDef{{Kind: "chart", Title: "Nope"}},
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	for name, handler := range map[string]http.HandlerFunc{
		"create": handlers.CreateCustomTemplate,
		"update": handlers.UpdateCustomTemplate,
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := authenticatedRequest(t, http.MethodPost, "/api/report-templates", bytes.NewReader(body))
			request.SetPathValue("id", "c-1")
			handler(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("got status %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			if !strings.Contains(recorder.Body.String(), "chart") {
				t.Errorf("body %q does not name the offending block kind", recorder.Body.String())
			}
		})
	}
}

// ai-summary blocks are rejected while the feature flag is off, the same way
// an unknown block kind is — the product isn't meant to ship this to anyone
// until a cost estimator exists.
func TestCustomTemplateRejectsAISummaryBlocksWhenDisabled(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil, nil, nil, nil, false)
	req := customTemplateRequest{
		Name:   "Design",
		Blocks: []template.BlockDef{{Kind: template.BlockAISummary, Title: "Insight"}},
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := authenticatedRequest(t, http.MethodPost, "/api/report-templates", bytes.NewReader(body))
	handlers.CreateCustomTemplate(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), ErrAISummaryDisabled.Error()) {
		t.Errorf("body %q does not explain the block is disabled", recorder.Body.String())
	}
}

// checkAISummaryBlocks is the one gate every save path for a block
// composition runs through, so it's tested directly for both outcomes rather
// than only indirectly through the handlers above.
func TestCheckAISummaryBlocks(t *testing.T) {
	blocks := []template.BlockDef{{Kind: template.BlockAISummary}}

	disabled := &Handlers{aiSummaryEnabled: false}
	if err := disabled.checkAISummaryBlocks(blocks); !errors.Is(err, ErrAISummaryDisabled) {
		t.Errorf("got %v, want ErrAISummaryDisabled while the flag is off", err)
	}

	enabled := &Handlers{aiSummaryEnabled: true}
	if err := enabled.checkAISummaryBlocks(blocks); err != nil {
		t.Errorf("got %v, want no error once the flag is on", err)
	}
	if err := disabled.checkAISummaryBlocks([]template.BlockDef{{Kind: template.BlockTable}}); err != nil {
		t.Errorf("got %v, want no error for a design with no ai-summary block", err)
	}
}

// Archiving and restoring both need the ID in the path.
func TestArchiveAndRestoreTemplateRequireAnID(t *testing.T) {
	handlers := NewHandlers(nil, nil, nil, nil, nil, nil, false)

	for name, handler := range map[string]http.HandlerFunc{
		"archive": handlers.ArchiveTemplate,
		"restore": handlers.RestoreTemplate,
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler(recorder, authenticatedRequest(t, http.MethodPost, "/api/report-templates//archive", nil))

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("got status %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}

// The request is checked before the query runs, so a half-filled preview never
// reaches the user's data source.
func TestPreviewTemplateRejectsIncompleteRequests(t *testing.T) {
	handlers := NewHandlers(nil, nil, template.NewRegistrySource(template.NewRegistry(template.Starters()...)), nil, nil, nil, false)

	spec := query.Spec{Table: "sales", Fields: []query.Field{{Column: "region"}}}
	tests := map[string]previewTemplateRequest{
		"dataSourceId": {QuerySpec: spec, TemplateID: "tabular"},
		"templateId":   {DataSourceID: "ds-1", QuerySpec: spec},
	}

	for field, req := range tests {
		t.Run(field, func(t *testing.T) {
			body, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("marshal returned error: %v", err)
			}

			recorder := httptest.NewRecorder()
			handlers.PreviewTemplate(recorder,
				authenticatedRequest(t, http.MethodPost, "/api/reports/preview-template", bytes.NewReader(body)))

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("got status %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			if !strings.Contains(recorder.Body.String(), field) {
				t.Errorf("body %q does not name the missing field %q", recorder.Body.String(), field)
			}
		})
	}
}

func TestReportResponseJSONShape(t *testing.T) {
	created := time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC)
	body, err := json.Marshal(toResponse(Report{
		ID:             "r-1",
		DataSourceID:   "ds-1",
		DataSourceName: "Warehouse",
		Name:           "Monthly sales",
		Prompt:         "sales by region, excluding refunds",
		Query:          "SELECT \"region\" AS \"region\"\nFROM \"sales\"\nLIMIT 1000",
		QuerySpec: query.Spec{
			Table:  "sales",
			Fields: []query.Field{{Column: "region"}, {Column: "total", Aggregate: query.AggregateSum}},
			Limit:  1000,
		},
		TemplateID: "grouped-totals",
		TemplateConfig: template.Config{
			Columns: map[string][]string{"group": {"region"}},
			Text:    map[string]string{"title": "Monthly sales"},
		},
		Formats:   []render.Format{render.FormatPDF, render.FormatCSV},
		CreatedAt: created,
		UpdatedAt: created,
	}))
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	want := `{"id":"r-1","dataSourceId":"ds-1","dataSourceName":"Warehouse","name":"Monthly sales",` +
		`"prompt":"sales by region, excluding refunds",` +
		`"query":"SELECT \"region\" AS \"region\"\nFROM \"sales\"\nLIMIT 1000",` +
		`"querySpec":{"table":"sales","fields":[{"column":"region"},{"column":"total","aggregate":"sum"}],"limit":1000},` +
		`"templateId":"grouped-totals",` +
		`"templateConfig":{"columns":{"group":["region"]},"text":{"title":"Monthly sales"}},` +
		`"formats":["pdf","csv"],` +
		`"createdAt":"2026-08-12T09:30:00Z","updatedAt":"2026-08-12T09:30:00Z"}`
	if string(body) != want {
		t.Errorf("got %s, want %s", body, want)
	}
}

// A report saved before query building replaced LLM-written SQL keeps its text
// but has no specification, and nothing will run it as text.
func TestLegacyReportsAreNotRunnable(t *testing.T) {
	legacy := Report{
		ID:    "r-legacy",
		Name:  "Old report",
		Query: "SELECT * FROM sales",
	}

	if !legacy.QuerySpec.IsZero() {
		t.Error("a report with no specification should report itself as empty")
	}
	// The UI reads this to mark the report as needing a rebuild.
	if got := toResponse(legacy).QuerySpec; len(got.Fields) != 0 {
		t.Errorf("got %+v, want an empty specification", got)
	}
}

// A report is delivered as the files its client asked for, so the request says
// which — and a request that asks for nothing, or for something this server
// can't produce, is refused rather than quietly given a default.
func TestChosenFormats(t *testing.T) {
	tests := []struct {
		name    string
		request createReportRequest
		want    []render.Format
		wantErr error
	}{
		{
			name:    "a request that leaves formats out gets a PDF",
			request: createReportRequest{Name: "n"},
			want:    []render.Format{render.FormatPDF},
		},
		{
			name:    "the formats asked for",
			request: createReportRequest{Formats: []string{"csv", "xlsx"}},
			want:    []render.Format{render.FormatXLSX, render.FormatCSV},
		},
		{
			name:    "asking for nothing is a mistake, not a default",
			request: createReportRequest{Formats: []string{}},
			wantErr: render.ErrNoFormats,
		},
		{
			name:    "a format this server cannot produce",
			request: createReportRequest{Formats: []string{"pdf", "docx"}},
			wantErr: render.ErrUnknownFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := chosenFormats(tt.request)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("chosenFormats returned error: %v", err)
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
}

// A report is rendered unattended on a schedule, so each way it can fail has to
// tell the user whether it's theirs to fix, ours, or their data source's.
func TestWriteRenderError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "a report saved before query building",
			err:        ErrNotRunnable,
			wantStatus: http.StatusConflict,
			wantBody:   "rebuilt",
		},
		{
			name:       "a format the report is not configured for",
			err:        fmt.Errorf("%w: this report is not configured for csv", render.ErrUnknownFormat),
			wantStatus: http.StatusBadRequest,
			wantBody:   "csv",
		},
		{
			name:       "the data source could not answer",
			err:        fmt.Errorf("%w: connection refused", ErrQueryFailed),
			wantStatus: http.StatusBadGateway,
			wantBody:   "connection refused",
		},
		{
			name:       "a mapping the query no longer supports",
			err:        fmt.Errorf("%w: mapped to \"profit\"", template.ErrInvalidConfig),
			wantStatus: http.StatusBadRequest,
			wantBody:   "profit",
		},
		{
			name:       "a data source that is gone",
			err:        datasource.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantBody:   "data source not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeRenderError(recorder, tt.err)

			if recorder.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", recorder.Code, tt.wantStatus)
			}
			if !strings.Contains(recorder.Body.String(), tt.wantBody) {
				t.Errorf("body %q does not mention %q", recorder.Body.String(), tt.wantBody)
			}
		})
	}
}

// The report has to be loaded before anything runs, so a download of one that
// isn't there never reaches the user's data source.
func TestDownloadRequiresAnID(t *testing.T) {
	handlers := NewHandlers(nil, nil, template.NewRegistrySource(template.NewRegistry(template.Starters()...)), nil, nil, nil, false)

	recorder := httptest.NewRecorder()
	handlers.Download(recorder, authenticatedRequest(t, http.MethodGet, "/api/reports//download", nil))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

// authenticatedRequest builds a request carrying a valid session, which is how
// the handlers get a user ID out of the context.
func authenticatedRequest(t *testing.T, method, target string, body io.Reader) *http.Request {
	t.Helper()

	issuer := auth.NewTokenIssuer("test-secret")
	token, err := issuer.IssueToken("u-1")
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	request := httptest.NewRequest(method, target, body)
	request.Header.Set("Authorization", "Bearer "+token)

	// RequireAuth owns the context key the handlers read, so the way to a request
	// carrying a user ID is through the middleware itself.
	var authenticated *http.Request
	auth.NewHandlers(nil, issuer).RequireAuth(func(_ http.ResponseWriter, r *http.Request) {
		authenticated = r
	})(httptest.NewRecorder(), request)
	if authenticated == nil {
		t.Fatal("RequireAuth rejected a valid session")
	}
	return authenticated
}
