package datasource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Max20050/docuwave/internal/auth"
)

const testConnectionTimeout = 5 * time.Second

// Handlers exposes HTTP handlers for managing data sources.
type Handlers struct {
	store     *Store
	encryptor *Encryptor
	schemas   *SchemaProvider
}

func NewHandlers(store *Store, encryptor *Encryptor, schemas *SchemaProvider) *Handlers {
	return &Handlers{store: store, encryptor: encryptor, schemas: schemas}
}

// storeSchema reads a newly connected source's structure and keeps it, because
// that's what the report builder builds queries out of. A failure doesn't undo
// the connection: the source is saved either way, and its schema can be read
// again on demand.
func storeSchema(ctx context.Context, schemas *SchemaProvider, userID, dataSourceID string) {
	if _, _, err := schemas.Refresh(ctx, userID, dataSourceID); err != nil {
		log.Printf("data source %s connected, but reading its schema failed: %v", dataSourceID, err)
	}
}

type dataSourceRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DBName   string `json:"dbName"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func ptr[T any](v T) *T { return &v }

func (r dataSourceRequest) connectionConfig() ConnectionConfig {
	return ConnectionConfig{
		Host:     r.Host,
		Port:     r.Port,
		DBName:   r.DBName,
		Username: r.Username,
		Password: r.Password,
	}
}

func (r dataSourceRequest) missingField() string {
	switch {
	case r.Type == "":
		return "type"
	case r.Host == "":
		return "host"
	case r.Port <= 0:
		return "port"
	case r.DBName == "":
		return "dbName"
	case r.Username == "":
		return "username"
	default:
		return ""
	}
}

type dataSourceResponse struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Type            string       `json:"type"`
	Host            *string      `json:"host,omitempty"`
	Port            *int         `json:"port,omitempty"`
	DBName          *string      `json:"dbName,omitempty"`
	Username        *string      `json:"username,omitempty"`
	SpreadsheetID   *string      `json:"spreadsheetId,omitempty"`
	SpreadsheetName *string      `json:"spreadsheetName,omitempty"`
	URL             *string      `json:"url,omitempty"`
	Method          *string      `json:"method,omitempty"`
	Headers         []RestHeader `json:"headers,omitempty"`
	AuthType        *string      `json:"authType,omitempty"`
	CreatedAt       string       `json:"createdAt"`
}

func toResponse(ds DataSource) dataSourceResponse {
	// Secrets (the SQL password, the REST auth config) never leave the
	// package: encrypted_password and rest_encrypted_auth_config aren't
	// scanned into DataSource in the first place.
	headers, _ := decodeRestHeaders(ds.RestHeaders)
	return dataSourceResponse{
		ID:              ds.ID,
		Name:            ds.Name,
		Type:            ds.Type,
		Host:            ds.Host,
		Port:            ds.Port,
		DBName:          ds.DBName,
		Username:        ds.Username,
		SpreadsheetID:   ds.SpreadsheetID,
		SpreadsheetName: ds.SpreadsheetName,
		URL:             ds.RestURL,
		Method:          ds.RestMethod,
		Headers:         headers,
		AuthType:        ds.RestAuthType,
		CreatedAt:       ds.CreatedAt.Format(time.RFC3339),
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// TestConnection handles POST /api/datasources/test, verifying reachability without persisting anything.
func (h *Handlers) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req dataSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if field := req.missingField(); field != "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s is required", field))
		return
	}

	connector, err := NewConnector(req.Type, req.connectionConfig())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), testConnectionTimeout)
	defer cancel()

	if err := connector.TestConnection(ctx); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("could not reach data source: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Create handles POST /api/datasources: verifies reachability, encrypts the password, and persists the source.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req dataSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if field := req.missingField(); field != "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s is required", field))
		return
	}

	connector, err := NewConnector(req.Type, req.connectionConfig())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), testConnectionTimeout)
	defer cancel()

	if err := connector.TestConnection(ctx); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("could not reach data source: %v", err))
		return
	}

	encryptedPassword, err := h.encryptor.Encrypt(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to secure credentials")
		return
	}

	created, err := h.store.Create(r.Context(), DataSource{
		UserID:   userID,
		Name:     req.Name,
		Type:     req.Type,
		Host:     ptr(req.Host),
		Port:     ptr(req.Port),
		DBName:   ptr(req.DBName),
		Username: ptr(req.Username),
	}, encryptedPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save data source")
		return
	}

	storeSchema(r.Context(), h.schemas, userID, created.ID)

	writeJSON(w, http.StatusCreated, toResponse(created))
}

// List handles GET /api/datasources.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	sources, err := h.store.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list data sources")
		return
	}

	resp := make([]dataSourceResponse, 0, len(sources))
	for _, ds := range sources {
		resp = append(resp, toResponse(ds))
	}
	writeJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /api/datasources/{id}.
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
			writeError(w, http.StatusNotFound, "data source not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete data source")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
