package datasource

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

// googleSheetsConnector verifies that a specific spreadsheet is reachable
// with the connected account's OAuth token. It implements the same Connector
// interface as the SQL connectors.
type googleSheetsConnector struct {
	oauthConfig   *oauth2.Config
	token         *oauth2.Token
	spreadsheetID string
}

// get performs an authorized GET against the Sheets API, decoding the
// response into target when one is supplied.
func (c *googleSheetsConnector) get(ctx context.Context, reqURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.oauthConfig.Client(ctx, c.token).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d from sheets api: %s", resp.StatusCode, string(body))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *googleSheetsConnector) TestConnection(ctx context.Context) error {
	reqURL := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s?fields=spreadsheetId", c.spreadsheetID)
	return c.get(ctx, reqURL, nil)
}

type spreadsheetProperties struct {
	Sheets []struct {
		Properties struct {
			Title string `json:"title"`
		} `json:"properties"`
	} `json:"sheets"`
}

type valueRange struct {
	Values [][]string `json:"values"`
}

// firstSheetTitle returns the title of the spreadsheet's first sheet, which is
// the one this connector reads.
func (c *googleSheetsConnector) firstSheetTitle(ctx context.Context) (string, error) {
	var properties spreadsheetProperties
	metaURL := fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s?fields=sheets.properties.title", c.spreadsheetID)
	if err := c.get(ctx, metaURL, &properties); err != nil {
		return "", err
	}
	if len(properties.Sheets) == 0 {
		return "", errors.New("spreadsheet has no sheets")
	}
	return properties.Sheets[0].Properties.Title, nil
}

// Introspect reads the header row of the spreadsheet's first sheet, which is
// the closest equivalent Sheets has to a column list.
func (c *googleSheetsConnector) Introspect(ctx context.Context) (Schema, error) {
	title, err := c.firstSheetTitle(ctx)
	if err != nil {
		return Schema{}, err
	}

	// A1 notation quotes the sheet title, and escapes any quote inside it by doubling.
	headerRange := fmt.Sprintf("'%s'!1:1", strings.ReplaceAll(title, "'", "''"))

	var header valueRange
	valuesURL := fmt.Sprintf(
		"https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s",
		c.spreadsheetID, url.PathEscape(headerRange),
	)
	if err := c.get(ctx, valuesURL, &header); err != nil {
		return Schema{}, err
	}

	return Schema{Fields: headerFields(header.Values)}, nil
}

func (c *googleSheetsConnector) QueryLanguage() string {
	return "Google Visualization API Query Language"
}

// gvizResponse is the payload of the Google Visualization endpoint, the only
// interface Sheets offers that runs a query rather than returning raw cells.
type gvizResponse struct {
	Status string `json:"status"`
	Errors []struct {
		Message         string `json:"message"`
		DetailedMessage string `json:"detailed_message"`
	} `json:"errors"`
	Table struct {
		Cols []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
			Type  string `json:"type"`
		} `json:"cols"`
		Rows []struct {
			C []*struct {
				V any    `json:"v"`
				F string `json:"f"`
			} `json:"c"`
		} `json:"rows"`
	} `json:"table"`
}

// unwrapGviz strips the JSONP padding the visualization endpoint wraps its
// JSON in (`/*O_o*/ google.visualization.Query.setResponse({...});`).
func unwrapGviz(body []byte) ([]byte, error) {
	start := bytes.IndexByte(body, '{')
	end := bytes.LastIndexByte(body, '}')
	if start < 0 || end < start {
		return nil, errors.New("unexpected response from the sheets query endpoint")
	}
	return body[start : end+1], nil
}

// gvizCellValue picks the value to show for a cell. Dates arrive as an opaque
// "Date(2024,0,15)" construct, so the formatted string is the useful one.
func gvizCellValue(raw any, formatted string, columnType string) any {
	switch columnType {
	case "date", "datetime", "timeofday":
		if formatted != "" {
			return formatted
		}
	}
	return raw
}

// RunQuery runs a visualization query against the spreadsheet's first sheet.
// The endpoint is inherently read-only, so unlike the SQL connectors there's
// no transaction to open.
//
// The endpoint takes no parameters, so the compiler writes values into the query
// itself and hands none over here; args arriving non-empty would mean values
// that were never applied, which is worse than an error.
func (c *googleSheetsConnector) RunQuery(ctx context.Context, query string, args []any, limit int) (QueryResult, error) {
	if len(args) > 0 {
		return QueryResult{}, errors.New("google sheets queries cannot take bound values")
	}

	title, err := c.firstSheetTitle(ctx)
	if err != nil {
		return QueryResult{}, err
	}

	params := url.Values{}
	params.Set("tqx", "out:json")
	params.Set("headers", "1") // the first row is the header, matching Introspect
	params.Set("sheet", title)
	params.Set("tq", query)
	reqURL := fmt.Sprintf(
		"https://docs.google.com/spreadsheets/d/%s/gviz/tq?%s",
		c.spreadsheetID, params.Encode(),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return QueryResult{}, err
	}
	resp, err := c.oauthConfig.Client(ctx, c.token).Do(req)
	if err != nil {
		return QueryResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return QueryResult{}, err
	}
	payload, err := unwrapGviz(body)
	if err != nil {
		// A non-JSON body here is usually an auth or sharing failure, which
		// arrives as an HTML sign-in page rather than a query error.
		return QueryResult{}, fmt.Errorf("sheets query failed with status %d", resp.StatusCode)
	}

	var parsed gvizResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return QueryResult{}, fmt.Errorf("decode sheets query response: %w", err)
	}
	if parsed.Status == "error" {
		return QueryResult{}, errors.New(gvizErrorMessage(parsed))
	}

	return gvizResult(parsed, limit), nil
}

func gvizErrorMessage(parsed gvizResponse) string {
	if len(parsed.Errors) == 0 {
		return "the sheets query was rejected"
	}
	first := parsed.Errors[0]
	if first.DetailedMessage != "" {
		return first.DetailedMessage
	}
	return first.Message
}

// gvizResult flattens a decoded visualization response into a QueryResult.
func gvizResult(parsed gvizResponse, limit int) QueryResult {
	result := QueryResult{
		Columns: make([]string, 0, len(parsed.Table.Cols)),
		Rows:    make([][]any, 0, len(parsed.Table.Rows)),
	}
	for _, col := range parsed.Table.Cols {
		label := col.Label
		if label == "" {
			label = col.ID
		}
		result.Columns = append(result.Columns, label)
	}

	for _, row := range parsed.Table.Rows {
		if len(result.Rows) == limit {
			result.Truncated = true
			break
		}
		values := make([]any, len(result.Columns))
		for i := range values {
			// Trailing empty cells are omitted, and empty ones arrive as null.
			if i >= len(row.C) || row.C[i] == nil {
				continue
			}
			columnType := ""
			if i < len(parsed.Table.Cols) {
				columnType = parsed.Table.Cols[i].Type
			}
			values[i] = gvizCellValue(row.C[i].V, row.C[i].F, columnType)
		}
		result.Rows = append(result.Rows, values)
	}

	return result
}
