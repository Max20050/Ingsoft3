package datasource

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleSheetsConfig holds the OAuth2 client configuration used to connect a
// user's Google account for reading their Sheets, scoped to read-only access.
type GoogleSheetsConfig struct {
	oauth *oauth2.Config
}

// NewGoogleSheetsConfig builds the OAuth2 config from client credentials.
func NewGoogleSheetsConfig(clientID, clientSecret, redirectURL string) *GoogleSheetsConfig {
	return &GoogleSheetsConfig{
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/spreadsheets.readonly",
				"https://www.googleapis.com/auth/drive.metadata.readonly",
			},
			Endpoint: google.Endpoint,
		},
	}
}
