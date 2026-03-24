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
	DefaultPPIOAPIBase      = "https://api.ppio.com/v1"
	ppioModelsEndpoint      = "https://api.ppio.com/openai/models"
	ppioMgmtModelsEndpoint  = "https://api-server.ppio.com/v1/product/model/list"
	DefaultTimeout          = 30 * time.Second
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
			Timeout: DefaultTimeout,
		},
	}, nil
}

// FetchModels fetches all models from PPIO API.
// Always uses DefaultPPIOAPIBase (/v1) for the models endpoint,
// since the channel's base URL may point to /openai or /anthropic.
func (c *PPIOClient) FetchModels() ([]PPIOModel, error) {
	url := ppioModelsEndpoint

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PPIO API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var modelsResp PPIOModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return modelsResp.Data, nil
}

// FetchAllModels fetches the full model catalog (including pa/ closed-source models)
// via the PPIO management API using the mgmt console token.
func (c *PPIOClient) FetchAllModels(mgmtToken string) ([]PPIOModelV2, error) {
	url := ppioMgmtModelsEndpoint + "?visibility=1"

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("PPIO mgmt API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
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

// FetchAllModelsMerged fetches models from both V1 (public) and V2 (mgmt) APIs
// and merges them into a single V2 list. V2 wins on ID overlap (richer data).
// If mgmtToken is empty, only V1 models are returned (converted to V2 format).
func (c *PPIOClient) FetchAllModelsMerged(mgmtToken string) ([]PPIOModelV2, error) {
	// Always fetch V1 (public API)
	v1Models, v1Err := c.FetchModels()

	var v2Models []PPIOModelV2

	if mgmtToken != "" {
		var v2Err error

		v2Models, v2Err = c.FetchAllModels(mgmtToken)
		if v2Err != nil {
			return nil, fmt.Errorf("failed to fetch models from mgmt API: %w", v2Err)
		}

		// V1 failure is non-fatal when we have V2 results
		if v1Err != nil {
			// Log but continue with V2 only
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
