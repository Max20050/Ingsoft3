package recipient

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Max20050/docuwave/internal/auth"
)

// Handlers exposes HTTP handlers for managing recipients and the groups they're organized into.
type Handlers struct {
	store  *Store
	groups *GroupStore
}

func NewHandlers(store *Store, groups *GroupStore) *Handlers {
	return &Handlers{store: store, groups: groups}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type recipientResponse struct {
	ID         string         `json:"id"`
	Email      string         `json:"email"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes"`
	CreatedAt  string         `json:"createdAt"`
}

func toResponse(rec Recipient) recipientResponse {
	attributes := rec.Attributes
	if attributes == nil {
		attributes = map[string]any{}
	}
	return recipientResponse{
		ID:         rec.ID,
		Email:      rec.Email,
		Name:       rec.Name,
		Attributes: attributes,
		CreatedAt:  rec.CreatedAt.Format(time.RFC3339),
	}
}

type createRecipientRequest struct {
	Email      string         `json:"email"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes"`
}

// Create handles POST /api/recipients.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req createRecipientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	created, err := h.store.Create(r.Context(), Recipient{
		UserID:     userID,
		Email:      req.Email,
		Name:       req.Name,
		Attributes: req.Attributes,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save recipient")
		return
	}

	writeJSON(w, http.StatusCreated, toResponse(created))
}

// List handles GET /api/recipients.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	recipients, err := h.store.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list recipients")
		return
	}

	resp := make([]recipientResponse, 0, len(recipients))
	for _, rec := range recipients {
		resp = append(resp, toResponse(rec))
	}
	writeJSON(w, http.StatusOK, resp)
}

// Delete handles DELETE /api/recipients/{id}.
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
			writeError(w, http.StatusNotFound, "recipient not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete recipient")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type groupResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

func toGroupResponse(g Group) groupResponse {
	return groupResponse{ID: g.ID, Name: g.Name, CreatedAt: g.CreatedAt.Format(time.RFC3339)}
}

type createGroupRequest struct {
	Name string `json:"name"`
}

// CreateGroup handles POST /api/recipient-groups.
func (h *Handlers) CreateGroup(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	created, err := h.groups.Create(r.Context(), userID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save recipient group")
		return
	}

	writeJSON(w, http.StatusCreated, toGroupResponse(created))
}

// ListGroups handles GET /api/recipient-groups.
func (h *Handlers) ListGroups(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	groups, err := h.groups.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list recipient groups")
		return
	}

	resp := make([]groupResponse, 0, len(groups))
	for _, g := range groups {
		resp = append(resp, toGroupResponse(g))
	}
	writeJSON(w, http.StatusOK, resp)
}

// DeleteGroup handles DELETE /api/recipient-groups/{id}.
func (h *Handlers) DeleteGroup(w http.ResponseWriter, r *http.Request) {
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

	if err := h.groups.Delete(r.Context(), userID, id); err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			writeError(w, http.StatusNotFound, "recipient group not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete recipient group")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeMembershipError maps a group/recipient lookup failure onto an HTTP response.
func writeMembershipError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrGroupNotFound):
		writeError(w, http.StatusNotFound, "recipient group not found")
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "recipient not found")
	default:
		writeError(w, http.StatusInternalServerError, "failed to update group membership")
	}
}

// ListMembers handles GET /api/recipient-groups/{id}/members.
func (h *Handlers) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	groupID := r.PathValue("id")
	members, err := h.groups.Members(r.Context(), userID, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}

	resp := make([]recipientResponse, 0, len(members))
	for _, rec := range members {
		resp = append(resp, toResponse(rec))
	}
	writeJSON(w, http.StatusOK, resp)
}

type addMemberRequest struct {
	RecipientID string `json:"recipientId"`
}

// AddMember handles POST /api/recipient-groups/{id}/members.
func (h *Handlers) AddMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	groupID := r.PathValue("id")

	var req addMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RecipientID == "" {
		writeError(w, http.StatusBadRequest, "recipientId is required")
		return
	}

	if err := h.groups.AddMember(r.Context(), userID, groupID, req.RecipientID); err != nil {
		writeMembershipError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveMember handles DELETE /api/recipient-groups/{id}/members/{recipientId}.
func (h *Handlers) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	groupID := r.PathValue("id")
	recipientID := r.PathValue("recipientId")

	if err := h.groups.RemoveMember(r.Context(), userID, groupID, recipientID); err != nil {
		writeMembershipError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
