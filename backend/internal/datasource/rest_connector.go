package datasource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// RestHeader is one caller-supplied header sent with every request a REST API
// connector makes.
type RestHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// restAuthConfig is how a REST API source authenticates. It's stored
// encrypted as a whole, because basic/bearer/api-key credentials are secrets
// even though the shape around them (Type, HeaderName) isn't.
type restAuthConfig struct {
	Type        string `json:"type"` // "none", "basic", "bearer", or "api_key"
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Token       string `json:"token,omitempty"`
	HeaderName  string `json:"headerName,omitempty"`
	HeaderValue string `json:"headerValue,omitempty"`
}

// restAuthTypes lists the auth config shapes a REST API source can use.
var restAuthTypes = map[string]bool{
	"none":    true,
	"basic":   true,
	"bearer":  true,
	"api_key": true,
}

// validate reports whether the config carries what its declared Type needs.
func (a restAuthConfig) validate() error {
	if !restAuthTypes[a.Type] {
		return fmt.Errorf("unsupported auth type: %s", a.Type)
	}
	switch a.Type {
	case "basic":
		if a.Username == "" || a.Password == "" {
			return errors.New("basic auth requires a username and password")
		}
	case "bearer":
		if a.Token == "" {
			return errors.New("bearer auth requires a token")
		}
	case "api_key":
		if a.HeaderName == "" || a.HeaderValue == "" {
			return errors.New("api key auth requires a header name and value")
		}
	}
	return nil
}

// apply sets whatever headers the auth config implies on an outgoing request.
func (a restAuthConfig) apply(req *http.Request) {
	switch a.Type {
	case "basic":
		req.SetBasicAuth(a.Username, a.Password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+a.Token)
	case "api_key":
		req.Header.Set(a.HeaderName, a.HeaderValue)
	}
}

// restConnector reaches a user-configured REST API. Unlike the SQL
// connectors, it isn't built through NewConnector/connectorRegistry: its
// configuration (URL, headers, auth) doesn't fit ConnectionConfig, the same
// reason Google Sheets is built directly by its own handlers and resolver.
type restConnector struct {
	url     string
	method  string
	headers []RestHeader
	auth    restAuthConfig
	body    string
	// mapping is this source's stored api_field -> our_field mapping (see
	// FieldMappingStore). It's nil/empty for a connector built where no
	// mapping is needed (TestConnection, Introspect) — only RunQuery uses it.
	mapping map[string]string
}

// buildRequest assembles the configured HTTP request, applying headers and
// auth in a fixed order so a caller-supplied header can't silently shadow the
// Authorization header auth sets.
func (c *restConnector) buildRequest(ctx context.Context) (*http.Request, error) {
	method := c.method
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if c.body != "" {
		bodyReader = strings.NewReader(c.body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.url, bodyReader)
	if err != nil {
		return nil, err
	}
	for _, h := range c.headers {
		req.Header.Set(h.Key, h.Value)
	}
	c.auth.apply(req)
	return req, nil
}

// TestConnection issues the configured request and reports failure for
// anything that isn't a 2xx response.
func (c *restConnector) TestConnection(ctx context.Context) error {
	req, err := c.buildRequest(ctx)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, c.url, string(body))
	}
	return nil
}

// QueryLanguage names what compiled queries against this source are shown as.
func (c *restConnector) QueryLanguage() string {
	return "REST API"
}

// fetchJSON issues the configured request and decodes its body as JSON.
func (c *restConnector) fetchJSON(ctx context.Context) (any, error) {
	req, err := c.buildRequest(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, c.url, string(body))
	}

	var decoded any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode response as JSON: %w", err)
	}
	return decoded, nil
}

// resultRows finds the array of result objects in a decoded JSON response: the
// response itself if it's an array, or the first array found one level down
// (the common "{ data: [...] }" / "{ results: [...] }" shape). It's the same
// question Introspect and RunQuery both need answered.
func resultRows(decoded any) ([]any, error) {
	switch v := decoded.(type) {
	case []any:
		return v, nil
	case map[string]any:
		for _, value := range v {
			if arr, ok := value.([]any); ok {
				return arr, nil
			}
		}
		// No array field: treat the object itself as the one result row.
		return []any{v}, nil
	default:
		return nil, errors.New("response is neither a JSON array nor object")
	}
}

// Introspect fetches the configured response and reports the fields found on
// its first result object, flattening nested objects to dot-notation, plus a
// simple inferred type per field.
func (c *restConnector) Introspect(ctx context.Context) (Schema, error) {
	decoded, err := c.fetchJSON(ctx)
	if err != nil {
		return Schema{}, err
	}

	rows, err := resultRows(decoded)
	if err != nil {
		return Schema{}, err
	}
	if len(rows) == 0 {
		return Schema{Fields: []string{}}, nil
	}

	first, ok := rows[0].(map[string]any)
	if !ok {
		return Schema{}, errors.New("result rows are not JSON objects")
	}

	fields := flattenFields("", first)
	return Schema{Fields: fields, FieldTypes: inferFieldTypes(fields, rows)}, nil
}

