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

// Start launches the OpenAI-compatible API server.
// Proxies requests to the running llama-server instance.
func Start(modelName string, host string, port int, inferenceURL string) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	// If inference URL is not provided, default to local llama-server
	if inferenceURL == "" {
		inferenceURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "model": modelName})
	})

	// List models
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []ModelInfo{
				{ID: modelName, Object: "model", OwnedBy: "unbound"},
			},
		})
	})

	// Chat completions — proxy to llama-server.
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
		var req struct {
			Stream bool `json:"stream"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
			return
		}
		proxyURL := fmt.Sprintf("%s/v1/chat/completions", strings.TrimRight(inferenceURL, "/"))
		proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, proxyURL, bytes.NewReader(body))
		if err != nil {
			http.Error(w, fmt.Sprintf("proxy request: %v", err), http.StatusBadGateway)
			return
		}
		proxyReq.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 10 * time.Minute}
		resp, err := client.Do(proxyReq)
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
		if _, err := io.Copy(w, resp.Body); err != nil {
			return
		}
		_ = req.Stream // stream responses are forwarded unchanged, including SSE.
	})

	// Ollama-compatible tags endpoint
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"models": []ModelInfo{
				{ID: modelName, Object: "model", OwnedBy: "unbound"},
			},
		})
	})

	fmt.Printf("🌐 Unbound API server listening on %s\n", addr)
	fmt.Println("📋 Endpoints:")
	fmt.Println("   GET  /health                - Health check")
	fmt.Println("   GET  /v1/models             - List models")
	fmt.Println("   POST /v1/chat/completions   - Chat completion (proxied)")
	fmt.Println("   GET  /api/tags              - Ollama-compatible model list")
	fmt.Printf("📎 Proxying inference to: %s\n", inferenceURL)

	if err := http.ListenAndServe(addr, mux); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}
