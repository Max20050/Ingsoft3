package datasource

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateFieldMapping(t *testing.T) {
	knownFields := []string{"id", "total", "email"}

	tests := []struct {
		name    string
		mapping map[string]string
		wantErr bool
	}{
		{"empty mapping", map[string]string{}, false},
		{"valid mapping", map[string]string{"total": "total_amount", "email": "customer_email"}, false},
		{"unknown api field", map[string]string{"missing": "total_amount"}, true},
		{"unknown system field", map[string]string{"total": "not_a_real_field"}, true},
		{"empty api field", map[string]string{"": "total_amount"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFieldMapping(tt.mapping, knownFields)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFieldMappingHandlersGetRequiresAuthentication(t *testing.T) {
	h := NewFieldMappingHandlers(nil, nil)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/datasources/abc/field-mapping", nil)
	h.Get(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestFieldMappingHandlersPutRequiresAuthentication(t *testing.T) {
	h := NewFieldMappingHandlers(nil, nil)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/datasources/abc/field-mapping", nil)
	h.Put(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestIsValidSystemField(t *testing.T) {
	if !isValidSystemField("total_amount") {
		t.Error("expected total_amount to be a valid system field")
	}
	if isValidSystemField("not_a_real_field") {
		t.Error("expected not_a_real_field to be invalid")
	}
}
