package datasource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
)

func TestRestConnectorTestConnectionSucceedsOn2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &restConnector{url: server.URL}
	if err := c.TestConnection(context.Background()); err != nil {
		t.Errorf("got error %v, want nil", err)
	}
}

func TestRestConnectorTestConnectionFailsOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := &restConnector{url: server.URL}
	if err := c.TestConnection(context.Background()); err == nil {
		t.Error("got nil error, want a failure for a 401 response")
	}
}

func TestRestConnectorTestConnectionFailsOnUnreachableHost(t *testing.T) {
	c := &restConnector{url: "http://127.0.0.1:0"}
	if err := c.TestConnection(context.Background()); err == nil {
		t.Error("got nil error, want a failure for an unreachable host")
	}
}

func TestRestConnectorBuildRequestAppliesHeadersAndAuth(t *testing.T) {
	var gotMethod string
	var gotHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &restConnector{
		url:     server.URL,
		method:  http.MethodPost,
		headers: []RestHeader{{Key: "X-Custom", Value: "abc"}},
		auth:    restAuthConfig{Type: "bearer", Token: "secret-token"},
	}
	if err := c.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("got method %q, want POST", gotMethod)
	}
	if got := gotHeaders.Get("X-Custom"); got != "abc" {
		t.Errorf("got X-Custom %q, want abc", got)
	}
	if got := gotHeaders.Get("Authorization"); got != "Bearer secret-token" {
		t.Errorf("got Authorization %q, want 'Bearer secret-token'", got)
	}
}

func TestRestAuthConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		auth    restAuthConfig
		wantErr bool
	}{
		{"none is always valid", restAuthConfig{Type: "none"}, false},
		{"unknown type is rejected", restAuthConfig{Type: "hmac"}, true},
		{"basic requires username and password", restAuthConfig{Type: "basic"}, true},
		{"basic with both fields is valid", restAuthConfig{Type: "basic", Username: "u", Password: "p"}, false},
		{"bearer requires a token", restAuthConfig{Type: "bearer"}, true},
		{"bearer with token is valid", restAuthConfig{Type: "bearer", Token: "t"}, false},
		{"api_key requires header name and value", restAuthConfig{Type: "api_key"}, true},
		{"api_key with both fields is valid", restAuthConfig{Type: "api_key", HeaderName: "X-Key", HeaderValue: "v"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.auth.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRestConnectorIntrospectFlatObjectAtRoot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 1, "name": "north"}`))
	}))
	defer server.Close()

	got, err := (&restConnector{url: server.URL}).Introspect(context.Background())
	if err != nil {
		t.Fatalf("Introspect returned error: %v", err)
	}
	sort.Strings(got.Fields)
	want := []string{"id", "name"}
	if !reflect.DeepEqual(got.Fields, want) {
		t.Errorf("got %#v, want %#v", got.Fields, want)
	}
}

func TestRestConnectorIntrospectArrayAtRoot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id": 1, "region": "north"}, {"id": 2, "region": "south"}]`))
	}))
	defer server.Close()

	got, err := (&restConnector{url: server.URL}).Introspect(context.Background())
	if err != nil {
		t.Fatalf("Introspect returned error: %v", err)
	}
	sort.Strings(got.Fields)
	want := []string{"id", "region"}
	if !reflect.DeepEqual(got.Fields, want) {
		t.Errorf("got %#v, want %#v", got.Fields, want)
	}
}

func TestRestConnectorIntrospectArrayNestedUnderKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [{"id": 1, "region": "north"}]}`))
	}))
	defer server.Close()

	got, err := (&restConnector{url: server.URL}).Introspect(context.Background())
	if err != nil {
		t.Fatalf("Introspect returned error: %v", err)
	}
	sort.Strings(got.Fields)
	want := []string{"id", "region"}
	if !reflect.DeepEqual(got.Fields, want) {
		t.Errorf("got %#v, want %#v", got.Fields, want)
	}
}

func TestRestConnectorIntrospectInfersFieldTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id": 1, "region": "north", "active": true, "score": 4.5, "tags": ["a", "b"], "note": null},
			{"id": 2, "region": "south", "active": false, "score": 1, "tags": ["c"], "note": "late"}
		]`))
	}))
	defer server.Close()

	got, err := (&restConnector{url: server.URL}).Introspect(context.Background())
	if err != nil {
		t.Fatalf("Introspect returned error: %v", err)
	}

	want := map[string]string{
		"id":     "number",
		"region": "string",
		"active": "boolean",
		"score":  "number",
		"tags":   "array",
		"note":   "string",
	}
	if !reflect.DeepEqual(got.FieldTypes, want) {
		t.Errorf("got %#v, want %#v", got.FieldTypes, want)
	}
}

func TestRestConnectorIntrospectArrayRootIsRepresentativeRow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id": 1, "region": "north"}, {"id": 2, "region": "south"}]`))
	}))
	defer server.Close()

	got, err := (&restConnector{url: server.URL}).Introspect(context.Background())
	if err != nil {
		t.Fatalf("Introspect returned error: %v", err)
	}
	if len(got.Fields) != 2 {
		t.Errorf("got %d fields, want 2 (one per key, not one per array element)", len(got.Fields))
	}
}

