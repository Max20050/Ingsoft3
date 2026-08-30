package report

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/Max20050/docuwave/internal/auth"
	"github.com/Max20050/docuwave/internal/datasource"
	"github.com/Max20050/docuwave/internal/llm"
	"github.com/Max20050/docuwave/internal/query"
	"github.com/Max20050/docuwave/internal/render"
	"github.com/Max20050/docuwave/internal/template"
)

// Handlers exposes HTTP handlers for the templates a report renders through,
// for managing report configurations, and for downloading a report's files.
//
// A report's query is never accepted as text. It arrives as a specification,
// which the runner compiles against the data source's stored schema — so the
// only SQL that runs is SQL this server assembled.
type Handlers struct {
	store     *Store
	runner    *Runner
	templates template.Source
	// custom and archive back the template picker's "Build my own design" and
	// "Show archived" affordances. They're nil in tests that only exercise the
	// report and starter-template paths, which never reach them.
	custom  *template.CustomStore
	archive *template.ArchiveStore
	// summaries generates ai-summary block text for the "Probar resumen" test
	// call. It's nil in tests that never exercise that path.
	summaries *llm.Generator
	// aiSummaryEnabled gates every ai-summary-block affordance behind a single
	// switch: the product isn't meant to ship this to anyone until a cost
	// estimator exists, so until then this stays off outside of whoever
	// explicitly turned it on to develop against it.
	aiSummaryEnabled bool
}

func NewHandlers(
	store *Store,
	runner *Runner,
	templates template.Source,
	custom *template.CustomStore,
	archive *template.ArchiveStore,
	summaries *llm.Generator,
	aiSummaryEnabled bool,
) *Handlers {
	return &Handlers{
		store: store, runner: runner, templates: templates, custom: custom, archive: archive,
		summaries: summaries, aiSummaryEnabled: aiSummaryEnabled,
	}
}

// ErrAISummaryDisabled is returned for any attempt to save an ai-summary
// block while the feature is switched off.
var ErrAISummaryDisabled = errors.New("AI summary blocks are not available yet")

// checkAISummaryBlocks rejects blocks the feature flag doesn't allow yet.
// PrepareBlocks already proved every block names a known catalog kind; this
// is the one further check specific to a kind still behind a flag.
func (h *Handlers) checkAISummaryBlocks(blocks []template.BlockDef) error {
	if h.aiSummaryEnabled {
		return nil
	}
	for _, b := range blocks {
		if b.Kind == template.BlockAISummary {
			return ErrAISummaryDisabled
		}
	}
	return nil
}

// requireAIProviderIfNeeded rejects saving a report against a template that
// has ai-summary blocks when its owner has no LLM provider configured yet —
// better to say so now than to have every future run of a scheduled report
// print "could not generate this summary" unattended.
func (h *Handlers) requireAIProviderIfNeeded(ctx context.Context, userID string, t template.Template) error {
	if len(template.AISummaryBlocks(t)) == 0 {
		return nil
	}
	if h.summaries == nil {
		return ErrAISummaryDisabled
	}
	has, err := h.summaries.HasProvider(ctx, userID)
	if err != nil {
		return err
	}
	if !has {
		return fmt.Errorf("this report uses an AI summary block: configure an LLM provider first")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeResolveError maps a data source lookup failure onto an HTTP response.
func writeResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, datasource.ErrNotFound):
		writeError(w, http.StatusNotFound, "data source not found")
	case errors.Is(err, datasource.ErrConnectionNotFound):
		writeError(w, http.StatusNotFound, "google sheets connection not found")
	default:
		writeError(w, http.StatusInternalServerError, "failed to prepare data source connection")
	}
}

// writeQueryError maps a failure to build a query onto an HTTP response. A spec
// the schema doesn't support is the user's to fix and says so specifically,
// because the fix is usually to pick a different column or refresh the schema.
func writeQueryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, query.ErrInvalidSpec), errors.Is(err, query.ErrUnsupportedSource):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, datasource.ErrIntrospectFailed):
		writeError(w, http.StatusBadGateway, err.Error())
	default:
		writeResolveError(w, err)
	}
}

// writeTemplateError maps a template failure onto an HTTP response. A missing
// template or an unusable mapping is the caller's to fix; a render fault is ours.
func writeTemplateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, template.ErrUnknownTemplate), errors.Is(err, template.ErrInvalidConfig):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "failed to render the template")
	}
}

