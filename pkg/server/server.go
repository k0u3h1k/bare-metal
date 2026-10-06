package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/k0u3h1k/bare-metal/pkg/model"
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

	// Chat completions — proxy to llama-server
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
			return
		}

		// Check if inference is running
		mgr := model.NewManager()
		server := mgr.GetServer()
		if server == nil || !server.IsRunning() {
			http.Error(w, `{"error":"no model loaded. Run 'unbound run <model>' first."}`, http.StatusServiceUnavailable)
			return
		}

		// Build proxy request to llama-server
		proxyURL := fmt.Sprintf("%s/v1/chat/completions", inferenceURL)
		proxyBody, err := json.Marshal(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("encode request: %v", err), http.StatusInternalServerError)
			return
		}

		if req.Stream {
			// Streaming: use SSE
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			// We use a direct HTTP client to forward streaming
			client := &http.Client{}
			proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, proxyURL, strings.NewReader(string(proxyBody)))
			if err != nil {
				http.Error(w, fmt.Sprintf("proxy request: %v", err), http.StatusBadGateway)
				return
			}
			proxyReq.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(proxyReq)
			if err != nil {
				http.Error(w, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			// Forward the SSE stream
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming not supported", http.StatusInternalServerError)
				return
			}

			if _, err := io.Copy(w, resp.Body); err != nil {
				return
			}
			flusher.Flush()
			return
		}

		// Non-streaming: proxy and return response
		resp, err := http.Post(proxyURL, "application/json", strings.NewReader(string(proxyBody)))
		if err != nil {
			http.Error(w, fmt.Sprintf("inference error: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")

		// Check if llama-server returned an error
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			w.WriteHeader(resp.StatusCode)
			w.Write(body)
			return
		}

		if _, err := io.Copy(w, resp.Body); err != nil {
			return
		}
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
