package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
)

type BingEngine struct {
	apiKey string
	client *http.Client
}

func NewBingEngine(apiKey string) *BingEngine {
	return &BingEngine{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type bingResponse struct {
	WebPages bingResponseWebPages `json:"webPages"`
}

type bingResponseWebPages struct {
	Value []bingResponseWebPagesValue `json:"value"`
}

type bingResponseWebPagesValue struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func (b *BingEngine) Search(ctx context.Context, query SearchQuery) ([]SearchResult, error) {
	querys := url.Values{}
	querys.Set("q", strings.Join(query.Queries, " "))
	querys.Set("count", strconv.Itoa(query.MaxResults))
	if query.Language != "" {
		querys.Set("mkt", query.Language)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://api.bing.microsoft.com/v7.0/search?"+querys.Encode(),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", b.apiKey)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bing search failed: %d - %s", resp.StatusCode, string(body))
	}

	var response bingResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(response.WebPages.Value))
	for _, item := range response.WebPages.Value {
		results = append(results, SearchResult{
			Title:   item.Name,
			Link:    item.URL,
			Content: item.Snippet,
		})
	}

	return results, nil
}
