package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type WebFetch struct{}

func (WebFetch) Name() string { return "webfetch" }
func (WebFetch) Desc() string {
	return "Fetch and extract readable content from a URL (HTML → markdown/text)"
}
func (WebFetch) Args() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "URL to fetch (HTTP or HTTPS)",
			},
			"maxChars": map[string]any{
				"type":        "number",
				"description": "Maximum characters to return (truncates when exceeded, default 50000)",
			},
		},
		"required": []string{"url"},
	}
}

func (w WebFetch) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var args struct {
		URL      string `json:"url"`
		MaxChars int    `json:"maxChars"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}

	if args.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	// Ensure URL has scheme
	if !strings.HasPrefix(args.URL, "http://") && !strings.HasPrefix(args.URL, "https://") {
		args.URL = "https://" + args.URL
	}

	if args.MaxChars <= 0 {
		args.MaxChars = 50000
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set a browser-like User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read body with limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(args.MaxChars)+1024))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	content := string(body)

	// Simple HTML to text extraction
	content = extractText(content)

	// Apply maxChars limit
	if len(content) > args.MaxChars {
		content = content[:args.MaxChars] + "\n\n[Content truncated. Use maxChars parameter to get more.]"
	}

	return content, nil
}

// extractText performs simple HTML to text extraction
func extractText(html string) string {
	// Remove script and style tags with their content
	html = removeTag(html, "script")
	html = removeTag(html, "style")
	html = removeTag(html, "noscript")

	// Replace common block elements with newlines
	replacements := map[string]string{
		"</p>":   "\n\n",
		"<br>":   "\n",
		"<br/>":  "\n",
		"<br />": "\n",
		"</div>": "\n",
		"</h1>":  "\n\n",
		"</h2>":  "\n\n",
		"</h3>":  "\n\n",
		"</h4>":  "\n\n",
		"</h5>":  "\n\n",
		"</h6>":  "\n\n",
		"</li>":  "\n",
	}

	for tag, replacement := range replacements {
		html = strings.ReplaceAll(html, tag, replacement)
	}

	// Replace opening headers with the tag + space
	for i := 1; i <= 6; i++ {
		hTag := fmt.Sprintf("<h%d", i)
		html = strings.ReplaceAll(html, hTag, "\n\n# ")
	}

	// Simple tag stripping - remove remaining tags
	var result strings.Builder
	inTag := false
	for _, r := range html {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(r)
			}
		}
	}

	text := result.String()

	// Normalize whitespace
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.ReplaceAll(text, "  ", " ")

	// Remove excessive newlines
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	// Clean up HTML entities
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	return strings.TrimSpace(text)
}

// removeTag removes HTML tags and their content
func removeTag(html, tag string) string {
	startTag := "<" + tag
	endTag := "</" + tag + ">"

	for {
		startIdx := strings.Index(strings.ToLower(html), startTag)
		if startIdx == -1 {
			break
		}

		// Find where the opening tag ends
		tagEnd := strings.Index(html[startIdx:], ">")
		if tagEnd == -1 {
			break
		}
		tagEnd += startIdx + 1

		// Find the closing tag
		endIdx := strings.Index(strings.ToLower(html[tagEnd:]), endTag)
		if endIdx == -1 {
			// No closing tag, just remove from start to tag end
			html = html[:startIdx] + html[tagEnd:]
			continue
		}
		endIdx += tagEnd

		html = html[:startIdx] + html[endIdx+len(endTag):]
	}

	return html
}
