package datasource

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/Max20050/docuwave/internal/auth"
)

const (
	sheetsOAuthStateCookie = "sheets_oauth_state"
	sheetsOAuthUserCookie  = "sheets_oauth_user"
)

// SheetsHandlers exposes HTTP handlers for connecting a Google account and
// choosing a spreadsheet to use as a data source.
type SheetsHandlers struct {
	store       *Store
	connections *SheetsStore
	encryptor   *Encryptor
	oauthConfig *GoogleSheetsConfig
	tokens      *auth.TokenIssuer
	schemas     *SchemaProvider
	frontendURL string
}

func NewSheetsHandlers(store *Store, connections *SheetsStore, encryptor *Encryptor, oauthConfig *GoogleSheetsConfig, tokens *auth.TokenIssuer, schemas *SchemaProvider, frontendURL string) *SheetsHandlers {
	return &SheetsHandlers{
		store:       store,
		connections: connections,
		encryptor:   encryptor,
		oauthConfig: oauthConfig,
		tokens:      tokens,
		schemas:     schemas,
		frontendURL: frontendURL,
	}
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Login handles GET /api/datasources/google-sheets/login?token=<jwt>, redirecting
// to Google's consent screen to authorize read-only access to the user's Sheets.
// The session token travels as a query parameter because this endpoint is reached
// via a plain browser navigation, which can't carry an Authorization header.
func (h *SheetsHandlers) Login(w http.ResponseWriter, r *http.Request) {
	userID, err := h.tokens.VerifyToken(r.URL.Query().Get("token"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	state, err := randomState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start google sheets connection")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sheetsOAuthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   int((10 * time.Minute).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     sheetsOAuthUserCookie,
		Value:    userID,
		Path:     "/",
		MaxAge:   int((10 * time.Minute).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	authURL := h.oauthConfig.oauth.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles GET /api/datasources/google-sheets/callback, exchanging the
// code, storing the encrypted OAuth token, and sending the user back to the
// data sources screen to pick a spreadsheet.
func (h *SheetsHandlers) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(sheetsOAuthStateCookie)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sheetsOAuthStateCookie, Value: "", Path: "/", MaxAge: -1})

	userCookie, err := r.Cookie(sheetsOAuthUserCookie)
	if err != nil || userCookie.Value == "" {
		writeError(w, http.StatusBadRequest, "missing google sheets session")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sheetsOAuthUserCookie, Value: "", Path: "/", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	token, err := h.oauthConfig.oauth.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to exchange authorization code")
		return
	}
	if token.RefreshToken == "" {
		writeError(w, http.StatusBadGateway, "google did not grant offline access; please reconnect and accept all prompts")
		return
	}

	tokenJSON, err := json.Marshal(token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store google credentials")
		return
	}
	encryptedToken, err := h.encryptor.Encrypt(string(tokenJSON))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to secure google credentials")
		return
	}

	conn, err := h.connections.SaveConnection(r.Context(), userCookie.Value, encryptedToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save google sheets connection")
		return
	}

	redirectURL := h.frontendURL + "/datasources?sheetsConnection=" + url.QueryEscape(conn.ID)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

type spreadsheetSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type driveFilesResponse struct {
	Files []spreadsheetSummary `json:"files"`
}

// ListSpreadsheets handles GET /api/datasources/google-sheets/connections/{id}/spreadsheets,
// listing the spreadsheets visible to the connected Google account.
func (h *SheetsHandlers) ListSpreadsheets(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	client, err := h.clientForConnection(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeConnectionError(w, err)
		return
	}

	const driveFilesURL = "https://www.googleapis.com/drive/v3/files?" +
		"q=mimeType%3D%27application%2Fvnd.google-apps.spreadsheet%27+and+trashed%3Dfalse" +
		"&fields=files(id,name)&pageSize=100"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, driveFilesURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list spreadsheets")
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to reach google drive")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("google drive returned an error: %s", string(body)))
		return
	}

	var parsed driveFilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		writeError(w, http.StatusBadGateway, "failed to parse google drive response")
		return
	}

	writeJSON(w, http.StatusOK, parsed.Files)
}

type sheetsDataSourceRequest struct {
	Name            string `json:"name"`
	ConnectionID    string `json:"connectionId"`
	SpreadsheetID   string `json:"spreadsheetId"`
	SpreadsheetName string `json:"spreadsheetName"`
}

// Create handles POST /api/datasources/google-sheets: verifies the spreadsheet
// is reachable with the connected account's token, then persists it as a data source.
func (h *SheetsHandlers) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req sheetsDataSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.ConnectionID == "" || req.SpreadsheetID == "" {
		writeError(w, http.StatusBadRequest, "name, connectionId, and spreadsheetId are required")
		return
	}

	token, err := h.tokenForConnection(r.Context(), userID, req.ConnectionID)
	if err != nil {
		writeConnectionError(w, err)
		return
	}

	connector := &googleSheetsConnector{
		oauthConfig:   h.oauthConfig.oauth,
		token:         token,
		spreadsheetID: req.SpreadsheetID,
	}
	ctx, cancel := context.WithTimeout(r.Context(), testConnectionTimeout)
	defer cancel()
	if err := connector.TestConnection(ctx); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("could not reach spreadsheet: %v", err))
		return
	}

	created, err := h.store.CreateSheetsSource(r.Context(), DataSource{
		UserID:             userID,
		Name:               req.Name,
		Type:               sheetsSourceType,
		SpreadsheetID:      ptr(req.SpreadsheetID),
		SpreadsheetName:    ptr(req.SpreadsheetName),
		GoogleConnectionID: ptr(req.ConnectionID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save data source")
		return
	}

	storeSchema(r.Context(), h.schemas, userID, created.ID)

	writeJSON(w, http.StatusCreated, toResponse(created))
}

// sheetsToken loads and decrypts the OAuth token behind a stored Google
// Sheets connection owned by the given user.
func sheetsToken(ctx context.Context, connections *SheetsStore, encryptor *Encryptor, userID, connectionID string) (*oauth2.Token, error) {
	encryptedToken, err := connections.GetEncryptedToken(ctx, userID, connectionID)
	if err != nil {
		return nil, err
	}
	tokenJSON, err := encryptor.Decrypt(encryptedToken)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal([]byte(tokenJSON), &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (h *SheetsHandlers) tokenForConnection(ctx context.Context, userID, connectionID string) (*oauth2.Token, error) {
	return sheetsToken(ctx, h.connections, h.encryptor, userID, connectionID)
}

func (h *SheetsHandlers) clientForConnection(ctx context.Context, userID, connectionID string) (*http.Client, error) {
	token, err := h.tokenForConnection(ctx, userID, connectionID)
	if err != nil {
		return nil, err
	}
	return h.oauthConfig.oauth.Client(ctx, token), nil
}

func writeConnectionError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrConnectionNotFound) {
		writeError(w, http.StatusNotFound, "google sheets connection not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "failed to load google sheets connection")
}
