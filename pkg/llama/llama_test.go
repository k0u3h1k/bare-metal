package llama

import (
    "fmt"
    "net"
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

func TestFreePort(t *testing.T) {
    port, err := FreePort()
    if err != nil {
        t.Fatalf("FreePort() returned error: %v", err)
    }
    if port <= 0 || port > 65535 {
        t.Fatalf("FreePort() returned invalid port: %d", port)
    }
    // Verify the port is actually free by binding to it
    l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
    if err != nil {
        t.Fatalf("port %d reported as free but cannot bind: %v", port, err)
    }
    l.Close()
}

func TestStopIdempotent(t *testing.T) {
    // Stop with no cmd or process should be a no-op
    s := &Server{}
    if err := s.Stop(); err != nil {
        t.Fatalf("Stop() on empty sever: %v", err)
    }
    // Second call should also be a no-op (idempotent)
    if err := s.Stop(); err != nil {
        t.Fatalf("Stop() second call: %v", err)
    }
}

func TestStopNoCmd(t *testing.T) {
    // Sever with nil cmd should not panic or error
    s := &Server{Port: 8080, Host: "127.0.0.1"}
    if err := s.Stop(); err != nil {
        t.Fatalf("Stop with nil cmd: %v", err)
    }
    // Verify stopped flag is set
    s.stopMu.Lock()
    if !s.stopped {
        t.Error("expected stopped=true after Stop() on nil cmd")
    }
    s.stopMu.Unlock()
}