package engine

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ArxivEngine struct {
	client *http.Client
}

func NewArxivEngine() *ArxivEngine {
	return &ArxivEngine{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type arxivFeed struct {
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	Title   string `xml:"title"`
	ID      string `xml:"id"`
	Summary string `xml:"summary"`
	Authors []struct {
		Name string `xml:"name"`
	} `xml:"author"`
	Published string `xml:"published"`
}

func (a *ArxivEngine) Search(ctx context.Context, query SearchQuery) ([]SearchResult, error) {
	searchQueryItems := make([]string, 0, len(query.Queries))
	for _, q := range query.Queries {
		searchQueryItems = append(searchQueryItems, "all:"+url.QueryEscape(q))
	}

	searchQuery := strings.Join(searchQueryItems, "+AND+")
	if query.ArxivCategory != "" {
		searchQuery = fmt.Sprintf("%s+AND+cat:%s", searchQuery, query.ArxivCategory)
	}

	searchURL := fmt.Sprintf(
		"https://export.arxiv.org/api/query?search_query=%s&max_results=%d",
		searchQuery,
		query.MaxResults,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("arxiv search failed: %d - %s", resp.StatusCode, string(body))
	}

	var feed arxivFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		authors := make([]string, 0, len(entry.Authors))
		for _, author := range entry.Authors {
			authors = append(authors, author.Name)
		}

		// Convert arxiv ID to URL
		arxivID := strings.TrimPrefix(entry.ID, "http://arxiv.org/abs/")
		link := "https://arxiv.org/abs/" + arxivID

		content := fmt.Sprintf("%s\nAuthors: %s\nPublished: %s",
			entry.Summary,
			strings.Join(authors, ", "),
			entry.Published,
		)

		results = append(results, SearchResult{
			Title:   entry.Title,
			Link:    link,
			Content: content,
		})
	}

	return results, nil
}