// flattenFields lists an object's keys as dot-notated field names, descending
// into nested objects so "user": {"city": ...} becomes "user.city".
func flattenFields(prefix string, obj map[string]any) []string {
	fields := make([]string, 0, len(obj))
	for key, value := range obj {
		name := key
		if prefix != "" {
			name = prefix + "." + key
		}
		if nested, ok := value.(map[string]any); ok {
			fields = append(fields, flattenFields(name, nested)...)
			continue
		}
		fields = append(fields, name)
	}
	return fields
}

// fieldValue navigates a decoded JSON object by a dot-notated path, the same
// notation flattenFields produces, returning ok=false if any segment is
// missing.
func fieldValue(row map[string]any, path string) (any, bool) {
	var cur any = row
	for part := range strings.SplitSeq(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// inferFieldTypes reports one simple type per field, taken from the first
// non-null value found for that field across rows — a single representative
// row might happen to have null in a field another row fills in.
func inferFieldTypes(fields []string, rows []any) map[string]string {
	types := make(map[string]string, len(fields))
	for _, field := range fields {
		typ := "unknown"
		for _, row := range rows {
			obj, ok := row.(map[string]any)
			if !ok {
				continue
			}
			value, ok := fieldValue(obj, field)
			if !ok || value == nil {
				continue
			}
			typ = jsonValueType(value)
			break
		}
		types[field] = typ
	}
	return types
}

// jsonValueType names the simple type a decoded JSON value counts as.
func jsonValueType(value any) string {
	switch value.(type) {
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

// restQueryPlan mirrors query.restPlan's JSON shape — the compiled form of a
// REST spec: which mapped (our_field) fields to select, in order. Duplicated
// here rather than imported because backend/internal/query already imports
// this package (to validate specs against a Schema), so the reverse import
// would cycle; only the wire shape needs to be shared.
type restQueryPlan struct {
	Fields []string `json:"fields"`
}

// RunQuery executes the configured request, decodes its JSON response, and
// remaps each result row's api_field keys to their mapped our_field name
// using the source's stored field mapping (see FieldMappingStore) — keeping
// only the fields the compiled plan selected, in that order.
//
// Every plan field is required to have a mapped api_field: report.Runner
// builds the schema a spec is validated against from this same mapping, so
// under normal use every field reaching here is already covered. This is
// still checked explicitly (rather than trusted) so a mapping edited after a
// report was built fails loudly instead of silently returning an empty or
// partial row — never silent empty data.
//
// REST sources support only "select and reorder already-mapped fields" (see
// query.compileREST): the plan carries no filters, sorts, or aggregates.
// Pagination is out of scope — this fetches the configured response once;
// a source whose data spans multiple pages only returns its first page,
// which is a known limitation rather than an oversight.
func (c *restConnector) RunQuery(ctx context.Context, query string, args []any, limit int) (QueryResult, error) {
	if len(args) > 0 {
		return QueryResult{}, errors.New("rest api queries cannot take bound values")
	}

	var plan restQueryPlan
	if err := json.Unmarshal([]byte(query), &plan); err != nil {
		return QueryResult{}, fmt.Errorf("decode rest query plan: %w", err)
	}
	if len(plan.Fields) == 0 {
		return QueryResult{}, errors.New("rest api query selects no fields")
	}

	// ourToAPI inverts the stored api_field -> our_field mapping, since the
	// plan names fields by their mapped our_field name.
	ourToAPI := make(map[string]string, len(c.mapping))
	for apiField, ourField := range c.mapping {
		ourToAPI[ourField] = apiField
	}

	for _, field := range plan.Fields {
		if _, ok := ourToAPI[field]; !ok {
			return QueryResult{}, fmt.Errorf(
				"field %q has no mapped API field — map it in the data source's field mapping before running this report", field)
		}
	}

	decoded, err := c.fetchJSON(ctx)
	if err != nil {
		return QueryResult{}, err
	}
	rows, err := resultRows(decoded)
	if err != nil {
		return QueryResult{}, err
	}

	result := QueryResult{Columns: plan.Fields, Rows: make([][]any, 0, len(rows))}
	for _, raw := range rows {
		if len(result.Rows) == limit {
			result.Truncated = true
			break
		}
		obj, ok := raw.(map[string]any)
		if !ok {
			return QueryResult{}, errors.New("result rows are not JSON objects")
		}

		row := make([]any, len(plan.Fields))
		for i, field := range plan.Fields {
			value, ok := fieldValue(obj, ourToAPI[field])
			if !ok {
				row[i] = nil
				continue
			}
			row[i] = value
		}
		result.Rows = append(result.Rows, row)
	}

	return result, nil
}
