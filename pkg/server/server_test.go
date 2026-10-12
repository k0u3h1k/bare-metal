package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatCompletionsProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"proxied","echo":` + string(body) + `}`))
	}))
	defer upstream.Close()

	api := httptest.NewServer(NewHandler("test-model", upstream.URL))
	defer api.Close()

	resp, err := http.Post(api.URL+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"id":"proxied"`) {
		t.Fatalf("unexpected proxied body: %s", body)
	}
}

func TestStreamingChatCompletionsProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("upstream does not support flushing")
		}
		_, _ = io.WriteString(w, "data: one\n\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "data: two\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	api := httptest.NewServer(NewHandler("test-model", upstream.URL))
	defer api.Close()

	resp, err := http.Post(api.URL+"/v1/chat/completions", "application/json",
		strings.NewReader(`{"stream":true,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content type = %q", got)
	}
	if string(body) != "data: one\n\ndata: two\n\n" {
		t.Fatalf("stream body = %q", body)
	}
}

func TestHealthAndModels(t *testing.T) {
	api := httptest.NewServer(NewHandler("test-model", "http://127.0.0.1:1"))
	defer api.Close()

	t.Run("health", func(t *testing.T) {
		resp, err := http.Get(api.URL + "/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var got map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK || got["status"] != "ok" || got["model"] != "test-model" {
			t.Fatalf("status = %d, body = %#v", resp.StatusCode, got)
		}
	})

	t.Run("models", func(t *testing.T) {
		resp, err := http.Get(api.URL + "/v1/models")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var got struct {
			Object string      `json:"object"`
			Data   []ModelInfo `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK || got.Object != "list" || len(got.Data) != 1 || got.Data[0].ID != "test-model" {
			t.Fatalf("status = %d, body = %#v", resp.StatusCode, got)
		}
	})
}

func TestChatCompletionsUpstreamDown(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	upstreamURL := upstream.URL
	upstream.Close()

	api := httptest.NewServer(NewHandler("test-model", upstreamURL))
	defer api.Close()
	resp, err := http.Post(api.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}
}
