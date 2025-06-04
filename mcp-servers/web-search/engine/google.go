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

// https://developers.google.com/custom-search/v1/overview?hl=zh-cn
// https://programmablesearchengine.google.com/controlpanel/create
// https://zhuanlan.zhihu.com/p/174666017
type GoogleEngine struct {
	apiKey string
	cx     string
	client *http.Client
}

func NewGoogleEngine(apiKey, cx string) *GoogleEngine {
	return &GoogleEngine{
		apiKey: apiKey,
		cx:     cx,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type googleResponse struct {
	Items []googleResponseItem `json:"items"`
}

type googleResponseItem struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

func (g *GoogleEngine) Search(ctx context.Context, query SearchQuery) ([]SearchResult, error) {
	if query.MaxResults > 10 {
		query.MaxResults = 10 // Google API limit
	}

	querys := url.Values{}
	querys.Set("cx", g.cx)
	querys.Set("q", strings.Join(query.Queries, " "))
	querys.Set("num", strconv.Itoa(query.MaxResults))
	querys.Set("key", g.apiKey)

	if query.Language != "" {
		querys.Set("lr", "lang_"+query.Language)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://www.googleapis.com/customsearch/v1?"+querys.Encode(),
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google search failed: %d - %s", resp.StatusCode, string(body))
	}

	var response googleResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(response.Items))
	for _, item := range response.Items {
		results = append(results, SearchResult{
			Title:   item.Title,
			Link:    item.Link,
			Content: item.Snippet,
		})
	}

	return results, nil
}
