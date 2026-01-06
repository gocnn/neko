package tool

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gocnn/neko"
)

// VisitWebpageTool fetches and returns webpage content.
type VisitWebpageTool struct {
	neko.BaseTool
	client    *http.Client
	maxLength int
}

// NewVisitWebpageTool creates a webpage visiting tool.
func NewVisitWebpageTool(maxLength int) *VisitWebpageTool {
	if maxLength <= 0 {
		maxLength = 50000
	}
	return &VisitWebpageTool{
		client:    &http.Client{Timeout: 30 * time.Second},
		maxLength: maxLength,
	}
}

func (t *VisitWebpageTool) Name() string        { return "visit_webpage" }
func (t *VisitWebpageTool) Description() string { return "Fetches content from a URL." }
func (t *VisitWebpageTool) OutputType() string  { return "string" }

func (t *VisitWebpageTool) Inputs() map[string]neko.ToolInput {
	return map[string]neko.ToolInput{
		"url": {Type: "string", Description: "URL to visit", Required: true},
	}
}

func (t *VisitWebpageTool) Execute(args map[string]any) (any, error) {
	urlStr, ok := args["url"].(string)
	if !ok || urlStr == "" {
		return nil, fmt.Errorf("url is required")
	}

	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "https://" + urlStr
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; neko-go/1.0)")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(t.maxLength)))
	if err != nil {
		return nil, err
	}

	// Basic HTML to text conversion (simplified)
	content := string(body)
	content = stripHTML(content)

	if len(content) > t.maxLength {
		content = content[:t.maxLength] + "... (truncated)"
	}

	return content, nil
}

func stripHTML(s string) string {
	// Simple HTML tag removal
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	// Collapse whitespace
	return strings.Join(strings.Fields(result.String()), " ")
}
