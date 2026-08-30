package datasource

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Max20050/docuwave/internal/auth"
)

// FieldMappingHandlers exposes HTTP handlers for mapping a data source's
// detected fields onto DocuWave's predefined system fields.
type FieldMappingHandlers struct {
	mappings *FieldMappingStore
	schemas  *SchemaProvider
}

func NewFieldMappingHandlers(mappings *FieldMappingStore, schemas *SchemaProvider) *FieldMappingHandlers {
	return &FieldMappingHandlers{mappings: mappings, schemas: schemas}
}

type fieldMappingResponse struct {
	DataSourceID string            `json:"dataSourceId"`
	Mapping      map[string]string `json:"mapping"`
	SystemFields []SystemField     `json:"systemFields"`
	UpdatedAt    string            `json:"updatedAt,omitempty"`
}

type fieldMappingRequest struct {
	Mapping map[string]string `json:"mapping"`
}

// Get handles GET /api/datasources/{id}/field-mapping, returning the stored
// mapping (empty if none has been saved yet) alongside the fixed catalog of
// system fields it can point to.
func (h *FieldMappingHandlers) Get(w http.ResponseWriter, r *http.Request) {
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

	// Confirms the source is owned by the caller before revealing anything
	// about its stored mapping.
	if _, _, err := h.schemas.Schema(r.Context(), userID, id); err != nil {
		writeResolveError(w, err)
		return
	}

	mapping, updatedAt, err := h.mappings.Get(r.Context(), id)
	if err != nil {
		if !errors.Is(err, ErrFieldMappingNotFound) {
			writeError(w, http.StatusInternalServerError, "failed to load field mapping")
			return
		}
		mapping = map[string]string{}
	}

	resp := fieldMappingResponse{
		DataSourceID: id,
		Mapping:      mapping,
		SystemFields: SystemFields,
	}
	if !updatedAt.IsZero() {
		resp.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

// Put handles PUT /api/datasources/{id}/field-mapping, validating that every
// mapped api_field exists in the source's current schema and every our_field
// is one of the predefined system fields before saving.
func (h *FieldMappingHandlers) Put(w http.ResponseWriter, r *http.Request) {
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

	var req fieldMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	schema, _, err := h.schemas.Schema(r.Context(), userID, id)
	if err != nil {
		writeResolveError(w, err)
		return
	}

	if err := validateFieldMapping(req.Mapping, schema.Fields); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.mappings.Save(r.Context(), id, req.Mapping); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save field mapping")
		return
	}

	writeJSON(w, http.StatusOK, fieldMappingResponse{
		DataSourceID: id,
		Mapping:      req.Mapping,
		SystemFields: SystemFields,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
	})
}

// validateFieldMapping checks every api_field in mapping is a field the
// source's schema actually has, and every our_field is a predefined system
// field.
func validateFieldMapping(mapping map[string]string, knownFields []string) error {
	known := make(map[string]bool, len(knownFields))
	for _, f := range knownFields {
		known[f] = true
	}

	for apiField, ourField := range mapping {
		if apiField == "" {
			return errors.New("api_field cannot be empty")
		}
		if !known[apiField] {
			return fmt.Errorf("unknown api field: %s", apiField)
		}
		if !isValidSystemField(ourField) {
			return fmt.Errorf("unknown system field: %s", ourField)
		}
	}
	return nil
}
