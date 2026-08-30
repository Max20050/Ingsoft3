package llm

import (
	"fmt"
	"strings"
)

// buildQueryPrompt frames the user's natural language request as an
// instruction. Providers send it under a "Request:" heading, after the schema,
// so this text only has to describe what to produce — not restate the schema.
func buildQueryPrompt(dialect, prompt string) string {
	return fmt.Sprintf(`Write a single read-only %s query that answers the request below, using only the tables and columns listed in the schema above.

Respond with the query text and nothing else: no explanation, no commentary, and no markdown code fences.
The query must only read data — never produce INSERT, UPDATE, DELETE, or any schema change.

Request: %s`, dialect, prompt)
}

// sanitizeQuery strips the packaging models tend to add around a query:
// markdown code fences (optionally tagged with a language), surrounding
// whitespace, and a trailing semicolon that not every dialect accepts.
func sanitizeQuery(raw string) string {
	query := strings.TrimSpace(raw)

	if fenced, ok := strings.CutPrefix(query, "```"); ok {
		query = fenced
		// A fence may be tagged (```sql); the tag runs to the end of the line.
		if newline := strings.IndexByte(query, '\n'); newline >= 0 {
			if tag := strings.TrimSpace(query[:newline]); !strings.ContainsAny(tag, " \t") {
				query = query[newline+1:]
			}
		}
		if closing := strings.LastIndex(query, "```"); closing >= 0 {
			query = query[:closing]
		}
		query = strings.TrimSpace(query)
	}

	return strings.TrimSpace(strings.TrimSuffix(query, ";"))
}
