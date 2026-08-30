package datasource

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalizeValueMakesDriverTypesJSONSafe(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want any
	}{
		{name: "nil stays nil", in: nil, want: nil},
		{name: "bytes become text", in: []byte("north"), want: "north"},
		{name: "times become RFC3339", in: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), want: "2026-01-02T03:04:05Z"},
		{name: "numbers pass through", in: int64(7), want: int64(7)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeValue(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestColumnLetter(t *testing.T) {
	tests := []struct {
		index int
		want  string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
	}

	for _, tt := range tests {
		if got := columnLetter(tt.index); got != tt.want {
			t.Errorf("columnLetter(%d) = %q, want %q", tt.index, got, tt.want)
		}
	}
}

func TestSchemaDescribeSQL(t *testing.T) {
	schema := Schema{Tables: []Table{
		{Name: "orders", Columns: []Column{{Name: "id", Type: "uuid"}, {Name: "total", Type: "numeric"}}},
		{Name: "sales.regions", Columns: []Column{{Name: "name", Type: "text"}}},
	}}

	want := strings.Join([]string{
		"Table orders",
		"  - id (uuid)",
		"  - total (numeric)",
		"Table sales.regions",
		"  - name (text)",
	}, "\n")

	if got := schema.Describe(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Sheets sources are described by column letter because that's how their
// query language addresses columns.
func TestSchemaDescribeSheetsUsesColumnLetters(t *testing.T) {
	schema := Schema{Fields: []string{"Region", "", "Sales"}}

	want := strings.Join([]string{
		"Column A: Region",
		"Column B: (unnamed)",
		"Column C: Sales",
	}, "\n")

	if got := schema.Describe(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSchemaDescribeEmpty(t *testing.T) {
	if got := (Schema{}).Describe(); got == "" {
		t.Error("got an empty description; the LLM needs something to read")
	}
}

func TestQueryResultJSONShape(t *testing.T) {
	body, err := json.Marshal(QueryResult{
		Columns: []string{"region", "total"},
		Rows:    [][]any{{"North", 10}},
	})
	if err != nil {
		t.Fatalf("marshal returned error: %v", err)
	}

	want := `{"columns":["region","total"],"rows":[["North",10]],"truncated":false}`
	if string(body) != want {
		t.Errorf("got %s, want %s", body, want)
	}
}
