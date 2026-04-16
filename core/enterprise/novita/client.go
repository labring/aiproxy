//go:build enterprise

package novita

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
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// DefaultNovitaAPIBase is the OpenAI-compatible endpoint used for the channel.
	DefaultNovitaAPIBase = "https://api.novita.ai/v3/openai"
	// DefaultNovitaAnthropicBase is the Anthropic-compatible endpoint.
	// Novita serves Anthropic protocol at /anthropic/v1 (not /v3/anthropic).
	DefaultNovitaAnthropicBase = "https://api.novita.ai/anthropic/v1"
	// novitaModelsEndpoint is the standard model listing endpoint (under the v3/openai base).
	novitaModelsEndpoint = "https://api.novita.ai/v3/openai/models"
	// novitaMgmtEndpoint is the management API endpoint providing full model catalog
	// with richer data (RPM/TPM, cache pricing, tiered billing).
	novitaMgmtEndpoint = "https://api-server.novita.ai/v1/product/model/list"
	// DefaultNovitaMultimodalBase is the base URL for Novita native multimodal channels.
	DefaultNovitaMultimodalBase = "https://api.novita.ai"
	// novitaMultimodalModelsEndpoint is the multimodal model catalog API.
	novitaMultimodalModelsEndpoint = "https://api-server.novita.ai/v1/product/multimodal-model/list"
	// novitaBatchPriceEndpoint is the SKU-based batch pricing API.
	novitaBatchPriceEndpoint = "https://api-server.novita.ai/v1/product/batch-price"
	// DefaultTimeout is the HTTP client timeout.
	defaultNovitaTimeout = 30 * time.Second
	// maxResponseSize caps the body we read from Novita API (50 MB).
	maxResponseSize = 50 << 20
)

// NovitaClient handles communication with Novita API.
type NovitaClient struct {
	APIKey  string
	APIBase string
	client  *http.Client
}

// NewNovitaClient creates a new Novita API client.
// Priority: database config (from selected channel) > environment variables.
func NewNovitaClient() (*NovitaClient, error) {
	cfg := GetNovitaConfig()
	apiKey := cfg.APIKey
	apiBase := cfg.APIBase

	if apiKey == "" {
		apiKey = os.Getenv("NOVITA_API_KEY")
	}

	if apiKey == "" {
		return nil, errors.New("Novita API Key is not configured. Please select a Novita channel in the Sync page or set NOVITA_API_KEY environment variable")
	}

	if apiBase == "" {
		apiBase = os.Getenv("NOVITA_API_BASE")
	}

	if apiBase == "" {
		apiBase = DefaultNovitaAPIBase
	}

	return &NovitaClient{
		APIKey:  apiKey,
		APIBase: apiBase,
		client: &http.Client{
			Timeout: defaultNovitaTimeout,
		},
	}, nil
}

// FetchModels fetches models from the standard Novita /v3/openai/models API.
func (c *NovitaClient) FetchModels(ctx context.Context) ([]NovitaModel, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultNovitaTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, novitaModelsEndpoint, nil)
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
		return nil, fmt.Errorf("Novita API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var modelsResp NovitaModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return modelsResp.Data, nil
}

// novitaMultimodalModelTypes lists the mgmt API model_type values for non-chat
// models that require separate requests (same approach as PPIO).
var novitaMultimodalModelTypes = []string{"embedding", "image", "video", "audio"}

