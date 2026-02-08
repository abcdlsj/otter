package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

const braveSearchAPI = "https://api.search.brave.com/res/v1/web/search"

type WebSearch struct{}

func (WebSearch) Name() string { return "websearch" }
func (WebSearch) Desc() string {
	return "Search the web using Brave Search API. Returns titles, URLs, and snippets for fast research."
}
func (WebSearch) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query string",
			},
			"count": map[string]any{
				"type":        "number",
				"description": "Number of results to return (1-20, default 10)",
			},
			"freshness": map[string]any{
				"type":        "string",
				"description": "Filter by time: 'pd' (past 24h), 'pw' (past week), 'pm' (past month), 'py' (past year)",
			},
			"country": map[string]any{
				"type":        "string",
				"description": "2-letter country code for region-specific results (e.g., 'US', 'CN', 'JP', 'DE')",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "ISO language code for search results (e.g., 'en', 'zh', 'ja', 'de')",
			},
		},
		"required": []string{"query"},
	}
}

func (w WebSearch) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		Query     string `json:"query"`
		Count     int    `json:"count"`
		Freshness string `json:"freshness"`
		Country   string `json:"country"`
		Language  string `json:"language"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}

	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Get API key from environment
	apiKey := w.getAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("BRAVE_API_KEY not set. Get one at https://brave.com/search/api/")
	}

	// Validate and normalize count
	if args.Count <= 0 || args.Count > 20 {
		args.Count = 10
	}

	// Build request URL
	u, err := url.Parse(braveSearchAPI)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("q", args.Query)
	q.Set("count", fmt.Sprintf("%d", args.Count))
	q.Set("offset", "0")
	if args.Freshness != "" {
		q.Set("freshness", args.Freshness)
	}
	if args.Country != "" {
		q.Set("country", args.Country)
	}
	if args.Language != "" {
		q.Set("search_lang", args.Language)
	}
	u.RawQuery = q.Encode()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search API returned %s", resp.Status)
	}

	// Parse response
	var result struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse search results: %w", err)
	}

	if len(result.Web.Results) == 0 {
		return "No results found for: " + args.Query, nil
	}

	// Format results
	output := fmt.Sprintf("Search results for: %s\n\n", args.Query)
	for i, r := range result.Web.Results {
		output += fmt.Sprintf("%d. %s\n   URL: %s\n   %s\n\n", i+1, r.Title, r.URL, r.Description)
	}

	return output, nil
}

func (w WebSearch) getAPIKey() string {
	// Check environment variable
	if key := os.Getenv("BRAVE_API_KEY"); key != "" {
		return key
	}
	// Also check without _KEY suffix for compatibility
	if key := os.Getenv("BRAVE_API"); key != "" {
		return key
	}
	return ""
}
