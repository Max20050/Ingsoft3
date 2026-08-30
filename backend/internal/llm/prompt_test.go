package llm

import (
	"strings"
	"testing"
)

func TestSanitizeQuery(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "plain query is left alone",
			raw:  "SELECT 1",
			want: "SELECT 1",
		},
		{
			name: "trims surrounding whitespace",
			raw:  "\n  SELECT 1  \n",
			want: "SELECT 1",
		},
		{
			name: "strips a tagged code fence",
			raw:  "```sql\nSELECT region FROM sales\n```",
			want: "SELECT region FROM sales",
		},
		{
			name: "strips an untagged code fence",
			raw:  "```\nSELECT 1\n```",
			want: "SELECT 1",
		},
		{
			name: "strips a trailing semicolon the sheets dialect rejects",
			raw:  "SELECT A, sum(B) GROUP BY A;",
			want: "SELECT A, sum(B) GROUP BY A",
		},
		{
			name: "keeps the first line when the fence has no language tag on it",
			raw:  "```SELECT 1```",
			want: "SELECT 1",
		},
		{
			name: "empty response stays empty",
			raw:  "   ",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeQuery(tt.raw); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A fenced block whose first line is prose, not a language tag, must keep that
// line — dropping it would silently truncate the query.
func TestSanitizeQueryKeepsMultiWordFirstLine(t *testing.T) {
	got := sanitizeQuery("```\nSELECT a FROM b\nWHERE a > 1\n```")
	want := "SELECT a FROM b\nWHERE a > 1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildQueryPromptNamesDialectAndRequest(t *testing.T) {
	prompt := buildQueryPrompt("PostgreSQL SQL", "sum of sales by region")

	for _, want := range []string{"PostgreSQL SQL", "sum of sales by region", "read-only"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt does not mention %q:\n%s", want, prompt)
		}
	}
}
