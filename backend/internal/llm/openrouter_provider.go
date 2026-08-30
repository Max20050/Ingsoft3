package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"

// openRouterModel is "auto" rather than naming a single upstream model:
// OpenRouter routes a request tagged this way to whichever of its many
// backing models fits, which is what makes OpenRouter worth offering
// alongside a provider with one fixed model.
const openRouterModel = "openrouter/auto"

// openRouterProvider speaks OpenAI's chat-completions request and response
// shape — OpenRouter is a drop-in proxy in front of many providers, so this
// mirrors openAIProvider rather than duplicating its own client library.
type openRouterProvider struct {
	apiKey string
}

func (p *openRouterProvider) GenerateQuery(ctx context.Context, schema string, prompt string) (string, error) {
	return p.send(ctx, fmt.Sprintf("Schema:\n%s\n\nRequest:\n%s", schema, prompt))
}

func (p *openRouterProvider) GenerateText(ctx context.Context, prompt string) (string, error) {
	return p.send(ctx, prompt)
}

func (p *openRouterProvider) send(ctx context.Context, content string) (string, error) {
	reqBody := openAIRequest{
		Model: openRouterModel,
		Messages: []openAIMessage{
			{Role: "user", Content: content},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	// OpenRouter asks callers to identify themselves with these two headers —
	// unlike the others, it fronts many providers on one account, and uses
	// them for its own per-app rate limiting and abuse handling.
	req.Header.Set("HTTP-Referer", "https://github.com/Max20050/DocuWave")
	req.Header.Set("X-Title", "DocuWave")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// OpenRouter's success and error shapes both match OpenAI's.
	var parsed openAIResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode openrouter response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return "", fmt.Errorf("openrouter API error: %s", parsed.Error.Message)
		}
		return "", fmt.Errorf("openrouter API error: status %d", resp.StatusCode)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openrouter API returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}
