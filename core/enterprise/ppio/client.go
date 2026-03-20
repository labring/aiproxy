//go:build enterprise

package ppio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
