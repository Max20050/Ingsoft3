package datasource

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

// Every source type must be able to report its own schema and run queries.
var (
	_ Connector = (*postgresConnector)(nil)
	_ Connector = (*mysqlConnector)(nil)
	_ Connector = (*googleSheetsConnector)(nil)
	_ Connector = (*restConnector)(nil)
)

func TestGroupColumnsGroupsRowsInOrder(t *testing.T) {
	rows := []columnRow{
		{Schema: "public", Table: "orders", Column: "id", Type: "uuid"},
		{Schema: "public", Table: "orders", Column: "total", Type: "numeric"},
		{Schema: "public", Table: "customers", Column: "email", Type: "text"},
	}

	got := groupColumns(rows)
	want := []Table{
		{Name: "orders", Columns: []Column{{Name: "id", Type: "uuid"}, {Name: "total", Type: "numeric"}}},
		{Name: "customers", Columns: []Column{{Name: "email", Type: "text"}}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestGroupColumnsQualifiesNonPublicSchemas(t *testing.T) {
	rows := []columnRow{
		{Schema: "public", Table: "orders", Column: "id", Type: "uuid"},
		{Schema: "sales", Table: "orders", Column: "id", Type: "uuid"},
		{Schema: "", Table: "orders", Column: "id", Type: "int(11)"}, // MySQL sends no schema
	}

	got := groupColumns(rows)
	want := []string{"orders", "sales.orders"}
	if len(got) != len(want) {
		t.Fatalf("got %d tables, want %d: %+v", len(got), len(want), got)
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("table %d: got name %q, want %q", i, got[i].Name, name)
		}
	}
	// The schema-less row belongs to the same table as the "public" one.
	if len(got[0].Columns) != 2 {
		t.Errorf("got %d columns for %q, want 2", len(got[0].Columns), got[0].Name)
	}
}

func TestGroupColumnsWithNoRows(t *testing.T) {
	got := groupColumns(nil)
	if got == nil {
		t.Fatal("got nil, want an empty slice so it serializes as []")
	}
	if len(got) != 0 {
		t.Errorf("got %d tables, want 0", len(got))
	}
}

func TestHeaderFields(t *testing.T) {
	tests := []struct {
		name   string
		values [][]string
		want   []string
	}{
		{
			name:   "trims whitespace and trailing blanks",
			values: [][]string{{"Region", " Sales ", "", ""}},
			want:   []string{"Region", "Sales"},
		},
		{
			name:   "keeps interior blanks so positions still line up",
			values: [][]string{{"Region", "", "Sales"}},
			want:   []string{"Region", "", "Sales"},
		},
		{
			name:   "ignores rows below the header",
			values: [][]string{{"Region"}, {"North"}},
			want:   []string{"Region"},
		},
		{
			name:   "empty sheet",
			values: nil,
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := headerFields(tt.values)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

// The report builder builds its table and column pickers from this response, and
// shows fetchedAt so the user knows how current the picture is.
func TestSchemaResponseJSONShape(t *testing.T) {
	fetched := time.Date(2026, 8, 12, 9, 30, 0, 0, time.UTC)

	sqlBody, err := json.Marshal(schemaResponse{
		DataSourceID: "ds-1",
		Type:         "postgres",
		FetchedAt:    fetched.Format(time.RFC3339),
		Schema:       Schema{Tables: []Table{{Name: "orders", Columns: []Column{{Name: "id", Type: "uuid"}}}}},
	})
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	wantSQL := `{"dataSourceId":"ds-1","type":"postgres","fetchedAt":"2026-08-12T09:30:00Z",` +
		`"tables":[{"name":"orders","columns":[{"name":"id","type":"uuid"}]}]}`
	if string(sqlBody) != wantSQL {
		t.Errorf("got %s, want %s", sqlBody, wantSQL)
	}

	sheetsBody, err := json.Marshal(schemaResponse{
		DataSourceID: "ds-2",
		Type:         "google_sheets",
		FetchedAt:    fetched.Format(time.RFC3339),
		Schema:       Schema{Fields: []string{"Region", "Sales"}},
	})
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}
	wantSheets := `{"dataSourceId":"ds-2","type":"google_sheets","fetchedAt":"2026-08-12T09:30:00Z","fields":["Region","Sales"]}`
	if string(sheetsBody) != wantSheets {
		t.Errorf("got %s, want %s", sheetsBody, wantSheets)
	}
}

func TestSchemaHandlerRequiresAuthentication(t *testing.T) {
	handlers := NewSchemaHandlers(nil, nil)

	for name, handler := range map[string]http.HandlerFunc{"get": handlers.Get, "refresh": handlers.Refresh} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler(recorder, httptest.NewRequest(http.MethodGet, "/api/datasources/ds-1/schema", nil))

			if recorder.Code != http.StatusUnauthorized {
				t.Errorf("got status %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}
