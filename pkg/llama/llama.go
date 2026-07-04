// Package llama manages the llama.cpp subprocess for model inference.
// Uses pre-built llama-server binaries downloaded from GitHub releases.
// This keeps the Unbound Go binary lightweight by delegating heavy C++
// inference to a separate process.
package llama

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// llamaReleaseTag is the pinned llama.cpp release tag.
	llamaReleaseTag = "b9870"
	// llamaRepo is the llama.cpp GitHub repository.
	llamaRepo = "ggml-org/llama.cpp"
	// healthCheckTimeout is how long to wait for the server to start.
	healthCheckTimeout = 30 * time.Second
	// healthCheckInterval is how often to poll for readiness.
	healthCheckInterval = 500 * time.Millisecond
)

// Server manages a llama.cpp server subprocess.
type Server struct {
	ModelPath string
	Port      int
	Host      string
	Threads   int
	GPULayers int
	CtxSize   int

	cmd      *exec.Cmd
	binDir   string
	isOwnBin bool // whether we downloaded the binary ourselves
}

// NewServer creates a new llama.cpp server manager.
func NewServer(modelPath string) *Server {
	return &Server{
		ModelPath: modelPath,
		Port:      0, // auto-assign
		Host:      "127.0.0.1",
		Threads:   0, // auto-detect
		GPULayers: 0, // CPU only
		CtxSize:   4096,
	}
}

// EnsureBinary downloads the llama-server binary if not already cached.
// Returns the path to the binary.
func (s *Server) EnsureBinary(binDir string) (string, error) {
	s.binDir = binDir

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("creating bin directory: %w", err)
	}

	binaryPath := filepath.Join(binDir, "llama-server")
	if s.isOwnBin {
		binaryPath = filepath.Join(binDir, "llama-server-"+llamaReleaseTag)
	}

	// Check if binary already exists
	if _, err := os.Stat(binaryPath); err == nil {
		// Verify it's executable
		if err := os.Chmod(binaryPath, 0755); err == nil {
			return binaryPath, nil
		}
	}

	// Binary not found — download it
	fmt.Println("📥 Downloading llama.cpp server binary...")
	assetName := s.assetName()
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", llamaRepo, llamaReleaseTag, assetName)
	downloadPath := filepath.Join(binDir, assetName)

	if err := s.downloadFile(downloadURL, downloadPath); err != nil {
		return "", fmt.Errorf("downloading llama.cpp: %w", err)
	}
	defer os.Remove(downloadPath)

	fmt.Println("📦 Extracting llama.cpp server binary...")
	if err := s.extractTarGz(downloadPath, binDir); err != nil {
		return "", fmt.Errorf("extracting llama.cpp: %w", err)
	}

	// The binary inside the tarball is named "llama-server"
	extractedPath := filepath.Join(binDir, "llama-server")
	if err := os.Chmod(extractedPath, 0755); err != nil {
		return "", fmt.Errorf("making binary executable: %w", err)
	}

	// Rename to versioned name for clarity
	versionedPath := filepath.Join(binDir, "llama-server-"+llamaReleaseTag)
	if err := os.Rename(extractedPath, versionedPath); err == nil {
		binaryPath = versionedPath
		// Symlink the generic name
		os.Symlink(versionedPath, filepath.Join(binDir, "llama-server"))
	}

	fmt.Printf("✅ llama.cpp server ready: %s\n", binaryPath)
	return binaryPath, nil
}

// Start launches the llama-server process and waits for it to be ready.
func (s *Server) Start(binaryPath string) error {
	args := []string{
		"-m", s.ModelPath,
		"--host", s.Host,
		"--port", fmt.Sprintf("%d", s.Port),
		"-c", fmt.Sprintf("%d", s.CtxSize),
		"--no-mmap", // More portable
	}

	if s.Threads > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", s.Threads))
	}
	if s.GPULayers > 0 {
		args = append(args, "-ngl", fmt.Sprintf("%d", s.GPULayers))
	}

	// Also support flash attention if available
	args = append(args, "--flash-attn")

	fmt.Printf("🔧 Starting llama-server with %s...\n", filepath.Base(s.ModelPath))
	s.cmd = exec.Command(binaryPath, args...)

	// Capture stdout/stderr
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("starting llama-server: %w", err)
	}

	// Wait for server to be ready
	baseURL := fmt.Sprintf("http://%s:%d", s.Host, s.Port)
	deadline := time.Now().Add(healthCheckTimeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Printf("✅ Model loaded and ready at %s\n", baseURL)
				return nil
			}
		}
		time.Sleep(healthCheckInterval)
	}

	// Server didn't start in time — kill it
	s.Stop()
	return fmt.Errorf("llama-server failed to start within %s (check model file compatibility)", healthCheckTimeout)
}

