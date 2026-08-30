package datasource

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestUnwrapGvizStripsJSONPPadding(t *testing.T) {
	body := []byte("/*O_o*/\ngoogle.visualization.Query.setResponse({\"status\":\"ok\"});")

	got, err := unwrapGviz(body)
	if err != nil {
		t.Fatalf("unwrapGviz returned error: %v", err)
	}
	if string(got) != `{"status":"ok"}` {
		t.Errorf("got %s", got)
	}
}

// An HTML sign-in page is what an unauthorized request gets back, and it has
// to be reported as a failure rather than parsed.
func TestUnwrapGvizRejectsNonJSON(t *testing.T) {
	if _, err := unwrapGviz([]byte("<html>sign in</html>")); err == nil {
		t.Error("got nil error for an HTML body, want a failure")
	}
}

func decodeGviz(t *testing.T, payload string) gvizResponse {
	t.Helper()
	var parsed gvizResponse
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	return parsed
}

func TestGvizResultFlattensRows(t *testing.T) {
	parsed := decodeGviz(t, `{
		"status":"ok",
		"table":{
			"cols":[
				{"id":"A","label":"Region","type":"string"},
				{"id":"B","label":"","type":"number"},
				{"id":"C","label":"Day","type":"date"}
			],
			"rows":[
				{"c":[{"v":"North"},{"v":10.0,"f":"10"},{"v":"Date(2026,0,15)","f":"1/15/2026"}]},
				{"c":[{"v":"South"},null]}
			]
		}
	}`)

	got := gvizResult(parsed, 10)

	wantColumns := []string{"Region", "B", "Day"}
	if !reflect.DeepEqual(got.Columns, wantColumns) {
		t.Errorf("columns: got %#v, want %#v", got.Columns, wantColumns)
	}
	// Dates use the formatted value; the raw one is an opaque Date() construct.
	wantRows := [][]any{
		{"North", 10.0, "1/15/2026"},
		{"South", nil, nil},
	}
	if !reflect.DeepEqual(got.Rows, wantRows) {
		t.Errorf("rows: got %#v, want %#v", got.Rows, wantRows)
	}
	if got.Truncated {
		t.Error("got truncated = true, want false")
	}
}

func TestGvizResultTruncatesAtLimit(t *testing.T) {
	parsed := decodeGviz(t, `{
		"status":"ok",
		"table":{
			"cols":[{"id":"A","label":"Region","type":"string"}],
			"rows":[{"c":[{"v":"North"}]},{"c":[{"v":"South"}]},{"c":[{"v":"East"}]}]
		}
	}`)

	got := gvizResult(parsed, 2)

	if len(got.Rows) != 2 {
		t.Errorf("got %d rows, want 2", len(got.Rows))
	}
	if !got.Truncated {
		t.Error("got truncated = false, want true")
	}
}

func TestGvizErrorMessagePrefersDetail(t *testing.T) {
	parsed := decodeGviz(t, `{
		"status":"error",
		"errors":[{"message":"INVALID_QUERY","detailed_message":"Column D does not exist"}]
	}`)

	if got := gvizErrorMessage(parsed); got != "Column D does not exist" {
		t.Errorf("got %q", got)
	}
}

func TestGvizErrorMessageFallsBackWhenEmpty(t *testing.T) {
	if got := gvizErrorMessage(gvizResponse{}); got == "" {
		t.Error("got an empty message; the user needs something to read")
	}
}
