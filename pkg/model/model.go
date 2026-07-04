package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/k0u3h1k/bare-metal/internal/config"
	"github.com/k0u3h1k/bare-metal/pkg/llama"
)

// Model represents a downloaded and cached model with metadata.
type Model struct {
	Name         string `json:"name"`
	RepoID       string `json:"repo_id"`
	Filename     string `json:"filename"`
	SizeBytes    int64  `json:"size_bytes"`
	SizeHuman    string `json:"size_human,omitempty"`
	Params       string `json:"params,omitempty"`
	Quantization string `json:"quantization,omitempty"`
	IsReady      bool   `json:"is_ready"`
}

// Manager handles model downloading, caching, lifecycle, and inference.
type Manager struct {
	cacheDir string
	binDir   string

	mu           sync.Mutex
	activeServer *llama.Server
	activeModel  string
}

// NewManager creates a new model manager.
func NewManager() *Manager {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".unbound")
	return &Manager{
		cacheDir: config.App.ModelDir,
		binDir:   filepath.Join(dataDir, "bin"),
	}
}

// List returns all cached models with metadata from their manifests.
func (m *Manager) List() ([]Model, error) {
	entries, err := os.ReadDir(m.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Model{}, nil
		}
		return nil, fmt.Errorf("reading model cache: %w", err)
	}

	var models []Model
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		manifest, err := LoadManifest(m.cacheDir, e.Name())
		if err == nil && manifest != nil {
			sizeHuman := fmt.Sprintf("%.1f MB", float64(manifest.SizeBytes)/(1024*1024))
			models = append(models, Model{
				Name:         manifest.Name,
				RepoID:       manifest.RepoID,
				Filename:     manifest.Filename,
				SizeBytes:    manifest.SizeBytes,
				SizeHuman:    sizeHuman,
				Params:       manifest.Params,
				Quantization: manifest.Quantization,
				IsReady:      manifest.IsReady,
			})
			continue
		}

		// Fallback: list files in the directory
		subEntries, _ := os.ReadDir(filepath.Join(m.cacheDir, e.Name()))
		var totalSize int64
		for _, s := range subEntries {
			if info, err := s.Info(); err == nil && !info.IsDir() {
				totalSize += info.Size()
			}
		}
		sizeHuman := fmt.Sprintf("%.1f MB", float64(totalSize)/(1024*1024))
		models = append(models, Model{
			Name:      e.Name(),
			SizeBytes: totalSize,
			SizeHuman: sizeHuman,
			IsReady:   true,
		})
	}
	return models, nil
}

// Pull downloads a model by name, resolving aliases and handling the full lifecycle.
func (m *Manager) Pull(input string) error {
	result, err := ResolveModel(input)
	if err != nil {
		return err
	}
	return m.DownloadFile(result)
}

// Remove deletes a cached model by name.
func (m *Manager) Remove(name string) error {
	// Try to load manifest for clean removal
	dir := filepath.Join(m.cacheDir, name)
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("model '%s' not found in cache", name)
		}
		return fmt.Errorf("accessing model: %w", err)
	}
	return os.RemoveAll(dir)
}

// GetModelPath returns the path to a cached model file.
// Returns the first .gguf file found in the model directory.
func (m *Manager) GetModelPath(name string) (string, error) {
	manifest, err := LoadManifest(m.cacheDir, name)
	if err == nil && manifest.IsReady {
		return manifest.ModelPath(m.cacheDir), nil
	}

	// Fallback: scan directory for .gguf files
	dir := filepath.Join(m.cacheDir, name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("model '%s' not found: %w", name, err)
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".gguf" {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no model file found in %s", dir)
}

// Load downloads a model if needed, then starts it via llama-server subprocess.
func (m *Manager) Load(name string, port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if this model is already loaded
	if m.activeModel == name && m.activeServer != nil && m.activeServer.IsRunning() {
		fmt.Printf("✅ Model '%s' is already loaded.\n", name)
		return nil
	}

	// Unload any existing model
	if m.activeServer != nil {
		if err := m.activeServer.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error stopping previous model: %v\n", err)
		}
		m.activeServer = nil
		m.activeModel = ""
	}

	// First, ensure the model is downloaded
	result, err := ResolveModel(name)
	if err == nil {
		// Check if already cached
		manifest, err := LoadManifest(m.cacheDir, result.Alias)
		if err != nil || !manifest.IsReady {
			fmt.Println("📥 Model not cached locally. Pulling first...")
			if err := m.DownloadFile(result); err != nil {
				return fmt.Errorf("downloading model: %w", err)
			}
		}
	}

	// Find the model file
	modelPath, err := m.GetModelPath(name)
	if err != nil {
		return fmt.Errorf("finding model: %w", err)
	}

	// Ensure llama-server binary is available
	server := llama.NewServer(modelPath)
	binaryPath, err := server.EnsureBinary(m.binDir)
	if err != nil {
		return fmt.Errorf("preparing llama.cpp: %w", err)
	}

	if port > 0 {
		server.Port = port
	} else {
		server.Port = 8080 // default
	}

	fmt.Printf("📂 Model file: %s\n", modelPath)

	if err := server.Start(binaryPath); err != nil {
		return fmt.Errorf("starting model: %w", err)
	}

	m.activeServer = server
	m.activeModel = name
	return nil
}

// Unload stops the currently running model.
func (m *Manager) Unload(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeServer == nil {
		fmt.Println("No model is currently loaded.")
		return nil
	}

	if m.activeModel != "" && m.activeModel != name {
		fmt.Printf("Different model loaded (%s). Stopping it...\n", m.activeModel)
	}

	if err := m.activeServer.Stop(); err != nil {
		return fmt.Errorf("unloading model: %w", err)
	}

	m.activeServer = nil
	m.activeModel = ""
	return nil
}

// GetServer returns the active llama server instance, if any.
func (m *Manager) GetServer() *llama.Server {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeServer
}

// GetInferenceURL returns the base URL for the running inference server.
func (m *Manager) GetInferenceURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.activeServer == nil {
		return ""
	}
	return fmt.Sprintf("http://%s:%d", m.activeServer.Host, m.activeServer.Port)
}
