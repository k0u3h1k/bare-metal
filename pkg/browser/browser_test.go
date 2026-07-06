package browser

import (
	"strings"
	"testing"
)

func TestExtractHTML(t *testing.T) {
	html := `<html><head><title>Test Page</title></head><body>
		<h1>Hello World</h1>
		<p>This is a paragraph with <a href="https://example.com">a link</a>.</p>
		<script>alert("skip me");</script>
		<style>.skip-me{}</style>
	</body></html>`

	title, text := extractHTML(html)

	if title != "Test Page" {
		t.Errorf("title = %q, want %q", title, "Test Page")
	}

	if !strings.Contains(text, "Hello World") {
		t.Errorf("text should contain 'Hello World', got: %s", text)
	}
	if !strings.Contains(text, "This is a paragraph") {
		t.Errorf("text should contain 'This is a paragraph', got: %s", text)
	}
	if !strings.Contains(text, "https://example.com") {
		t.Errorf("text should contain the link URL, got: %s", text)
	}
	if strings.Contains(text, "skip me") {
		t.Errorf("text should NOT contain script content, got: %s", text)
	}
	if strings.Contains(text, ".skip-me") {
		t.Errorf("text should NOT contain style content, got: %s", text)
	}
}

func TestExtractHTML_NestedElements(t *testing.T) {
	html := `<html><body>
		<ul>
			<li>Item 1</li>
			<li>Item 2</li>
		</ul>
		<p>After list</p>
	</body></html>`

	_, text := extractHTML(html)

	if !strings.Contains(text, "Item 1") {
		t.Errorf("text should contain 'Item 1', got: %s", text)
	}
	if !strings.Contains(text, "After list") {
		t.Errorf("text should contain 'After list', got: %s", text)
	}
}

func TestExtractHTML_NoContent(t *testing.T) {
	_, text := extractHTML("<html></html>")
	if text != "" {
		t.Errorf("expected empty text, got: %s", text)
	}
}

func TestExtractHTML_NoTitle(t *testing.T) {
	html := `<html><body><p>Just content</p></body></html>`
	title, text := extractHTML(html)
	if title != "" {
		t.Errorf("expected empty title, got %q", title)
	}
	if !strings.Contains(text, "Just content") {
		t.Errorf("text should contain content, got: %s", text)
	}
}

func TestCanBrowse(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://example.com", true},
		{"http://example.com", true},
		{"ftp://example.com", false},
		{"file:///etc/passwd", false},
		{"javascript:alert(1)", false},
		{"", false},
		{"not-a-url", true}, // Will be defaulted to https
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := CanBrowse(tt.url)
			if got != tt.want {
				t.Errorf("CanBrowse(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestDescribeContent(t *testing.T) {
	r := &Result{
		URL:        "https://example.com",
		Title:      "Example",
		Text:       "Hello world.",
		StatusCode: 200,
	}
	desc := r.DescribeContent()
	if !strings.Contains(desc, "200") {
		t.Errorf("description should contain status code, got: %s", desc)
	}
	if !strings.Contains(desc, "Example") {
		t.Errorf("description should contain title, got: %s", desc)
	}
}

func TestResult_Truncated(t *testing.T) {
	r := &Result{
		URL:       "https://example.com",
		Text:      "Some content",
		Truncated: true,
	}
	desc := r.DescribeContent()
	if !strings.Contains(desc, "truncated") {
		t.Errorf("description should indicate truncation, got: %s", desc)
	}
}
