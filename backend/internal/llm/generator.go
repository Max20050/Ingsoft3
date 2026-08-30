package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrProviderFailed wraps anything the provider's API rejected — a bad key, a
// rate limit, an outage — so callers can tell it apart from a local failure.
var ErrProviderFailed = errors.New("LLM provider request failed")

// ErrEmptyQuery is returned when the provider answered, but with nothing that
// looks like a query.
var ErrEmptyQuery = errors.New("the LLM returned an empty query")

// QueryRequest describes the query to generate: the dialect the data source
// speaks, its schema as text, and what the user asked for in their own words.
type QueryRequest struct {
	Dialect string
	Schema  string
	Prompt  string
}

// Generator calls the calling user's own configured provider with their own API
// key.
//
// Nothing wires GenerateQuery into the report flow any more: a report's query is
// compiled from a structured specification, and no query text from a client — or
// from a model — is executed. What's kept here is the path from a stored
// provider choice to a provider call, which is what report conclusions will use.
type Generator struct {
	store     *Store
	encryptor *Encryptor
}

func NewGenerator(store *Store, encryptor *Encryptor) *Generator {
	return &Generator{store: store, encryptor: encryptor}
}

// GenerateQuery returns a query for the given request. It returns ErrNotFound
// when the user has configured no provider, and ErrProviderFailed when the
// provider's API rejected the call.
func (g *Generator) GenerateQuery(ctx context.Context, userID string, req QueryRequest) (string, error) {
	cfg, encryptedAPIKey, err := g.store.GetWithKey(ctx, userID)
	if err != nil {
		return "", err
	}

	apiKey, err := g.encryptor.Decrypt(encryptedAPIKey)
	if err != nil {
		return "", fmt.Errorf("decrypt API key: %w", err)
	}

	provider, err := NewProvider(cfg.Provider, apiKey)
	if err != nil {
		return "", err
	}

	raw, err := provider.GenerateQuery(ctx, req.Schema, buildQueryPrompt(req.Dialect, req.Prompt))
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrProviderFailed, err)
	}

	query := sanitizeQuery(raw)
	if query == "" {
		return "", ErrEmptyQuery
	}
	return query, nil
}

// ErrEmptySummary is returned when the provider answered, but with nothing
// worth printing.
var ErrEmptySummary = errors.New("the LLM returned an empty summary")

// GenerateSummary answers a free-text prompt with the calling user's own
// configured provider — what a report's ai-summary blocks call to turn their
// selected data and instructions into printed text. It returns ErrNotFound
// when the user has configured no provider, and ErrProviderFailed when the
// provider's API rejected the call.
func (g *Generator) GenerateSummary(ctx context.Context, userID string, prompt string) (string, error) {
	cfg, encryptedAPIKey, err := g.store.GetWithKey(ctx, userID)
	if err != nil {
		return "", err
	}

	apiKey, err := g.encryptor.Decrypt(encryptedAPIKey)
	if err != nil {
		return "", fmt.Errorf("decrypt API key: %w", err)
	}

	provider, err := NewProvider(cfg.Provider, apiKey)
	if err != nil {
		return "", err
	}

	raw, err := provider.GenerateText(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrProviderFailed, err)
	}

	text := strings.TrimSpace(raw)
	if text == "" {
		return "", ErrEmptySummary
	}
	return text, nil
}

// HasProvider reports whether userID has an LLM provider configured, without
// decrypting their key — what a report's save path checks before it'll let a
// user activate an ai-summary block they can't yet run.
func (g *Generator) HasProvider(ctx context.Context, userID string) (bool, error) {
	_, err := g.store.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
