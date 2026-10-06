package model

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// testPayload returns deterministic pseudo-random bytes of the given size.
func testPayload(size int) []byte {
	payload := make([]byte, size)
	rand.New(rand.NewSource(42)).Read(payload)
	return payload
}

// rangeRecorder records Range headers seen by the test server (mutex-guarded,
// since chunk downloads are concurrent).
type rangeRecorder struct {
	mu     sync.Mutex
	ranges []string
}

func (r *rangeRecorder) add(v string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ranges = append(r.ranges, v)
}

func (r *rangeRecorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.ranges...)
}

func (r *rangeRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.ranges)
}

// newRangeServer serves a payload with HEAD (Content-Length) and
// GET-with-Range (206 Partial Content) support, mimicking Hugging Face.
func newRangeServer(payload []byte, rec *rangeRecorder, opts ...func(want http.Request) bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rec != nil && r.Method == http.MethodGet {
			rec.add(r.Header.Get("Range"))
		}
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			start, end := 0, len(payload)-1
			status := http.StatusOK
			if rangeHdr := r.Header.Get("Range"); rangeHdr != "" {
				var s, e int
				if _, err := fmt.Sscanf(rangeHdr, "bytes=%d-%d", &s, &e); err != nil {
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				if s < 0 || e >= len(payload) || s > e {
					w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				start, end = s, e
				status = http.StatusPartialContent
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
			}
			w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
			w.WriteHeader(status)
			_, _ = w.Write(payload[start : end+1])
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}

// newTestManager returns a Manager pointed at temp directories.
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return &Manager{cacheDir: t.TempDir(), binDir: filepath.Join(t.TempDir(), "bin")}
}

// TestDownloadWithProgress_FreshDownload_MultiChunk verifies a complete fresh
// download of a 9 MB payload (3 chunks) produces the exact original content.
func TestDownloadWithProgress_FreshDownload_MultiChunk(t *testing.T) {
	payload := testPayload(9 << 20) // 9 MB -> 3 chunks at 4 MB chunk size
	rec := &rangeRecorder{}
	srv := newRangeServer(payload, rec)
	defer srv.Close()

	m := newTestManager(t)
	manifest := &Manifest{Name: "fresh-model", RepoID: "test/repo", Filename: "model.gguf"}

	if err := m.downloadWithProgress(srv.URL, manifest); err != nil {
		t.Fatalf("downloadWithProgress: %v", err)
	}

	data, err := os.ReadFile(manifest.ModelPath(m.cacheDir))
	if err != nil {
		t.Fatalf("reading final model file: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("downloaded content mismatch: got %d bytes, want %d", len(data), len(payload))
	}
	if manifest.SizeBytes != int64(len(payload)) {
		t.Errorf("manifest.SizeBytes = %d, want %d", manifest.SizeBytes, len(payload))
	}
	if _, err := os.Stat(manifest.PartPath(m.cacheDir)); !os.IsNotExist(err) {
		t.Errorf(".part file should be removed after successful download")
	}
	if rec.count() != 3 {
		t.Errorf("expected 3 chunk GET requests, got %d", rec.count())
	}
}

// TestDownloadWithProgress_ResumeFromPartFile verifies that an existing
// .part file causes the download to resume at its offset (the server sees a
// Range request starting there) and the final file is still complete.
func TestDownloadWithProgress_ResumeFromPartFile(t *testing.T) {
	payload := testPayload(1 << 20) // 1 MB -> single chunk
	rec := &rangeRecorder{}
	srv := newRangeServer(payload, rec)
	defer srv.Close()

	m := newTestManager(t)
	manifest := &Manifest{Name: "resume-model", RepoID: "test/repo", Filename: "model.gguf"}
	partPath := manifest.PartPath(m.cacheDir)
	if err := os.MkdirAll(filepath.Dir(partPath), 0755); err != nil {
		t.Fatal(err)
	}
	cut := int64(400 * 1024)
	if err := os.WriteFile(partPath, payload[:cut], 0644); err != nil {
		t.Fatal(err)
	}

	if err := m.downloadWithProgress(srv.URL, manifest); err != nil {
		t.Fatalf("downloadWithProgress: %v", err)
	}

	data, err := os.ReadFile(manifest.ModelPath(m.cacheDir))
	if err != nil {
		t.Fatalf("reading final model file: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("resumed content mismatch: got %d bytes, want %d", len(data), len(payload))
	}
	ranges := rec.all()
	if len(ranges) == 0 {
		t.Fatal("expected at least one Range GET request")
	}
	if !strings.HasPrefix(ranges[0], fmt.Sprintf("bytes=%d-", cut)) {
		t.Errorf("resume should start at offset %d, got Range %q", cut, ranges[0])
	}
}

// TestDownloadWithProgress_CompletePartRenamesWithoutGET verifies that a
// .part file which already holds the complete payload is finalized (renamed)
// without any chunk GET requests.
func TestDownloadWithProgress_CompletePartRenamesWithoutGET(t *testing.T) {
	payload := testPayload(1 << 20)
	rec := &rangeRecorder{}
	srv := newRangeServer(payload, rec)
	defer srv.Close()

	m := newTestManager(t)
	manifest := &Manifest{Name: "complete-model", RepoID: "test/repo", Filename: "model.gguf"}
	partPath := manifest.PartPath(m.cacheDir)
	if err := os.MkdirAll(filepath.Dir(partPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partPath, payload, 0644); err != nil {
		t.Fatal(err)
	}

	if err := m.downloadWithProgress(srv.URL, manifest); err != nil {
		t.Fatalf("downloadWithProgress: %v", err)
	}

	data, err := os.ReadFile(manifest.ModelPath(m.cacheDir))
	if err != nil {
		t.Fatalf("finalized file should exist at ModelPath: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("finalized content mismatch")
	}
	if _, err := os.Stat(partPath); !os.IsNotExist(err) {
		t.Errorf(".part file should have been renamed away")
	}
	if got := rec.count(); got != 0 {
		t.Errorf("no GET requests expected when .part file is complete, got %d", got)
	}
}

// TestDownloadWithProgress_RetryAfterServerError verifies that a chunk request
// that fails once (HTTP 500) is retried and the download still completes with
// correct content.
func TestDownloadWithProgress_RetryAfterServerError(t *testing.T) {
	payload := testPayload(1 << 20)
	var mu sync.Mutex
	getCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			mu.Lock()
			getCalls++
			calls := getCalls
			mu.Unlock()
			if calls == 1 {
				http.Error(w, "transient failure", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		}
	}))
	defer srv.Close()

	m := newTestManager(t)
	manifest := &Manifest{Name: "retry-model", RepoID: "test/repo", Filename: "model.gguf"}

	if err := m.downloadWithProgress(srv.URL, manifest); err != nil {
		t.Fatalf("downloadWithProgress should succeed after retry: %v", err)
	}

	data, err := os.ReadFile(manifest.ModelPath(m.cacheDir))
	if err != nil {
		t.Fatalf("reading final model file: %v", err)
	}
	if !bytes.Equal(data, payload) {
		t.Errorf("content mismatch after retry")
	}
}

// TestVerifySHA256 verifies checksum validation passes/fails correctly.
func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.bin")
	content := []byte("unbound test content")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(content)
	valid := hex.EncodeToString(sum[:])
	if err := verifySHA256(path, valid); err != nil {
		t.Errorf("verifySHA256(valid) = %v, want nil", err)
	}

	if err := verifySHA256(path, "deadbeef"); err == nil {
		t.Error("verifySHA256(invalid) should fail")
	} else if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("error should mention mismatch, got: %v", err)
	}

	if err := verifySHA256(filepath.Join(dir, "missing.bin"), valid); err == nil {
		t.Error("verifySHA256(missing file) should fail")
	}
}
