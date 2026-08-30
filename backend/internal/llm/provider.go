// Package llm manages user-configured LLM providers: storing their (encrypted)
// API keys and calling the selected provider.
//
// Report queries are no longer written by a model. They're compiled from a
// structured specification against the data source's own schema (see
// internal/query), so nothing a model produces is ever executed. This plumbing
// — the per-user provider choice, the encrypted key, the provider registry —
// stays because it's what will write the conclusions drawn from a report's data.
package llm

import (
	"context"
	"fmt"
)

// Provider generates a query from a data source schema and a natural language prompt.
// Each supported LLM provider implements this behind NewProvider, so adding a new
// provider never requires changing store/handler logic.
type Provider interface {
	GenerateQuery(ctx context.Context, schema string, prompt string) (string, error)
	// GenerateText answers a single prompt with free text — no schema framing,
	// no query-shaped expectations. It's what a report's ai-summary blocks call.
	GenerateText(ctx context.Context, prompt string) (string, error)
}

type providerFactory func(apiKey string) Provider

var providerRegistry = map[string]providerFactory{
	"claude":     func(apiKey string) Provider { return &claudeProvider{apiKey: apiKey} },
	"openai":     func(apiKey string) Provider { return &openAIProvider{apiKey: apiKey} },
	"openrouter": func(apiKey string) Provider { return &openRouterProvider{apiKey: apiKey} },
}

// SupportedProviders lists the provider identifiers accepted by NewProvider.
var SupportedProviders = []string{"claude", "openai", "openrouter"}

// NewProvider builds a Provider for the given provider type.
func NewProvider(providerType string, apiKey string) (Provider, error) {
	factory, ok := providerRegistry[providerType]
	if !ok {
		return nil, fmt.Errorf("unsupported LLM provider: %s", providerType)
	}
	return factory(apiKey), nil
}
