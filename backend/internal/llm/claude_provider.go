package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const claudeAPIURL = "https://api.anthropic.com/v1/messages"
const claudeModel = "claude-sonnet-5"

type claudeProvider struct {
	apiKey string
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *claudeProvider) GenerateQuery(ctx context.Context, schema string, prompt string) (string, error) {
	return p.send(ctx, fmt.Sprintf("Schema:\n%s\n\nRequest:\n%s", schema, prompt))
}

func (p *claudeProvider) GenerateText(ctx context.Context, prompt string) (string, error) {
	return p.send(ctx, prompt)
}

func (p *claudeProvider) send(ctx context.Context, content string) (string, error) {
	reqBody := claudeRequest{
		Model:     claudeModel,
		MaxTokens: 1024,
		Messages: []claudeMessage{
			{Role: "user", Content: content},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed claudeResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode claude response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return "", fmt.Errorf("claude API error: %s", parsed.Error.Message)
		}
		return "", fmt.Errorf("claude API error: status %d", resp.StatusCode)
	}

	if len(parsed.Content) == 0 {
		return "", fmt.Errorf("claude API returned no content")
	}
	return parsed.Content[0].Text, nil
}
