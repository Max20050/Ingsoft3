package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
)

// Handlers exposes HTTP handlers for registration, login, and the current user.
type Handlers struct {
	store  *Store
	tokens *TokenIssuer
}

func NewHandlers(store *Store, tokens *TokenIssuer) *Handlers {
	return &Handlers{store: store, tokens: tokens}
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validCredentials(email, password string) bool {
	if _, err := mail.ParseAddress(email); err != nil {
		return false
	}
	return len(password) >= 8
}

// Register handles POST /api/auth/register.
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	email := normalizeEmail(creds.Email)
	if !validCredentials(email, creds.Password) {
		writeError(w, http.StatusBadRequest, "valid email and a password of at least 8 characters are required")
		return
	}

	hash, err := HashPassword(creds.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process password")
		return
	}

	userID, err := h.store.CreateUser(r.Context(), email, hash)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	token, err := h.tokens.IssueToken(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{Token: token})
}

// Login handles POST /api/auth/login.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var creds credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	email := normalizeEmail(creds.Email)

	user, err := h.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to look up user")
		return
	}

	if !CheckPassword(user.PasswordHash, creds.Password) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := h.tokens.IssueToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token})
}

// Me handles GET /api/me, returning the authenticated user's public profile.
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	user, err := h.store.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to look up user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": user.ID, "email": user.Email})
}
