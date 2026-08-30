package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const oauthStateCookie = "oauth_state"

// GoogleConfig holds the OAuth2 client configuration for "Sign in with Google".
type GoogleConfig struct {
	oauth *oauth2.Config
}

// NewGoogleConfig builds the OAuth2 config from client credentials.
func NewGoogleConfig(clientID, clientSecret, redirectURL string) *GoogleConfig {
	return &GoogleConfig{
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
			Endpoint:     google.Endpoint,
		},
	}
}

type googleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
}

// GoogleHandlers exposes HTTP handlers for the "Sign in with Google" flow.
type GoogleHandlers struct {
	store       *Store
	tokens      *TokenIssuer
	google      *GoogleConfig
	frontendURL string
}

func NewGoogleHandlers(store *Store, tokens *TokenIssuer, google *GoogleConfig, frontendURL string) *GoogleHandlers {
	return &GoogleHandlers{store: store, tokens: tokens, google: google, frontendURL: frontendURL}
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Login handles GET /api/auth/google/login, redirecting to Google's consent screen.
func (h *GoogleHandlers) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start google sign-in")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   int((10 * time.Minute).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.google.oauth.AuthCodeURL(state), http.StatusFound)
}

// Callback handles GET /api/auth/google/callback, exchanging the auth code and
// issuing a session JWT identical in shape to the email/password flow.
func (h *GoogleHandlers) Callback(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	token, err := h.google.oauth.Exchange(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to exchange authorization code")
		return
	}

	info, err := h.fetchUserInfo(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch google profile")
		return
	}
	if !info.VerifiedEmail || info.Email == "" {
		writeError(w, http.StatusForbidden, "google account email is not verified")
		return
	}

	userID, err := h.findOrCreateUser(r.Context(), info)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign in with google")
		return
	}

	sessionToken, err := h.tokens.IssueToken(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	redirectURL := h.frontendURL + "/auth/callback?token=" + url.QueryEscape(sessionToken)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *GoogleHandlers) fetchUserInfo(ctx context.Context, token *oauth2.Token) (*googleUserInfo, error) {
	client := h.google.oauth.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unexpected status from google userinfo endpoint")
	}
	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// findOrCreateUser resolves a Google profile to a local user, linking the
// Google account to an existing email/password user if one already exists.
func (h *GoogleHandlers) findOrCreateUser(ctx context.Context, info *googleUserInfo) (string, error) {
	existing, err := h.store.GetUserByGoogleID(ctx, info.ID)
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return "", err
	}

	email := normalizeEmail(info.Email)
	existing, err = h.store.GetUserByEmail(ctx, email)
	if err == nil {
		if err := h.store.LinkGoogleID(ctx, existing.ID, info.ID); err != nil {
			return "", err
		}
		return existing.ID, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return "", err
	}

	return h.store.CreateGoogleUser(ctx, email, info.ID)
}
