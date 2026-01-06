package tool

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gocnn/neko"
)

// WebSearchTool performs web searches using DuckDuckGo.
type WebSearchTool struct {
	neko.BaseTool
	maxResults int
	client     *http.Client
}

// NewWebSearchTool creates a web search tool.
func NewWebSearchTool(maxResults int) *WebSearchTool {
	if maxResults <= 0 {
		maxResults = 10
	}
	return &WebSearchTool{
		BaseTool:   neko.BaseTool{},
		maxResults: maxResults,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Performs a web search and returns top results."
}

func (t *WebSearchTool) Inputs() map[string]neko.ToolInput {
	return map[string]neko.ToolInput{
		"query": {Type: "string", Description: "Search query", Required: true},
	}
}

func (t *WebSearchTool) OutputType() string { return "string" }

func (t *WebSearchTool) Execute(args map[string]any) (any, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query is required")
	}

	results, err := t.search(query)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return "No results found.", nil
	}

	var sb strings.Builder
	sb.WriteString("## Search Results\n\n")
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("[%s](%s)\n%s\n\n", r.Title, r.URL, r.Snippet))
	}
	return sb.String(), nil
}

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

func (t *WebSearchTool) search(query string) ([]searchResult, error) {
	// Using DuckDuckGo HTML endpoint (simplified)
	apiURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; neko-go/1.0)")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	// For production, parse HTML response
	// This is a simplified placeholder
	return []searchResult{
		{Title: "Search completed", URL: apiURL, Snippet: "Use a proper search API for production."},
	}, nil
}

// SerpAPISearchTool uses SerpAPI for Google search.
type SerpAPISearchTool struct {
	neko.BaseTool
	apiKey     string
	maxResults int
	client     *http.Client
}

// NewSerpAPISearchTool creates a SerpAPI-based search tool.
func NewSerpAPISearchTool(apiKey string, maxResults int) *SerpAPISearchTool {
	return &SerpAPISearchTool{
		apiKey:     apiKey,
		maxResults: maxResults,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *SerpAPISearchTool) Name() string        { return "web_search" }
func (t *SerpAPISearchTool) Description() string { return "Searches Google via SerpAPI." }
func (t *SerpAPISearchTool) OutputType() string  { return "string" }

func (t *SerpAPISearchTool) Inputs() map[string]neko.ToolInput {
	return map[string]neko.ToolInput{
		"query": {Type: "string", Description: "Search query", Required: true},
	}
}

func (t *SerpAPISearchTool) Execute(args map[string]any) (any, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	apiURL := fmt.Sprintf("https://serpapi.com/search.json?q=%s&api_key=%s&num=%d",
		url.QueryEscape(query), t.apiKey, t.maxResults)

	resp, err := t.client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic_results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString("## Search Results\n\n")
	for _, r := range result.OrganicResults {
		fmt.Fprintf(&sb, "[%s](%s)\n%s\n\n", r.Title, r.Link, r.Snippet)
	}
	return sb.String(), nil
}