// Stop gracefully shuts down the llama-server process.
func (s *Server) Stop() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}

	// Try graceful shutdown via API first
	baseURL := fmt.Sprintf("http://%s:%d", s.Host, s.Port)
	resp, err := http.Post(baseURL+"/shutdown", "application/json", nil)
	if err == nil {
		resp.Body.Close()
		// Give it a moment to shut down
		time.Sleep(1 * time.Second)
	}

	// Force kill if still running
	if s.cmd.ProcessState == nil || !s.cmd.ProcessState.Exited() {
		if err := s.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("killing llama-server: %w", err)
		}
	}

	// Wait for process to exit
	s.cmd.Wait()
	fmt.Println("🛑 llama-server stopped.")
	return nil
}

// IsRunning checks if the llama-server process is still alive.
func (s *Server) IsRunning() bool {
	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}
	// Check if process has exited
	return s.cmd.ProcessState == nil || !s.cmd.ProcessState.Exited()
}

// Health checks if the server responds.
func (s *Server) Health() bool {
	baseURL := fmt.Sprintf("http://%s:%d", s.Host, s.Port)
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Infer sends a chat completion request and returns the response.
func (s *Server) Infer(messages []map[string]string, maxTokens int, temperature float32) (string, error) {
	baseURL := fmt.Sprintf("http://%s:%d", s.Host, s.Port)

	body := map[string]interface{}{
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"stream":      false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	resp, err := http.Post(baseURL+"/v1/chat/completions", "application/json", strings.NewReader(string(jsonBody)))
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}

// InferStream sends a streaming chat completion request.
// Returns a channel of text chunks and an error channel.
func (s *Server) InferStream(messages []map[string]string, maxTokens int, temperature float32) (<-chan string, <-chan error) {
	textChan := make(chan string, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(textChan)
		defer close(errChan)

		baseURL := fmt.Sprintf("http://%s:%d", s.Host, s.Port)

		body := map[string]interface{}{
			"messages":    messages,
			"max_tokens":  maxTokens,
			"temperature": temperature,
			"stream":      true,
		}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			errChan <- fmt.Errorf("marshaling request: %w", err)
			return
		}

		resp, err := http.Post(baseURL+"/v1/chat/completions", "application/json", strings.NewReader(string(jsonBody)))
		if err != nil {
			errChan <- fmt.Errorf("sending request: %w", err)
			return
		}
		defer resp.Body.Close()

		dec := json.NewDecoder(resp.Body)
		for {
			var event struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := dec.Decode(&event); err != nil {
				if err == io.EOF {
					break
				}
				errChan <- fmt.Errorf("decoding stream: %w", err)
				return
			}

			if len(event.Choices) > 0 {
				textChan <- event.Choices[0].Delta.Content
				if event.Choices[0].FinishReason != nil {
					break
				}
			}
		}
	}()

	return textChan, errChan
}

// assetName returns the expected filename for the platform's pre-built binary.
func (s *Server) assetName() string {
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return fmt.Sprintf("llama-%s-bin-ubuntu-x64.tar.gz", llamaReleaseTag)
		case "arm64":
			return fmt.Sprintf("llama-%s-bin-ubuntu-arm64.tar.gz", llamaReleaseTag)
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			return fmt.Sprintf("llama-%s-bin-macos-x64.tar.gz", llamaReleaseTag)
		case "arm64":
			return fmt.Sprintf("llama-%s-bin-macos-arm64.tar.gz", llamaReleaseTag)
		}
	}
	// Fallback for unsupported platforms
	return fmt.Sprintf("llama-%s-bin-ubuntu-x64.tar.gz", llamaReleaseTag)
}

// downloadFile downloads a URL to a local file.
func (s *Server) downloadFile(url, path string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d downloading %s", resp.StatusCode, url)
	}

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	sizeMB := float64(written) / (1024 * 1024)
	fmt.Printf("   Downloaded %.1f MB\n", sizeMB)
	return nil
}

// extractTarGz extracts a tar.gz archive.
func (s *Server) extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		// Only extract the llama-server binary
		if header.Name != "llama-server" && !strings.HasSuffix(header.Name, "/llama-server") {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			writer, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(writer, tr); err != nil {
				writer.Close()
				return err
			}
			writer.Close()
		}
	}

	return nil
}
