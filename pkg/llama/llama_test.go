package llama

import (
	"runtime"
	"strings"
	"testing"
)

func TestAssetName(t *testing.T) {
	s := &Server{}
	asset := s.assetName()

	// Should start with the release tag
	if !strings.HasPrefix(asset, "llama-"+llamaReleaseTag+"-bin-") {
		t.Errorf("asset name should start with 'llama-%s-bin-', got %q", llamaReleaseTag, asset)
	}

	// Should end with .tar.gz
	if !strings.HasSuffix(asset, ".tar.gz") {
		t.Errorf("asset name should end with .tar.gz, got %q", asset)
	}

	// Should contain the platform
	switch runtime.GOOS {
	case "linux":
		if !strings.Contains(asset, "ubuntu") && !strings.Contains(asset, "linux") {
			t.Errorf("linux asset should contain 'ubuntu', got %q", asset)
		}
	case "darwin":
		if !strings.Contains(asset, "macos") {
			t.Errorf("macOS asset should contain 'macos', got %q", asset)
		}
	}
}

func TestNewServer(t *testing.T) {
	s := NewServer("/tmp/test.gguf")
	if s.ModelPath != "/tmp/test.gguf" {
		t.Errorf("ModelPath = %q, want %q", s.ModelPath, "/tmp/test.gguf")
	}
	if s.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want %q", s.Host, "127.0.0.1")
	}
	if s.CtxSize != 4096 {
		t.Errorf("CtxSize = %d, want %d", s.CtxSize, 4096)
	}
}

func TestServerNotRunning(t *testing.T) {
	s := &Server{}
	if s.IsRunning() {
		t.Error("empty server should not be running")
	}
}
