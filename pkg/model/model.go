package model

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0u3h1k/bare-metal/internal/config"
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

		// Try to load manifest for rich metadata
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
	// Try manifest first
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

// Load prepares a model for inference (placeholder for llama.cpp).
func (m *Manager) Load(name string) error {
	modelPath, err := m.GetModelPath(name)
	if err != nil {
		return err
	}
	fmt.Printf("🔧 Loading model '%s' from %s...\n", name, modelPath)
	fmt.Println("   (llama.cpp integration not yet implemented)")
	return nil
}

// Unload frees a model from memory (placeholder).
func (m *Manager) Unload(name string) error {
	fmt.Printf("📤 Unloading model '%s' from memory...\n", name)
	return nil
}