// fetchMgmtModels calls the Novita management model-list API with the given
// query string and returns the parsed model slice.
func (c *NovitaClient) fetchMgmtModels(ctx context.Context, mgmtToken, query string) ([]NovitaModelV2, error) {
	url := novitaMgmtEndpoint + query

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+mgmtToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://novita.ai")
	req.Header.Set("Referer", "https://novita.ai/")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models from mgmt API: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("Novita mgmt API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var mgmtResp NovitaMgmtModelsResponse
	if err := json.Unmarshal(body, &mgmtResp); err != nil {
		return nil, fmt.Errorf("failed to parse mgmt response: %w", err)
	}

	return mgmtResp.Data, nil
}

// FetchAllModels fetches the full model catalog (including pa/ closed-source models)
// via the Novita management API using the mgmt console token.
//
// The default query only returns open-source models. ?visibility=1 includes
// closed-source pa/ models (e.g. pa/gpt-5.4, pa/claude-opus-4-6).
// Non-chat types (embedding, image, video, audio) require separate requests.
// All requests are issued concurrently to minimize total latency.
func (c *NovitaClient) FetchAllModels(ctx context.Context, mgmtToken string) ([]NovitaModelV2, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultNovitaTimeout)
	defer cancel()

	// Chat models (the bulk of the catalog, including pa/ closed-source).
	var chatModels []NovitaModelV2

	// Multimodal results collected under a mutex.
	var (
		mu          sync.Mutex
		extraModels []NovitaModelV2
	)

	g, gctx := errgroup.WithContext(ctx)

	// Fetch chat models (required — failure is fatal).
	g.Go(func() error {
		models, err := c.fetchMgmtModels(gctx, mgmtToken, "?visibility=1")
		if err != nil {
			return err
		}

		chatModels = models

		return nil
	})

	// Fetch each non-chat type concurrently (non-fatal on failure).
	for _, modelType := range novitaMultimodalModelTypes {
		g.Go(func() error {
			extra, extraErr := c.fetchMgmtModels(gctx, mgmtToken, "?model_type="+modelType)
			if extraErr != nil {
				log.Printf("Novita sync: failed to fetch %s models (non-fatal): %v", modelType, extraErr)
				return nil
			}

			mu.Lock()
			extraModels = append(extraModels, extra...)
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Merge: deduplicate multimodal models against chat models.
	seen := make(map[string]struct{}, len(chatModels))
	for _, m := range chatModels {
		seen[m.ID] = struct{}{}
	}

	for _, m := range extraModels {
		if _, exists := seen[m.ID]; !exists {
			seen[m.ID] = struct{}{}
			chatModels = append(chatModels, m)
		}
	}

	return chatModels, nil
}

// FetchAllModelsMerged fetches models from both V1 (public) and V2 (mgmt) APIs
// concurrently and merges them into a single V2 list. V2 wins on ID overlap.
// If mgmtToken is empty, only V1 models are returned (converted to V2 format).
func (c *NovitaClient) FetchAllModelsMerged(ctx context.Context, mgmtToken string) ([]NovitaModelV2, error) {
	var (
		v1Models []NovitaModel
		v2Models []NovitaModelV2
		v1Err    error
	)

	if mgmtToken != "" {
		// Fetch V1 and V2 concurrently.
		g, gctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			var err error
			v1Models, err = c.FetchModels(gctx)
			if err != nil {
				v1Err = err // non-fatal, recorded for logging below
			}

			return nil // always succeed — V1 failure is non-fatal when V2 is available
		})

		g.Go(func() error {
			var err error

			v2Models, err = c.FetchAllModels(gctx, mgmtToken)
			if err != nil {
				return fmt.Errorf("failed to fetch models from mgmt API: %w", err)
			}

			return nil
		})

		if err := g.Wait(); err != nil {
			return nil, err
		}

		if v1Err != nil {
			log.Printf("Novita sync: V1 API fetch failed (non-fatal, using V2 only): %v", v1Err)
			return v2Models, nil
		}
	} else {
		// No mgmt token — V1 is the only source
		v1Models, v1Err = c.FetchModels(ctx)
		if v1Err != nil {
			return nil, fmt.Errorf("failed to fetch models: %w", v1Err)
		}
	}

	// Merge: V2 wins on overlap (richer data with cache pricing, RPM/TPM)
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

// ── Multimodal API (api-server.novita.ai) ────────────────────────────────

// FetchMultimodalModels fetches all multimodal models (image/video/audio) from
// the Novita console API. Requires the mgmt console token for authentication.
func (c *NovitaClient) FetchMultimodalModels(ctx context.Context, mgmtToken string) ([]NovitaMultimodalModel, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultNovitaTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, novitaMultimodalModelsEndpoint, nil)
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var listResp NovitaMultimodalListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse multimodal response: %w", err)
	}

	return listResp.Configs, nil
}

// FetchMultimodalPrices fetches batch SKU pricing for the given SKU codes.
// Returns a map of skuCode → raw basePrice0 value (divide by multimodalPriceDivisor for USD/request).
func (c *NovitaClient) FetchMultimodalPrices(ctx context.Context, mgmtToken string, skuCodes []string) (map[string]int64, error) {
	if len(skuCodes) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, defaultNovitaTimeout)
	defer cancel()

	reqBody := NovitaBatchPriceRequest{
		BusinessType: "model_api",
		ProductIDs:   skuCodes,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch-price request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, novitaBatchPriceEndpoint, bytes.NewReader(bodyBytes))
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var priceResp NovitaBatchPriceResponse
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
