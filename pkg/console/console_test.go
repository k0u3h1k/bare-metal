package console

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sseChunk builds a single OpenAI-compatible SSE data line for a content delta.
func sseChunk(content string) string {
	event := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"delta": map[string]interface{}{"content": content}},
		},
	}
	data, _ := json.Marshal(event)
	return fmt.Sprintf("data: %s\n\n", data)
}

// TestStreamCompletion_SSE verifies the streaming parser against a test server
// that emits OpenAI-compatible SSE chunks, and checks that stream=true and
// max_tokens are passed through in the request body.
func TestStreamCompletion_SSE(t *testing.T) {
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Errorf("request body is not valid JSON: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("response writer does not support flushing")
			return
		}
		for _, c := range []string{"Hello", ", ", "unbound", "!"} {
			fmt.Fprint(w, sseChunk(c))
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	var collected strings.Builder
	messages := []Message{
		{Role: "system", Content: "You are a test assistant."},
		{Role: "user", Content: "say hi"},
	}
	err := streamCompletion(srv.URL, messages, 128, func(token string) {
		collected.WriteString(token)
	})
	if err != nil {
		t.Fatalf("streamCompletion returned error: %v", err)
	}
	if got := collected.String(); got != "Hello, unbound!" {
		t.Errorf("streamed content = %q, want %q", got, "Hello, unbound!")
	}
	if gotBody["stream"] != true {
		t.Errorf("request body stream = %v, want true", gotBody["stream"])
	}
	if gotBody["max_tokens"] != float64(128) {
		t.Errorf("request body max_tokens = %v, want 128", gotBody["max_tokens"])
	}
}

// TestStreamCompletion_ServerError verifies that a non-200 response surfaces
// as an error instead of silently producing empty output.
func TestStreamCompletion_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := streamCompletion(srv.URL, []Message{{Role: "user", Content: "x"}}, 16, func(string) {})
	if err == nil {
		t.Fatal("expected error for HTTP 500 response")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error should mention HTTP status, got: %v", err)
	}
}

// TestStreamCompletion_MalformedData verifies that malformed SSE payloads
// surface as errors rather than being silently dropped.
func TestStreamCompletion_MalformedData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {not-valid-json\n\n")
	}))
	defer srv.Close()

	err := streamCompletion(srv.URL, []Message{{Role: "user", Content: "x"}}, 16, func(string) {})
	if err == nil {
		t.Fatal("expected error for malformed SSE data")
	}
	if !strings.Contains(err.Error(), "decoding SSE event") {
		t.Errorf("error should mention SSE decoding, got: %v", err)
	}
}

// TestDetectAndExecuteShellCommands_ExecutesShellBlock verifies that a
// ```bash block is executed with permission (via UNBOUND_ALLOW_ALL) and that
// surrounding non-block text is preserved.
func TestDetectAndExecuteShellCommands_ExecutesShellBlock(t *testing.T) {
	t.Setenv("UNBOUND_ALLOW_ALL", "1")

	var toolResults []string
	input := "Before the block.\n```bash\necho unbound-test-123\n```\nAfter the block."
	out := detectAndExecuteShellCommands(input, &toolResults, "http://127.0.0.1:9999", 16)

	if len(toolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(toolResults))
	}
	if !strings.Contains(toolResults[0], "unbound-test-123") {
		t.Errorf("tool result missing command output: %q", toolResults[0])
	}
	if !strings.Contains(out, "Before the block.") || !strings.Contains(out, "After the block.") {
		t.Errorf("non-block text was lost:\n%s", out)
	}
	if !strings.Contains(out, "_Executed: echo unbound-test-123_") {
		t.Errorf("shell block not replaced with summary:\n%s", out)
	}
}

// TestDetectAndExecuteShellCommands_NoBlockUntouched verifies that responses
// without shell blocks pass through unchanged and execute nothing.
func TestDetectAndExecuteShellCommands_NoBlockUntouched(t *testing.T) {
	t.Setenv("UNBOUND_ALLOW_ALL", "1")

	var toolResults []string
	input := "just plain text\nsecond line"
	out := detectAndExecuteShellCommands(input, &toolResults, "http://127.0.0.1:9999", 16)

	if len(toolResults) != 0 {
		t.Errorf("no shell blocks should not execute anything, got %d results", len(toolResults))
	}
	if out != input {
		t.Errorf("response without shell blocks should be unchanged, got:\n%s", out)
	}
}

// TestDetectAndExecuteShellCommands_UnclosedBlockPreserved verifies that a
// shell block which is never closed is NOT executed and its content is kept.
func TestDetectAndExecuteShellCommands_UnclosedBlockPreserved(t *testing.T) {
	t.Setenv("UNBOUND_ALLOW_ALL", "1")

	var toolResults []string
	input := "text\n```bash\necho never-run\n"
	out := detectAndExecuteShellCommands(input, &toolResults, "http://127.0.0.1:9999", 16)

	if len(toolResults) != 0 {
		t.Errorf("unclosed block should not be executed, got %d results", len(toolResults))
	}
	if !strings.Contains(out, "echo never-run") {
		t.Errorf("unclosed block content should be preserved, got:\n%s", out)
	}
}

// TestProcessToolCalls verifies the wrapper returns processed text and results.
func TestProcessToolCalls(t *testing.T) {
	t.Setenv("UNBOUND_ALLOW_ALL", "1")

	out, results := processToolCalls("a\n```sh\necho proc-tool-test\n```\nb", "http://127.0.0.1:9999", 16)
	if len(results) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(results))
	}
	if !strings.Contains(results[0], "proc-tool-test") {
		t.Errorf("tool result missing output: %q", results[0])
	}
	if strings.Contains(out, "```sh") {
		t.Errorf("shell block fence should be stripped:\n%s", out)
	}
}

// TestTruncate verifies the truncation helper used in block summaries.
func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(short) = %q, want unchanged", got)
	}
	long := strings.Repeat("a", 100)
	want := strings.Repeat("a", 7) + "..."
	if got := truncate(long, 10); got != want {
		t.Errorf("truncate(100 chars, 10) = %q, want %q", got, want)
	}
}
