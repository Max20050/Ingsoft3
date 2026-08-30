package llm

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Max20050/docuwave/internal/auth"
)

// Handlers exposes HTTP handlers for managing a user's LLM configuration.
type Handlers struct {
	store     *Store
	encryptor *Encryptor
}

func NewHandlers(store *Store, encryptor *Encryptor) *Handlers {
	return &Handlers{store: store, encryptor: encryptor}
}

type configRequest struct {
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
}

type configResponse struct {
	ID        string `json:"id"`
	Provider  string `json:"provider"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func toResponse(cfg Config) configResponse {
	return configResponse{
		ID:        cfg.ID,
		Provider:  cfg.Provider,
		CreatedAt: cfg.CreatedAt.Format(time.RFC3339),
		UpdatedAt: cfg.UpdatedAt.Format(time.RFC3339),
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

// Get handles GET /api/llm-config.
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	cfg, err := h.store.Get(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "no LLM configuration saved")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load LLM configuration")
		return
	}

	writeJSON(w, http.StatusOK, toResponse(cfg))
}

// Save handles PUT /api/llm-config: validates the provider, encrypts the API key, and persists it.
func (h *Handlers) Save(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req configRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}
	if req.APIKey == "" {
		writeError(w, http.StatusBadRequest, "apiKey is required")
		return
	}
	if _, err := NewProvider(req.Provider, req.APIKey); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	encryptedAPIKey, err := h.encryptor.Encrypt(req.APIKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to secure API key")
		return
	}

	saved, err := h.store.Upsert(r.Context(), userID, req.Provider, encryptedAPIKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save LLM configuration")
		return
	}

	writeJSON(w, http.StatusOK, toResponse(saved))
}

// Delete handles DELETE /api/llm-config.
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if err := h.store.Delete(r.Context(), userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "no LLM configuration saved")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete LLM configuration")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
