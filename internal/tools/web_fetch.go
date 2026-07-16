package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

const (
	webFetchTimeout     = 30 * time.Second
	maxWebFetchBodySize = 128 * 1024 // 128KB
)

type WebFetchTool struct{}

func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{}
}

func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

func (t *WebFetchTool) Description() string {
	return `Fetch content from a URL and return it as text.

Use this through the tool API whenever you say you will fetch, open, read, check, or inspect a URL or public web content. Do not merely describe fetching web content in assistant text.

HTML pages are automatically converted to Markdown for readability. Other content
types (JSON, plain text, XML) are returned as-is.

Use this for: reading documentation, fetching API specs, checking URLs, reading
README files from GitHub, or any public web content.

Limitations:
- JavaScript-rendered pages (SPAs) will return the pre-JS skeleton, not the
  dynamically loaded content.
- Auth-gated pages will return a redirect or login page.
- Maximum response size is 128KB; larger responses are truncated.`
}

func (t *WebFetchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to fetch",
			},
		},
		"required":             []string{"url"},
		"additionalProperties": false,
	}
}

func (t *WebFetchTool) ValidateInput(_ context.Context, input any) error {
	params, ok := input.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid input: expected map[string]any, got %T", input)
	}
	url, ok := params["url"].(string)
	if !ok || url == "" {
		if _, exists := params["url"]; !exists {
			return missingRequiredParameter("web_fetch", "url", `{"url":"https://example.com"}`, "Provide the complete public URL to fetch")
		}
		return fmt.Errorf("invalid input: url must be a non-empty string")
	}
	return nil
}

func (t *WebFetchTool) Execute(ctx context.Context, input any) (any, error) {
	params := input.(map[string]any)
	url := params["url"].(string)

	client := &http.Client{Timeout: webFetchTimeout}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "keen-agent/0.1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxWebFetchBodySize+1)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	truncated := false
	if len(bodyBytes) > maxWebFetchBodySize {
		bodyBytes = bodyBytes[:maxWebFetchBodySize]
		truncated = true
	}

	content := string(bodyBytes)

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		if md, err := htmltomarkdown.ConvertString(content); err == nil {
			content = md
		}
	}

	if truncated {
		content += "\n... (content truncated)"
	}

	return map[string]any{
		"url":         url,
		"status_code": resp.StatusCode,
		"content":     content,
	}, nil
}
