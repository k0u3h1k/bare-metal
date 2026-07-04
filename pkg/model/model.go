package model

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0u3h1k/bare-metal/internal/config"
)

// Model represents a downloaded and cached model.
type Model struct {
	Name    string
	Path    string
	Size    int64
	IsReady bool
}

// Manager handles model downloading, caching, and lifecycle.
type Manager struct {
	cacheDir string
}

// NewManager creates a new model manager.
func NewManager() *Manager {
	return &Manager{
		cacheDir: config.App.ModelDir,
	}
}

// List returns all cached models.
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
		info, err := e.Info()
		if err != nil {
			continue
		}
		models = append(models, Model{
			Name: e.Name(),
			Path: filepath.Join(m.cacheDir, e.Name()),
			Size: info.Size(),
		})
	}
	return models, nil
}

// Resolve maps a model alias or HuggingFace ID to a downloadable URL.
// This is a placeholder — real resolution will query Hugging Face API.
func Resolve(name string) (string, error) {
	// TODO: query Hugging Face API to resolve model ID -> GGUF file URL
	// For now, return the name as-is
	return name, nil
}

// Download pulls a model from Hugging Face Hub.
// Placeholder for actual implementation using huggingface hub client.
func (m *Manager) Download(name string) error {
	fmt.Printf("⏳ Downloading model '%s'...\n", name)
	fmt.Println("   (Model download not yet implemented)")

	// Ensure cache directory exists
	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	// TODO:
	// 1. Resolve model name to Hugging Face repo + file
	// 2. Stream download with progress bar
	// 3. Write to cache directory
	// 4. Verify file integrity

	return nil
}

// Remove deletes a cached model.
func (m *Manager) Remove(name string) error {
	modelPath := filepath.Join(m.cacheDir, name)
	if err := os.RemoveAll(modelPath); err != nil {
		return fmt.Errorf("removing model: %w", err)
	}
	return nil
}

// Load prepares a model for inference using llama.cpp.
// Placeholder for actual llama.cpp binding integration.
func (m *Manager) Load(name string) error {
	fmt.Printf("🔧 Loading model '%s' into memory...\n", name)
	fmt.Println("   (llama.cpp integration not yet implemented)")
	return nil
}

// Unload frees a model from memory.
func (m *Manager) Unload(name string) error {
	fmt.Printf("📤 Unloading model '%s' from memory...\n", name)
	// TODO: free llama.cpp context
	return nil
}
