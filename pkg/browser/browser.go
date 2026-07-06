// Package browser provides a lightweight text-based web browser for model use.
// It fetches URLs via HTTP, parses HTML, and extracts readable text content.
// All browsing requests are gated through the permission system.
package browser

import (
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
    "time"

    "golang.org/x/net/html"
)

const (
    // defaultTimeout is the HTTP request timeout.
    defaultTimeout = 30 * time.Second
    // maxBodySize is the maximum response body to read (10 MB).
    maxBodySize = 10 * 1024 * 1024
    // maxTextLength is the maximum extracted text length to return.
    maxTextLength = 50 * 1024 // 50 KB of readable text
    // userAgent is the browser user agent.
    userAgent = "Unbound/0.1.0 (local AI agent; https://github.com/k0u3h1k/bare-metal)"
)

// Result holds the fetched page content.
type Result struct {
    URL        string `json:"url"`
    Title      string `json:"title"`
    Text       string `json:"text"`
    StatusCode int    `json:"status_code"`
    ContentLen int64  `json:"content_length"`
    Truncated  bool   `json:"truncated"`
    Error      string `json:"error,omitempty"`
}

// Fetch downloads a URL and extracts readable text content.
// Returns an error if permission is denied or the request fails.
func Fetch(targetURL string) (*Result, error) {
    // Validate and clean the URL
    parsedURL, err := url.Parse(targetURL)
    if err != nil {
        return nil, fmt.Errorf("invalid URL '%s': %w", targetURL, err)
    }

    // Default to https if no scheme
    if parsedURL.Scheme == "" {
        parsedURL.Scheme = "https"
        targetURL = parsedURL.String()
    }

    // Only allow http and https
    if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
        return nil, fmt.Errorf("unsupported URL scheme '%s' (only http/https allowed)", parsedURL.Scheme)
    }

    // Make the HTTP request
    client := &http.Client{
        Timeout: defaultTimeout,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            if len(via) > 10 {
                return fmt.Errorf("too many redirects")
            }
            return nil
        },
    }

    req, err := http.NewRequest("GET", targetURL, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }
    req.Header.Set("User-Agent", userAgent)
    req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
    req.Header.Set("Accept-Language", "en-US,en;q=0.5")

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetching %s: %w", targetURL, err)
    }
    defer resp.Body.Close()

    // Read body (limited)
    limitedReader := io.LimitReader(resp.Body, maxBodySize)
    body, err := io.ReadAll(limitedReader)
    if err != nil {
        return nil, fmt.Errorf("reading response: %w", err)
    }

    contentType := resp.Header.Get("Content-Type")

    result := &Result{
        URL:        resp.Request.URL.String(), // final URL after redirects
        StatusCode: resp.StatusCode,
        ContentLen: int64(len(body)),
    }

    // Extract text content from HTML
    if strings.Contains(contentType, "text/html") {
        title, text := extractHTML(string(body))
        result.Title = title
        result.Text = text
    } else if strings.Contains(contentType, "text/plain") {
        result.Text = string(body)
    } else {
        // For non-text content, just return the URL and content type
        result.Text = fmt.Sprintf("[%s content — %d bytes, not displayed as text]", contentType, len(body))
    }

    // Truncate if needed
    if len(result.Text) > maxTextLength {
        result.Text = result.Text[:maxTextLength]
        result.Truncated = true
    }

    // Clean up whitespace
    result.Text = strings.TrimSpace(result.Text)

    return result, nil
}

// extractHTML parses HTML and returns the page title and readable text content.
func extractHTML(htmlContent string) (title string, text string) {
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        return "", "Error parsing HTML: " + err.Error()
    }

    var titleFound string
    var textParts []string

    // Blacklist of tags whose content we skip
    skipTags := map[string]bool{
        "script": true, "style": true, "noscript": true,
        "svg": true, "iframe": true, "canvas": true,
        "nav": true, "footer": true, "header": true,
    }

    var extract func(*html.Node, bool)
    extract = func(n *html.Node, skip bool) {
        if n.Type == html.ElementNode {
            tag := strings.ToLower(n.Data)
            if skipTags[tag] {
                skip = true
            }
            if tag == "title" {
                for c := n.FirstChild; c != nil; c = c.NextSibling {
                    if c.Type == html.TextNode {
                        titleFound = strings.TrimSpace(c.Data)
                    }
                }
            }
            // Add newlines for block elements
            if tag == "p" || tag == "div" || tag == "br" || tag == "li" ||
                tag == "h1" || tag == "h2" || tag == "h3" || tag == "h4" ||
                tag == "h5" || tag == "h6" || tag == "tr" || tag == "blockquote" {
                textParts = append(textParts, "\n")
            }
            if tag == "a" {
                // Also extract href for links
                for _, attr := range n.Attr {
                    if attr.Key == "href" {
                        textParts = append(textParts, " ["+attr.Val+"]")
                    }
                }
            }
        }

        if n.Type == html.TextNode && !skip {
            text := strings.TrimSpace(n.Data)
            if text != "" {
                textParts = append(textParts, text)
            }
        }

        for c := n.FirstChild; c != nil; c = c.NextSibling {
            extract(c, skip)
        }
    }

    extract(doc, false)

    // Join and clean up
    rawText := strings.Join(textParts, " ")
    rawText = strings.Join(strings.Fields(rawText), " ")
    rawText = strings.ReplaceAll(rawText, "\n ", "\n")

    return titleFound, rawText
}

// CanBrowse checks if a URL is likely browseable (text-based).
func CanBrowse(targetURL string) bool {
    if targetURL == "" {
        return false
    }
    parsed, err := url.Parse(targetURL)
    if err != nil {
        return false
    }
    scheme := parsed.Scheme
    if scheme == "" {
        scheme = "https"
    }
    return scheme == "http" || scheme == "https"
}

// DescribeContent returns a human-readable summary of what was fetched.
func (r *Result) DescribeContent() string {
    parts := []string{fmt.Sprintf("Status: %d", r.StatusCode)}
    if r.Title != "" {
        parts = append(parts, fmt.Sprintf("Title: %s", r.Title))
    }
    textLen := len(r.Text)
    if textLen > 0 {
        parts = append(parts, fmt.Sprintf("Text: %d chars", textLen))
    }
    if r.Truncated {
        parts = append(parts, "(truncated)")
    }
    return strings.Join(parts, " | ")
}
