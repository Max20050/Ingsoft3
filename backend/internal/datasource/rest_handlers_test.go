package datasource

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRestDataSourceRequestValidate(t *testing.T) {
	tests := []struct {
		name    string
		req     restDataSourceRequest
		wantErr bool
	}{
		{"missing url", restDataSourceRequest{}, true},
		{"url with default (none) auth", restDataSourceRequest{URL: "https://example.com"}, false},
		{
			"invalid auth type",
			restDataSourceRequest{URL: "https://example.com", Auth: restAuthRequest{Type: "hmac"}},
			true,
		},
		{
			"bearer without token",
			restDataSourceRequest{URL: "https://example.com", Auth: restAuthRequest{Type: "bearer"}},
			true,
		},
		{
			"bearer with token",
			restDataSourceRequest{URL: "https://example.com", Auth: restAuthRequest{Type: "bearer", Token: "t"}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRestHandlersTestConnectionRequiresValidBody(t *testing.T) {
	h := NewRestHandlers(nil, nil, nil)

	recorder := httptest.NewRecorder()
	body, _ := json.Marshal(restDataSourceRequest{})
	req := httptest.NewRequest(http.MethodPost, "/api/datasources/rest-api/test", bytes.NewReader(body))
	h.TestConnection(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestRestHandlersTestConnectionSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	h := NewRestHandlers(nil, nil, nil)
	recorder := httptest.NewRecorder()
	body, _ := json.Marshal(restDataSourceRequest{URL: server.URL})
	req := httptest.NewRequest(http.MethodPost, "/api/datasources/rest-api/test", bytes.NewReader(body))
	h.TestConnection(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("got status %d, want %d, body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestRestHandlersCreateRequiresAuthentication(t *testing.T) {
	h := NewRestHandlers(nil, nil, nil)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/datasources/rest-api", nil)
	h.Create(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