type reportResponse struct {
	ID             string          `json:"id"`
	DataSourceID   string          `json:"dataSourceId"`
	DataSourceName string          `json:"dataSourceName"`
	Name           string          `json:"name"`
	Prompt         string          `json:"prompt"`
	// Query is the SQL this server compiled from QuerySpec, shown to the user so
	// they can see exactly what their report reads. It is never accepted back.
	Query          string          `json:"query"`
	QuerySpec      query.Spec      `json:"querySpec"`
	TemplateID     string          `json:"templateId"`
	TemplateConfig template.Config `json:"templateConfig"`
	// Formats are the files this report is delivered as, and the ones it can be
	// downloaded in.
	Formats   []string `json:"formats"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

func toResponse(rep Report) reportResponse {
	return reportResponse{
		ID:             rep.ID,
		DataSourceID:   rep.DataSourceID,
		DataSourceName: rep.DataSourceName,
		Name:           rep.Name,
		Prompt:         rep.Prompt,
		Query:          rep.Query,
		QuerySpec:      rep.QuerySpec,
		TemplateID:     rep.TemplateID,
		TemplateConfig: rep.TemplateConfig,
		Formats:        render.Strings(rep.Formats),
		CreatedAt:      rep.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      rep.UpdatedAt.Format(time.RFC3339),
	}
}

// writeRenderError maps a failure to produce a report's files onto an HTTP
// response.
func writeRenderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotRunnable):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, render.ErrUnknownFormat), errors.Is(err, render.ErrNoFormats):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrQueryFailed):
		// The report's configuration was checked when it was saved and its query
		// is this server's own, so a failure now is the source's end: it's
		// unreachable, or the data behind the report has changed under it.
		writeError(w, http.StatusBadGateway, err.Error())
	case errors.Is(err, template.ErrUnknownTemplate), errors.Is(err, template.ErrInvalidConfig):
		writeTemplateError(w, err)
	default:
		writeQueryError(w, err)
	}
}

// templateResponse is a template's Meta plus the two things Meta itself
// doesn't carry because they're facts about the requesting user, not the
// template: whether it's one of theirs, and whether they've archived it.
type templateResponse struct {
	template.Meta
	Owned    bool `json:"owned"`
	Archived bool `json:"archived"`
	// Blocks is only set for a template this user owns — what a "rework this
	// design" editor needs to reopen it. Built-ins and other users' custom
	// templates never carry it.
	Blocks []template.BlockDef `json:"blocks,omitempty"`
}

// withBlocks fills in a custom template's own blocks when the response is for
// a template this user owns, so the picker can reopen it for editing without
// a second endpoint.
func (h *Handlers) withBlocks(ctx context.Context, userID string, resp templateResponse) templateResponse {
	if !resp.Owned || h.custom == nil {
		return resp
	}
	if custom, err := h.custom.Get(ctx, userID, resp.ID); err == nil {
		resp.Blocks = custom.Blocks
	}
	return resp
}

// ownedIDs returns the set of template IDs that are the given user's own
// custom templates, for marking cards "Mine" in the picker.
func (h *Handlers) ownedIDs(ctx context.Context, userID string) (map[string]bool, error) {
	if h.custom == nil {
		return map[string]bool{}, nil
	}
	customs, err := h.custom.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	owned := make(map[string]bool, len(customs))
	for _, c := range customs {
		owned[c.ID] = true
	}
	return owned, nil
}

// ListTemplates handles GET /api/report-templates: the templates a report can
// be rendered through — every built-in plus the user's own custom templates,
// minus whichever either they've archived — each with the slots the user maps
// their query output onto.
func (h *Handlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	metas, err := h.templates.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}
	owned, err := h.ownedIDs(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}

	resp := make([]templateResponse, 0, len(metas))
	for _, meta := range metas {
		resp = append(resp, h.withBlocks(r.Context(), userID, templateResponse{Meta: meta, Owned: owned[meta.ID]}))
	}
	writeJSON(w, http.StatusOK, resp)
}

// ListArchivedTemplates handles GET /api/report-templates/archived: the
// templates — built-in or custom — this user has archived, for the picker's
// collapsible "Show archived" section and its restore action.
func (h *Handlers) ListArchivedTemplates(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if h.archive == nil {
		writeJSON(w, http.StatusOK, []templateResponse{})
		return
	}

	archived, err := h.archive.Archived(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list archived templates")
		return
	}
	owned, err := h.ownedIDs(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list archived templates")
		return
	}

	resp := make([]templateResponse, 0, len(archived))
	for id := range archived {
		t, err := h.templates.Get(r.Context(), userID, id)
		if err != nil {
			// The template itself is gone (a custom template deleted outright,
			// which this scope doesn't otherwise do) — nothing to restore into,
			// so it's dropped from the list rather than surfaced as an error.
			continue
		}
		resp = append(resp,
			h.withBlocks(r.Context(), userID, templateResponse{Meta: t.Meta(), Owned: owned[id], Archived: true}))
	}
	sort.Slice(resp, func(i, j int) bool { return resp[i].Name < resp[j].Name })
	writeJSON(w, http.StatusOK, resp)
}

// ArchiveTemplate handles POST /api/report-templates/{id}/archive: hides a
// template — built-in or custom — from this user's picker without affecting
// any other account or any report that already references it.
func (h *Handlers) ArchiveTemplate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if _, err := h.templates.Get(r.Context(), userID, id); err != nil {
		writeTemplateError(w, err)
		return
	}
	if err := h.archive.Archive(r.Context(), userID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive template")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RestoreTemplate handles POST /api/report-templates/{id}/restore: un-archives
// a template for this user.
func (h *Handlers) RestoreTemplate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.archive.Restore(r.Context(), userID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore template")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type customTemplateRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Blocks      []template.BlockDef `json:"blocks"`
}

type customTemplateResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Blocks      []template.BlockDef `json:"blocks"`
	// Slots is the same slot list the template's Meta declares, included here
	// so the builder can show the mapping controls its own blocks will need
	// without a second round trip.
	Slots     []template.Slot `json:"slots"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
}

