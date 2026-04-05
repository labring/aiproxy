//go:build enterprise

package ppio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	DefaultPPIOAPIBase        = "https://api.ppinfra.com/v3/openai"
	DefaultPPIOAnthropicBase  = "https://api.ppinfra.com/anthropic"
	ppioModelsEndpoint        = "https://api.ppinfra.com/v3/openai/models"
	ppioMgmtModelsEndpoint    = "https://api-server.ppinfra.com/v1/product/model/list"
	defaultPPIOTimeout        = 30 * time.Second
	ppioMaxResponseSize       = 50 << 20 // 50 MB
)

// PPIOClient handles communication with PPIO API
type PPIOClient struct {
	APIKey  string
	APIBase string
	client  *http.Client
}

// NewPPIOClient creates a new PPIO API client.
// Priority: database config (from selected channel) > environment variables.
func NewPPIOClient() (*PPIOClient, error) {
	cfg := GetPPIOConfig()
	apiKey := cfg.APIKey
	apiBase := cfg.APIBase

	// Fall back to environment variables
	if apiKey == "" {
		apiKey = os.Getenv("PPIO_API_KEY")
	}

	if apiKey == "" {
		return nil, errors.New("PPIO API Key is not configured. Please select a PPIO channel in the Sync page or set PPIO_API_KEY environment variable")
	}

	if apiBase == "" {
		apiBase = os.Getenv("PPIO_API_BASE")
	}

	if apiBase == "" {
		apiBase = DefaultPPIOAPIBase
	}

	return &PPIOClient{
		APIKey:  apiKey,
		APIBase: apiBase,
		client: &http.Client{
			Timeout: defaultPPIOTimeout,
		},
	}, nil
}

// FetchModels fetches all models from PPIO API.
// Always uses DefaultPPIOAPIBase (/v1) for the models endpoint,
// since the channel's base URL may point to /openai or /anthropic.
func (c *PPIOClient) FetchModels(ctx context.Context) ([]PPIOModel, error) {
	url := ppioModelsEndpoint

	ctx, cancel := context.WithTimeout(ctx, defaultPPIOTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("PPIO API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, ppioMaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var modelsResp PPIOModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return modelsResp.Data, nil
}

// fetchMgmtModels calls the PPIO management model-list API with the given query
// string and returns the parsed model slice.
func (c *PPIOClient) fetchMgmtModels(ctx context.Context, mgmtToken, query string) ([]PPIOModelV2, error) {
	url := ppioMgmtModelsEndpoint + query

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+mgmtToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from mgmt API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("PPIO mgmt API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, ppioMaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var mgmtResp PPIOMgmtModelsResponse
	if err := json.Unmarshal(body, &mgmtResp); err != nil {
		return nil, fmt.Errorf("failed to parse mgmt response: %w", err)
	}

	if mgmtResp.Code != 0 {
		return nil, fmt.Errorf("PPIO mgmt API error: code=%d, message=%s", mgmtResp.Code, mgmtResp.Message)
	}

	return mgmtResp.Data, nil
}

// FetchAllModels fetches the full model catalog (including pa/ closed-source models)
// via the PPIO management API using the mgmt console token.
//
// PPIO's list API only returns chat-type models when queried with ?visibility=1.
// Non-chat types (e.g. embedding) require a separate ?model_type=<type> request.
// Currently only embedding models exist in non-chat categories; the function makes
// one extra request to merge them in.
func (c *PPIOClient) FetchAllModels(ctx context.Context, mgmtToken string) ([]PPIOModelV2, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultPPIOTimeout)
	defer cancel()

	// Fetch chat models (the bulk of the catalog, including pa/ closed-source).
	chatModels, err := c.fetchMgmtModels(ctx, mgmtToken, "?visibility=1")
	if err != nil {
		return nil, err
	}

	// Fetch embedding models separately — they are not returned by ?visibility=1.
	// Use visibility=1 to include pa/ closed-source embedding models alongside public ones.
	embeddingModels, embErr := c.fetchMgmtModels(ctx, mgmtToken, "?visibility=1&model_type=embedding")
	if embErr != nil {
		log.Printf("PPIO sync: failed to fetch embedding models (non-fatal): %v", embErr)
		return chatModels, nil
	}

	// Merge: skip any embedding model already present in the chat list (shouldn't
	// happen in practice but guards against API changes).
	chatSet := make(map[string]struct{}, len(chatModels))
	for _, m := range chatModels {
		chatSet[m.ID] = struct{}{}
	}

	for _, m := range embeddingModels {
		if _, exists := chatSet[m.ID]; !exists {
			chatModels = append(chatModels, m)
		}
	}

	return chatModels, nil
}

// FetchAllModelsMerged fetches models from both V1 (public) and V2 (mgmt) APIs
// and merges them into a single V2 list. V2 wins on ID overlap (richer data).
// If mgmtToken is empty, only V1 models are returned (converted to V2 format).
func (c *PPIOClient) FetchAllModelsMerged(ctx context.Context, mgmtToken string) ([]PPIOModelV2, error) {
	// Always fetch V1 (public API)
	v1Models, v1Err := c.FetchModels(ctx)

	var v2Models []PPIOModelV2

	if mgmtToken != "" {
		var v2Err error

		v2Models, v2Err = c.FetchAllModels(ctx, mgmtToken)
		if v2Err != nil {
			return nil, fmt.Errorf("failed to fetch models from mgmt API: %w", v2Err)
		}

		if v1Err != nil {
			log.Printf("PPIO sync: V1 API fetch failed (non-fatal, using V2 only): %v", v1Err)
			return v2Models, nil
		}
	} else {
		// No mgmt token — V1 is the only source
		if v1Err != nil {
			return nil, fmt.Errorf("failed to fetch models: %w", v1Err)
		}
	}

	// Merge: V2 wins on overlap (richer data with tiered billing, cache, RPM/TPM)
	v2Set := make(map[string]struct{}, len(v2Models))
	for _, m := range v2Models {
		v2Set[m.ID] = struct{}{}
	}

	for _, m := range v1Models {
		if _, exists := v2Set[m.ID]; !exists {
			v2Models = append(v2Models, m.ToV2())
		}
	}

	return v2Models, nil
}
