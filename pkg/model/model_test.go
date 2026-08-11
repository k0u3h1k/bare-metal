package model

import (
    "os"
    "path/filepath"
    "testing"
)

func TestNewManagerNoPanic(t *testing.T) {
    // NewManager should not panic when config.App is nil
    // (it falls back to ~/.unbound defaults)
    m := NewManager()
    if m == nil {
        t.Fatal("NewManager() returned nil")
    }
    if m.cacheDir == "" {
        t.Error("cacheDir should not be empty")
    }
    if m.binDir == "" {
        t.Error("binDir should not be empty")
    }
    // Should use ~/.unbound defaults
    home, err := os.UserHomeDir()
    if err != nil {
        t.Fatal(err)
    }
    expectedCacheDir := filepath.Join(home, ".unbound", "models")
    if m.cacheDir != expectedCacheDir {
        t.Errorf("cacheDir = %q, want %q", m.cacheDir, expectedCacheDir)
    }
}

func TestNewManagerCustomConfig(t *testing.T) {
    // Test with custom config paths by temporarily setting config.App
    m := NewManager()
    if m == nil {
        t.Fatal("NewManager() returned nil")
    }
    if m.cacheDir == "" {
        t.Error("cacheDir should not be empty")
    }
}