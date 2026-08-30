package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openAIAPIURL = "https://api.openai.com/v1/chat/completions"
const openAIModel = "gpt-4o"

type openAIProvider struct {
	apiKey string
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *openAIProvider) GenerateQuery(ctx context.Context, schema string, prompt string) (string, error) {
	return p.send(ctx, fmt.Sprintf("Schema:\n%s\n\nRequest:\n%s", schema, prompt))
}

func (p *openAIProvider) GenerateText(ctx context.Context, prompt string) (string, error) {
	return p.send(ctx, prompt)
}

func (p *openAIProvider) send(ctx context.Context, content string) (string, error) {
	reqBody := openAIRequest{
		Model: openAIModel,
		Messages: []openAIMessage{
			{Role: "user", Content: content},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed openAIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode openai response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return "", fmt.Errorf("openai API error: %s", parsed.Error.Message)
		}
		return "", fmt.Errorf("openai API error: status %d", resp.StatusCode)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai API returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