func toCustomTemplateResponse(t template.SavedCustomTemplate) customTemplateResponse {
	return customTemplateResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Blocks:      t.Blocks,
		Slots:       t.Meta().Slots,
		CreatedAt:   t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   t.UpdatedAt.Format(time.RFC3339),
	}
}

// CreateCustomTemplate handles POST /api/report-templates: saves a block
// composition the user built inside a report as a named, reusable template.
func (h *Handlers) CreateCustomTemplate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req customTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	blocks, err := template.PrepareBlocks(req.Blocks)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.checkAISummaryBlocks(blocks); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.custom.Create(r.Context(), userID, template.CustomTemplate{
		Name:        req.Name,
		Description: req.Description,
		Blocks:      blocks,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save template")
		return
	}
	writeJSON(w, http.StatusCreated, toCustomTemplateResponse(created))
}

// UpdateCustomTemplate handles PUT /api/report-templates/{id}: reworks a saved
// custom template's blocks. Because a template is a live reference, this
// changes every report that already uses it, going forward.
func (h *Handlers) UpdateCustomTemplate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var req customTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	blocks, err := template.PrepareBlocks(req.Blocks)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.checkAISummaryBlocks(blocks); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.custom.Update(r.Context(), userID, id, template.CustomTemplate{
		Name:        req.Name,
		Description: req.Description,
		Blocks:      blocks,
	})
	if err != nil {
		if errors.Is(err, template.ErrUnknownTemplate) {
			writeError(w, http.StatusNotFound, "template not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to save template")
		return
	}
	writeJSON(w, http.StatusOK, toCustomTemplateResponse(updated))
}

type previewRequest struct {
	DataSourceID string     `json:"dataSourceId"`
	QuerySpec    query.Spec `json:"querySpec"`
}

type previewResponse struct {
	datasource.QueryResult
	// SQL is what the specification compiled to, so the user can see what their
	// report actually reads.
	SQL      string `json:"sql"`
	Language string `json:"language"`
}

// previewLimit keeps a preview to a page of rows without ever showing more than
// the report itself would cover.
func previewLimit(spec query.Spec) int {
	if spec.Limit > 0 && spec.Limit < datasource.PreviewRowLimit {
		return spec.Limit
	}
	return datasource.PreviewRowLimit
}

