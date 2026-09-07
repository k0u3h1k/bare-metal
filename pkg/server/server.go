package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatMessage represents a message in the OpenAI-compatible chat format.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest is the request body for /v1/chat/completions.
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float32       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatCompletionResponse is the response body for /v1/chat/completions.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a single completion choice.
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage tracks token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ModelInfo represents a model in the /v1/models response.
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// NewHandler builds the OpenAI-compatible API handler. It is separated from
// Start so callers and tests can mount the API without binding a fixed port.
func NewHandler(modelName, inferenceURL string) http.Handler {
	if inferenceURL == "" {
		inferenceURL = "http://127.0.0.1:8080"
	}
	inferenceURL = strings.TrimRight(inferenceURL, "/")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok", "model": modelName}); err != nil {
			return
		}
	})
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data":   []ModelInfo{{ID: modelName, Object: "model", OwnedBy: "unbound"}},
		})
	})
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"models": []ModelInfo{{ID: modelName, Object: "model", OwnedBy: "unbound"}},
		})
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("read request: %v", err), http.StatusBadRequest)
			return
		}
		var request struct {
			Stream bool `json:"stream"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
			return
		}
		proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
			inferenceURL+"/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			http.Error(w, fmt.Sprintf("proxy request: %v", err), http.StatusBadGateway)
			return
		}
		proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		if proxyReq.Header.Get("Content-Type") == "" {
			proxyReq.Header.Set("Content-Type", "application/json")
		}
		resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(proxyReq)
		if err != nil {
			http.Error(w, fmt.Sprintf("inference error: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for key, values := range resp.Header {
			if key == "Content-Length" {
				continue
			}
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)

		if !request.Stream {
			_, _ = io.Copy(w, resp.Body)
			return
		}
		controller := http.NewResponseController(w)
		// Flush headers before reading the first event so clients can begin
		// consuming an SSE response immediately.
		_ = controller.Flush()
		buffer := make([]byte, 32*1024)
		for {
			n, readErr := resp.Body.Read(buffer)
			if n > 0 {
				if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
					return
				}
				_ = controller.Flush()
			}
			if readErr != nil {
				return
			}
		}
	})
	return mux
}

// Start launches the OpenAI-compatible API server.
// Proxies requests to the running llama-server instance.
func Start(modelName string, host string, port int, inferenceURL string) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	if inferenceURL == "" {
		inferenceURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	fmt.Printf("🌐 Unbound API server listening on %s\n", addr)
	fmt.Println("📋 Endpoints:")
	fmt.Println("   GET  /health                - Health check")
	fmt.Println("   GET  /v1/models             - List models")
	fmt.Println("   POST /v1/chat/completions   - Chat completion (proxied)")
	fmt.Println("   GET  /api/tags              - Ollama-compatible model list")
	fmt.Printf("📎 Proxying inference to: %s\n", inferenceURL)
	if err := http.ListenAndServe(addr, NewHandler(modelName, inferenceURL)); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}