// restQueryPlanJSON encodes a restQueryPlan the same way query.compileREST
// does, without importing the query package (which already imports this one).
func restQueryPlanJSON(t *testing.T, fields ...string) string {
	t.Helper()
	encoded, err := json.Marshal(restQueryPlan{Fields: fields})
	if err != nil {
		t.Fatalf("encode rest query plan: %v", err)
	}
	return string(encoded)
}

// RunQuery has to remap every field of a multi-field row from its api_field
// key to the our_field name the mapping points at, in the plan's order — not
// the response's key order.
func TestRestConnectorRunQueryRemapsFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id": 1, "full_name": "Ada Lovelace", "region_code": "north", "extra": "ignored"},
			{"id": 2, "full_name": "Grace Hopper", "region_code": "south", "extra": "ignored"}
		]`))
	}))
	defer server.Close()

	c := &restConnector{
		url: server.URL,
		mapping: map[string]string{
			"id":          "customer_id",
			"full_name":   "customer_name",
			"region_code": "region",
		},
	}

	plan := restQueryPlanJSON(t, "customer_name", "region", "customer_id")
	got, err := c.RunQuery(context.Background(), plan, nil, 100)
	if err != nil {
		t.Fatalf("RunQuery returned error: %v", err)
	}

	wantColumns := []string{"customer_name", "region", "customer_id"}
	if !reflect.DeepEqual(got.Columns, wantColumns) {
		t.Errorf("got columns %#v, want %#v", got.Columns, wantColumns)
	}

	wantRows := [][]any{
		{"Ada Lovelace", "north", float64(1)},
		{"Grace Hopper", "south", float64(2)},
	}
	if !reflect.DeepEqual(got.Rows, wantRows) {
		t.Errorf("got rows %#v, want %#v", got.Rows, wantRows)
	}
	if got.Truncated {
		t.Error("got Truncated true, want false — the limit was well above the row count")
	}
}

// RunQuery caps rows at the caller's limit and reports Truncated, the same as
// the other connectors.
func TestRestConnectorRunQueryRespectsLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"region_code": "north"}, {"region_code": "south"}, {"region_code": "east"}]`))
	}))
	defer server.Close()

	c := &restConnector{url: server.URL, mapping: map[string]string{"region_code": "region"}}

	plan := restQueryPlanJSON(t, "region")
	got, err := c.RunQuery(context.Background(), plan, nil, 2)
	if err != nil {
		t.Fatalf("RunQuery returned error: %v", err)
	}
	if len(got.Rows) != 2 {
		t.Errorf("got %d rows, want 2", len(got.Rows))
	}
	if !got.Truncated {
		t.Error("got Truncated false, want true — the source had more rows than the limit")
	}
}

// A report field with no mapped api_field must fail explicitly rather than
// silently return empty data for it — the documented acceptance criterion for
// issue #42.
func TestRestConnectorRunQueryFailsExplicitlyOnUnmappedField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"full_name": "Ada Lovelace"}]`))
	}))
	defer server.Close()

	c := &restConnector{url: server.URL, mapping: map[string]string{"full_name": "customer_name"}}

	// "region" isn't mapped to any api_field.
	plan := restQueryPlanJSON(t, "customer_name", "region")
	_, err := c.RunQuery(context.Background(), plan, nil, 100)
	if err == nil {
		t.Fatal("got nil error, want a failure for an unmapped field — never silent empty data")
	}
}

// A row missing one of the mapped api_fields (present in another row, absent
// in this one) reports that field as nil rather than failing the whole query
// — the same tolerance a SQL NULL gets.
func TestRestConnectorRunQueryFillsNilForFieldMissingOnARow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"full_name": "Ada Lovelace"}]`))
	}))
	defer server.Close()

	c := &restConnector{url: server.URL, mapping: map[string]string{
		"full_name":   "customer_name",
		"region_code": "region",
	}}

	plan := restQueryPlanJSON(t, "customer_name", "region")
	got, err := c.RunQuery(context.Background(), plan, nil, 100)
	if err != nil {
		t.Fatalf("RunQuery returned error: %v", err)
	}
	want := [][]any{{"Ada Lovelace", nil}}
	if !reflect.DeepEqual(got.Rows, want) {
		t.Errorf("got rows %#v, want %#v", got.Rows, want)
	}
}

func TestRestConnectorIntrospectFlattensNestedObjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id": 1, "address": {"city": "NYC", "zip": "10001"}}]`))
	}))
	defer server.Close()

	got, err := (&restConnector{url: server.URL}).Introspect(context.Background())
	if err != nil {
		t.Fatalf("Introspect returned error: %v", err)
	}
	sort.Strings(got.Fields)
	want := []string{"address.city", "address.zip", "id"}
	if !reflect.DeepEqual(got.Fields, want) {
		t.Errorf("got %#v, want %#v", got.Fields, want)
	}
}