// Preview handles POST /api/reports/preview: it compiles the specification,
// runs it read-only, and returns the first rows along with the SQL it built, so
// the user can see what the report will contain before saving it.
func (h *Handlers) Preview(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DataSourceID == "" {
		writeError(w, http.StatusBadRequest, "dataSourceId is required")
		return
	}

	prepared, err := h.runner.prepare(r.Context(), userID, req.DataSourceID, req.QuerySpec)
	if err != nil {
		writeQueryError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()

	result, err := prepared.run(ctx, previewLimit(req.QuerySpec))
	if err != nil {
		// The query is ours, so a failure here is the source rejecting it —
		// usually an aggregate over a column that doesn't hold numbers. Its own
		// message is the useful one.
		writeError(w, http.StatusBadRequest, fmt.Sprintf("query failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, previewResponse{
		QueryResult: result,
		SQL:         prepared.compiled.Text,
		Language:    prepared.language,
	})
}

type previewTemplateRequest struct {
	DataSourceID   string          `json:"dataSourceId"`
	QuerySpec      query.Spec      `json:"querySpec"`
	TemplateID     string          `json:"templateId"`
	TemplateConfig template.Config `json:"templateConfig"`
}

type previewTemplateResponse struct {
	HTML string `json:"html"`
}

// PreviewTemplate handles POST /api/reports/preview-template: it runs the
// compiled query, renders the chosen template with the rows it returned, and
// hands back the document. The preview is the same rendering path the delivered
// report uses, so what the user approves is what they'll get.
func (h *Handlers) PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req previewTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	switch {
	case req.DataSourceID == "":
		writeError(w, http.StatusBadRequest, "dataSourceId is required")
		return
	case req.TemplateID == "":
		writeError(w, http.StatusBadRequest, "templateId is required")
		return
	}

	prepared, err := h.runner.prepare(r.Context(), userID, req.DataSourceID, req.QuerySpec)
	if err != nil {
		writeQueryError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()

	result, err := prepared.run(ctx, previewLimit(req.QuerySpec))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("query failed: %v", err))
		return
	}

	html, err := template.RenderWith(r.Context(), h.templates, userID, req.TemplateID, template.Data{
		Columns:     result.Columns,
		Rows:        result.Rows,
		GeneratedAt: time.Now(),
	}, req.TemplateConfig)
	if err != nil {
		writeTemplateError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, previewTemplateResponse{HTML: string(html)})
}

type previewAISummaryRequest struct {
	DataSourceID string     `json:"dataSourceId"`
	QuerySpec    query.Spec `json:"querySpec"`
	// Columns are the report query's own output columns to share with the
	// model, the same ones the block's "Context columns" slot maps.
	Columns []string `json:"columns"`
	Prompt  string   `json:"prompt"`
	// Queries are the block's additional queries, run only to gather context —
	// never shown in the document.
	Queries []template.AISummaryQuery `json:"queries"`
}

type previewAISummaryResponse struct {
	Text string `json:"text"`
}

// PreviewAISummary handles POST /api/reports/preview-ai-summary: the "Probar
// resumen" button. It's the only path that calls a model before a report is
// saved — everything else about an ai-summary block only prints a
// placeholder until the report actually runs — so a user can check a prompt
// works without waiting for, or paying for, an unattended run.
func (h *Handlers) PreviewAISummary(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if !h.aiSummaryEnabled || h.summaries == nil {
		writeError(w, http.StatusBadRequest, ErrAISummaryDisabled.Error())
		return
	}

	var req previewAISummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DataSourceID == "" {
		writeError(w, http.StatusBadRequest, "dataSourceId is required")
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	prepared, err := h.runner.prepare(r.Context(), userID, req.DataSourceID, req.QuerySpec)
	if err != nil {
		writeQueryError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()

	result, err := prepared.run(ctx, previewLimit(req.QuerySpec))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("query failed: %v", err))
		return
	}

	// A synthetic block and report carry just enough for the runner's own
	// context and generation logic to run unchanged — the same code path a
	// saved report uses, so what this returns is what the report will
	// actually print. "preview" is an arbitrary block ID: nothing here is
	// persisted, so it only has to be internally consistent.
	block := template.BlockDef{ID: "preview", Kind: template.BlockAISummary, Title: "Preview", Queries: req.Queries}
	rep := Report{
		UserID:       userID,
		DataSourceID: req.DataSourceID,
		TemplateConfig: template.Config{
			Columns: map[string][]string{"preview:columns": req.Columns},
			Text:    map[string]string{"preview:prompt": req.Prompt},
		},
	}

	text := h.runner.aiSummaryText(r.Context(), rep, template.Data{Columns: result.Columns, Rows: result.Rows}, block)
	writeJSON(w, http.StatusOK, previewAISummaryResponse{Text: text})
}

type createReportRequest struct {
	Name         string `json:"name"`
	DataSourceID string `json:"dataSourceId"`
	// Prompt is the user's own description of the report. It no longer builds the
	// query; it records intent, and is what a future summary of the report's
	// data would be written against.
	Prompt         string          `json:"prompt"`
	QuerySpec      query.Spec      `json:"querySpec"`
	TemplateID     string          `json:"templateId"`
	TemplateConfig template.Config `json:"templateConfig"`
	// Formats are the files the report should be delivered as. A request that
	// leaves them out gets a PDF, which is what a report was before it could be
	// anything else; a request that sends an empty list is asking for nothing
	// and is rejected.
	Formats []string `json:"formats"`
}

