// Package manifest handles model metadata storage.
// Each cached model has a manifest.json file in its directory.
package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Manifest stores metadata about a downloaded model.
type Manifest struct {
	// Name is the user-facing model name (e.g. "llama3.2:1b").
	Name string `json:"name"`
	// RepoID is the Hugging Face repository ID (e.g. "meta-llama/Llama-3.2-1B-Instruct-GGUF").
	RepoID string `json:"repo_id"`
	// Filename is the specific file downloaded from the repo.
	Filename string `json:"filename"`
	// SizeBytes is the total file size in bytes.
	SizeBytes int64 `json:"size_bytes"`
	// SHA256 is the expected checksum (from HF metadata).
	SHA256 string `json:"sha256,omitempty"`
	// Quantization describes the model quantization (e.g. "Q4_K_M").
	Quantization string `json:"quantization,omitempty"`
	// Params is the parameter count (e.g. "7B", "1B").
	Params string `json:"params,omitempty"`
	// DownloadedAt is when the download completed.
	DownloadedAt time.Time `json:"downloaded_at"`
	// IsReady is true when the model is fully downloaded and verified.
	IsReady bool `json:"is_ready"`
}

// ManifestPath returns the path to a model's manifest file.
func ManifestPath(cacheDir, modelName string) string {
	return filepath.Join(cacheDir, modelName, "manifest.json")
}

// LoadManifest reads a manifest from disk.
func LoadManifest(cacheDir, modelName string) (*Manifest, error) {
	data, err := os.ReadFile(ManifestPath(cacheDir, modelName))
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	return &m, nil
}

// SaveManifest writes a manifest to disk.
func SaveManifest(cacheDir string, m *Manifest) error {
	dir := filepath.Join(cacheDir, m.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating model directory: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}
	if err := os.WriteFile(ManifestPath(cacheDir, m.Name), data, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	return nil
}

// ModelPath returns the path to the actual model file.
func (m *Manifest) ModelPath(cacheDir string) string {
	return filepath.Join(cacheDir, m.Name, m.Filename)
}

// PartPath returns the path for a partial download.
func (m *Manifest) PartPath(cacheDir string) string {
	return filepath.Join(cacheDir, m.Name, m.Filename+".part")
}

// StatePath returns the path for download state (resumability info).
func (m *Manifest) StatePath(cacheDir string) string {
	return filepath.Join(cacheDir, m.Name, m.Filename+".state")
}

// Delete removes a model from disk.
func (m *Manifest) Delete(cacheDir string) error {
	dir := filepath.Join(cacheDir, m.Name)
	return os.RemoveAll(dir)
}
