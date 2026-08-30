package datasource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Max20050/docuwave/internal/auth"
)

// RestHandlers exposes HTTP handlers for connecting a REST API as a data source.
type RestHandlers struct {
	store     *Store
	encryptor *Encryptor
	schemas   *SchemaProvider
}

func NewRestHandlers(store *Store, encryptor *Encryptor, schemas *SchemaProvider) *RestHandlers {
	return &RestHandlers{store: store, encryptor: encryptor, schemas: schemas}
}

// restAuthRequest is the wire shape of restAuthConfig: identical fields, kept
// as a separate type so a stray tag change on one doesn't silently change the
// other's JSON contract.
type restAuthRequest struct {
	Type        string `json:"type"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Token       string `json:"token,omitempty"`
	HeaderName  string `json:"headerName,omitempty"`
	HeaderValue string `json:"headerValue,omitempty"`
}

func (r restAuthRequest) toConfig() restAuthConfig {
	return restAuthConfig(r)
}

type restDataSourceRequest struct {
	Name    string          `json:"name"`
	URL     string          `json:"url"`
	Method  string          `json:"method"`
	Headers []RestHeader    `json:"headers"`
	Auth    restAuthRequest `json:"auth"`
	Body    string          `json:"body"`
}

func (r restDataSourceRequest) connector() *restConnector {
	return &restConnector{
		url:     r.URL,
		method:  r.Method,
		headers: r.Headers,
		auth:    r.Auth.toConfig(),
		body:    r.Body,
	}
}

// validate checks the fields every REST API source needs, independent of
// whether this is a test-only call or one that will be persisted.
func (r restDataSourceRequest) validate() error {
	if r.URL == "" {
		return errors.New("url is required")
	}
	auth := r.Auth.toConfig()
	if auth.Type == "" {
		auth.Type = "none"
	}
	return auth.validate()
}

// TestConnection handles POST /api/datasources/rest-api/test, verifying
// reachability without persisting anything.
func (h *RestHandlers) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req restDataSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), testConnectionTimeout)
	defer cancel()

	if err := req.connector().TestConnection(ctx); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("could not reach data source: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Create handles POST /api/datasources/rest-api: verifies reachability,
// encrypts the auth config, and persists the source.
func (h *RestHandlers) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req restDataSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), testConnectionTimeout)
	defer cancel()

	if err := req.connector().TestConnection(ctx); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("could not reach data source: %v", err))
		return
	}

	authType := req.Auth.Type
	if authType == "" {
		authType = "none"
	}
	authJSON, err := json.Marshal(req.Auth.toConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to secure credentials")
		return
	}
	encryptedAuthConfig, err := h.encryptor.Encrypt(string(authJSON))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to secure credentials")
		return
	}

	var headersJSON *string
	if len(req.Headers) > 0 {
		encoded, err := json.Marshal(req.Headers)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save data source")
			return
		}
		headersJSON = ptr(string(encoded))
	}

	var bodyField *string
	if req.Body != "" {
		bodyField = ptr(req.Body)
	}

	created, err := h.store.CreateRestSource(r.Context(), DataSource{
		UserID:       userID,
		Name:         req.Name,
		Type:         restSourceType,
		RestURL:      ptr(req.URL),
		RestMethod:   ptr(req.Method),
		RestHeaders:  headersJSON,
		RestAuthType: ptr(authType),
		RestBody:     bodyField,
	}, encryptedAuthConfig)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save data source")
		return
	}

	storeSchema(r.Context(), h.schemas, userID, created.ID)

	writeJSON(w, http.StatusCreated, toResponse(created))
}