// chosenFormats reads the formats a create request asked for.
func chosenFormats(req createReportRequest) ([]render.Format, error) {
	if req.Formats == nil {
		return []render.Format{render.FormatPDF}, nil
	}
	return render.ParseFormats(req.Formats)
}

// Create handles POST /api/reports, storing the query specification alongside
// the rest of the report configuration.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req createReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if field := missingField(req); field != "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s is required", field))
		return
	}

	if err := validatePlaceholderFilters(req.QuerySpec.PlaceholderFilters); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// The mapping is checked against the template's slots, but not against the
	// query's columns: that would mean running the query to save a report.
	// Rendering re-checks it against the rows it actually gets.
	t, err := h.templates.Get(r.Context(), userID, req.TemplateID)
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	if err := template.Validate(t, req.TemplateConfig, nil); err != nil {
		writeTemplateError(w, err)
		return
	}
	if err := h.checkAISummaryBlocks(template.AISummaryBlocks(t)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.requireAIProviderIfNeeded(r.Context(), userID, t); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	formats, err := chosenFormats(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Compiling proves the specification is one this source can answer, and
	// resolving the source along the way proves it belongs to this user. The SQL
	// is stored for the user to read; it's recompiled from the spec on every run,
	// so a relative date window stays relative.
	prepared, err := h.runner.prepare(r.Context(), userID, req.DataSourceID, req.QuerySpec)
	if err != nil {
		writeQueryError(w, err)
		return
	}

	created, err := h.store.Create(r.Context(), Report{
		UserID:         userID,
		DataSourceID:   req.DataSourceID,
		Name:           req.Name,
		Prompt:         req.Prompt,
		Query:          prepared.compiled.Text,
		QuerySpec:      req.QuerySpec,
		TemplateID:     req.TemplateID,
		TemplateConfig: req.TemplateConfig,
		Formats:        formats,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save report")
		return
	}

	writeJSON(w, http.StatusCreated, toResponse(created))
}

// validatePlaceholderFilters checks the shape of a spec's placeholder filters.
// They're never compiled here — a recipient resolves them into literal Filters
// later — so this is the only check they get before being stored.
func validatePlaceholderFilters(filters []query.PlaceholderFilter) error {
	for _, f := range filters {
		switch {
		case f.Column == "":
			return fmt.Errorf("a placeholder filter's column is required")
		case f.RecipientField == "":
			return fmt.Errorf("a placeholder filter's recipientField is required")
		case !query.IsOperator(f.Operator):
			return fmt.Errorf("unknown placeholder filter operator: %s", f.Operator)
		}
	}
	return nil
}

// missingField names the first required field a request left empty. The query
// specification isn't checked here — compiling it against the schema reports far
// more usefully than "querySpec is required" could.
func missingField(req createReportRequest) string {
	switch {
	case req.Name == "":
		return "name"
	case req.DataSourceID == "":
		return "dataSourceId"
	case req.TemplateID == "":
		return "templateId"
	default:
		return ""
	}
}

// List handles GET /api/reports.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	reports, err := h.store.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list reports")
		return
	}

	resp := make([]reportResponse, 0, len(reports))
	for _, rep := range reports {
		resp = append(resp, toResponse(rep))
	}
	writeJSON(w, http.StatusOK, resp)
}

// Download handles GET /api/reports/{id}/download?format=pdf: it runs the
// report and returns one of its files.
//
// The file is produced in memory by the same pipeline that scheduled and
// on-demand delivery use, so what a user downloads here is the attachment their
// recipients get. Without a format it returns the first the report is
// configured for.
func (h *Handlers) Download(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	rep, err := h.store.Get(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "report not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load report")
		return
	}

	// A stored report always has at least one format — the column is checked in
	// the database and read back through ParseFormats — so the first is a
	// sensible default when the request doesn't name one.
	format := rep.Formats[0]
	if requested := r.URL.Query().Get("format"); requested != "" {
		if format, err = render.ParseFormat(requested); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	artifact, err := h.runner.RenderFormat(r.Context(), rep, format)
	if err != nil {
		writeRenderError(w, err)
		return
	}

	// The filename is a slug of the report's name, so it can't carry anything
	// that would need escaping here.
	w.Header().Set("Content-Type", artifact.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", artifact.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(artifact.Bytes)))
	if _, err := w.Write(artifact.Bytes); err != nil {
		log.Printf("report %s: writing the %s response failed: %v", rep.ID, format, err)
	}
}

// Delete handles DELETE /api/reports/{id}.
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if err := h.store.Delete(r.Context(), userID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "report not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete report")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
