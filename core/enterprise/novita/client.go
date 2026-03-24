//go:build enterprise

package novita

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
	// DefaultNovitaAPIBase is the OpenAI-compatible endpoint used for the channel.
	DefaultNovitaAPIBase = "https://api.novita.ai/v3/openai"
	// novitaModelsEndpoint is the standard model listing endpoint (under the v3/openai base).
	novitaModelsEndpoint = "https://api.novita.ai/v3/openai/models"
	// novitaMgmtEndpoint is the management API endpoint providing full model catalog
	// with richer data (RPM/TPM, cache pricing, tiered billing).
	novitaMgmtEndpoint = "https://api-server.novita.ai/v1/product/model/list"
	// DefaultTimeout is the HTTP client timeout.
	defaultNovitaTimeout = 30 * time.Second
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
func (c *NovitaClient) FetchModels() ([]NovitaModel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultNovitaTimeout)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Novita API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var modelsResp NovitaModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return modelsResp.Data, nil
}

// FetchAllModels fetches the full model catalog via the Novita management API.
// Requires a management token with extended access.
func (c *NovitaClient) FetchAllModels(mgmtToken string) ([]NovitaModelV2, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultNovitaTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, novitaMgmtEndpoint, nil)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Novita mgmt API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var mgmtResp NovitaMgmtModelsResponse
	if err := json.Unmarshal(body, &mgmtResp); err != nil {
		return nil, fmt.Errorf("failed to parse mgmt response: %w", err)
	}

	return mgmtResp.Data, nil
}

// FetchAllModelsMerged fetches models from both V1 (public) and V2 (mgmt) APIs
// and merges them into a single V2 list. V2 wins on ID overlap (richer data).
// If mgmtToken is empty, only V1 models are returned (converted to V2 format).
func (c *NovitaClient) FetchAllModelsMerged(mgmtToken string) ([]NovitaModelV2, error) {
	// Always fetch V1 (public API)
	v1Models, v1Err := c.FetchModels()

	var v2Models []NovitaModelV2

	if mgmtToken != "" {
		var v2Err error

		v2Models, v2Err = c.FetchAllModels(mgmtToken)
		if v2Err != nil {
			return nil, fmt.Errorf("failed to fetch models from mgmt API: %w", v2Err)
		}

		// V1 failure is non-fatal when we have V2 results
		if v1Err != nil {
			log.Printf("Novita sync: V1 API fetch failed (non-fatal, using V2 only): %v", v1Err)
			return v2Models, nil
		}
	} else {
		// No mgmt token — V1 is the only source
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
