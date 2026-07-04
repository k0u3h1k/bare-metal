package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestSaveAndLoad(t *testing.T) {
	cacheDir := t.TempDir()

	m := &Manifest{
		Name:         "test-model",
		RepoID:       "test-org/test-repo",
		Filename:     "model.gguf",
		SizeBytes:    1024 * 1024 * 100, // 100 MB
		SHA256:       "abc123",
		Quantization: "Q4_K_M",
		Params:       "7B",
		IsReady:      true,
	}

	// Save
	if err := SaveManifest(cacheDir, m); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}

	// Check file exists
	manifestPath := ManifestPath(cacheDir, "test-model")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("manifest file not created at %s", manifestPath)
	}

	// Load
	loaded, err := LoadManifest(cacheDir, "test-model")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if loaded.Name != m.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, m.Name)
	}
	if loaded.RepoID != m.RepoID {
		t.Errorf("RepoID = %q, want %q", loaded.RepoID, m.RepoID)
	}
	if loaded.Filename != m.Filename {
		t.Errorf("Filename = %q, want %q", loaded.Filename, m.Filename)
	}
	if loaded.SizeBytes != m.SizeBytes {
		t.Errorf("SizeBytes = %d, want %d", loaded.SizeBytes, m.SizeBytes)
	}
	if loaded.SHA256 != m.SHA256 {
		t.Errorf("SHA256 = %q, want %q", loaded.SHA256, m.SHA256)
	}
	if loaded.Quantization != m.Quantization {
		t.Errorf("Quantization = %q, want %q", loaded.Quantization, m.Quantization)
	}
	if loaded.Params != m.Params {
		t.Errorf("Params = %q, want %q", loaded.Params, m.Params)
	}
	if loaded.IsReady != m.IsReady {
		t.Errorf("IsReady = %v, want %v", loaded.IsReady, m.IsReady)
	}
}

func TestManifestPaths(t *testing.T) {
	cacheDir := "/tmp/.unbound/models"
	m := &Manifest{
		Name:     "test-model",
		Filename: "model.gguf",
	}

	expectedManifest := filepath.Join(cacheDir, "test-model", "manifest.json")
	if got := ManifestPath(cacheDir, "test-model"); got != expectedManifest {
		t.Errorf("ManifestPath = %q, want %q", got, expectedManifest)
	}

	expectedModel := filepath.Join(cacheDir, "test-model", "model.gguf")
	if got := m.ModelPath(cacheDir); got != expectedModel {
		t.Errorf("ModelPath = %q, want %q", got, expectedModel)
	}

	expectedPart := filepath.Join(cacheDir, "test-model", "model.gguf.part")
	if got := m.PartPath(cacheDir); got != expectedPart {
		t.Errorf("PartPath = %q, want %q", got, expectedPart)
	}
}

func TestManifestDelete(t *testing.T) {
	cacheDir := t.TempDir()
	m := &Manifest{
		Name:     "test-model",
		Filename: "model.gguf",
	}

	if err := SaveManifest(cacheDir, m); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}

	// Verify it exists
	if _, err := os.Stat(filepath.Join(cacheDir, "test-model")); os.IsNotExist(err) {
		t.Fatal("model directory should exist after save")
	}

	// Delete
	if err := m.Delete(cacheDir); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(filepath.Join(cacheDir, "test-model")); !os.IsNotExist(err) {
		t.Error("model directory should be deleted")
	}
}
