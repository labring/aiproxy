//go:build enterprise

package ppio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	DefaultPPIOAPIBase        = "https://api.ppinfra.com/v3/openai"
	DefaultPPIOAnthropicBase  = "https://api.ppinfra.com/anthropic"
	// DefaultPPIOMultimodalBase is the base URL for PPIO native multimodal channels
	// (image, video, audio). The path suffix is provided by the request itself.
	DefaultPPIOMultimodalBase = "https://api.ppinfra.com"
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

// multimodalModelTypes lists the PPIO management API model_type values that
// cover non-chat multimodal models. Each requires its own API request since
// the default ?visibility=1 query only returns chat-family models.
var multimodalModelTypes = []string{"embedding", "image", "video", "audio"}


// FetchAllModels fetches the full model catalog (including pa/ closed-source models)
// via the PPIO management API using the mgmt console token.
//
// PPIO's list API only returns chat-type models when queried with ?visibility=1.
// Non-chat types (embedding, image, video, audio) require separate requests.
func (c *PPIOClient) FetchAllModels(ctx context.Context, mgmtToken string) ([]PPIOModelV2, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultPPIOTimeout)
	defer cancel()

	// Fetch chat models (the bulk of the catalog, including pa/ closed-source).
	allModels, err := c.fetchMgmtModels(ctx, mgmtToken, "?visibility=1")
	if err != nil {
		return nil, err
	}

	// Build a set of already-fetched model IDs to avoid duplicates.
	seen := make(map[string]struct{}, len(allModels))
	for _, m := range allModels {
		seen[m.ID] = struct{}{}
	}

	// Fetch each non-chat type separately and merge.
	for _, modelType := range multimodalModelTypes {
		extra, extraErr := c.fetchMgmtModels(ctx, mgmtToken, "?model_type="+modelType)
		if extraErr != nil {
			log.Printf("PPIO sync: failed to fetch %s models (non-fatal): %v", modelType, extraErr)
			continue
		}

		for _, m := range extra {
			if _, exists := seen[m.ID]; !exists {
				seen[m.ID] = struct{}{}
				allModels = append(allModels, m)
			}
		}
	}

	return allModels, nil
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
			v1Models = nil // treat as empty; fall through to known-model merge
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
			v2Set[m.ID] = struct{}{} // keep set current for dedup
			v2Models = append(v2Models, m.ToV2())
		}
	}

	return v2Models, nil
}

// ── Multimodal API (api-server.ppio.com) ──────────────────────────────────

const (
	ppioMultimodalModelsEndpoint = "https://api-server.ppio.com/v1/product/multimodal-model/list"
	ppioBatchPriceEndpoint       = "https://api-server.ppio.com/v1/product/batch-price"
)

// FetchMultimodalModels fetches all multimodal models (image/video/audio) from
// the PPIO console API. Requires the mgmt console token for authentication.
func (c *PPIOClient) FetchMultimodalModels(ctx context.Context, mgmtToken string) ([]PPIOMultimodalModel, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultPPIOTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ppioMultimodalModelsEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+mgmtToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch multimodal models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("multimodal API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, ppioMaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var listResp PPIOMultimodalListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse multimodal response: %w", err)
	}

	return listResp.Configs, nil
}

// FetchMultimodalPrices fetches batch SKU pricing for the given SKU codes.
// Returns a map of skuCode → raw basePrice0 value (divide by multimodalPriceDivisor for 元/次).
func (c *PPIOClient) FetchMultimodalPrices(ctx context.Context, mgmtToken string, skuCodes []string) (map[string]int64, error) {
	if len(skuCodes) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, defaultPPIOTimeout)
	defer cancel()

	reqBody := PPIOBatchPriceRequest{
		BusinessType: "model_api",
		ProductIDs:   skuCodes,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch-price request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ppioBatchPriceEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+mgmtToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch batch prices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("batch-price API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, ppioMaxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var priceResp PPIOBatchPriceResponse
	if err := json.Unmarshal(body, &priceResp); err != nil {
		return nil, fmt.Errorf("failed to parse batch-price response: %w", err)
	}

	result := make(map[string]int64, len(priceResp.Products))
	for _, p := range priceResp.Products {
		if p.BasePrice0 == "" || p.BasePrice0 == "0" {
			continue
		}

		raw, err := strconv.ParseInt(p.BasePrice0, 10, 64)
		if err == nil && raw > 0 {
			result[p.ProductID] = raw
		}
	}

	return result, nil
}
