package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// chunkSize is the size of each concurrent download chunk (4 MB).
	chunkSize = 4 * 1024 * 1024
	// maxChunks is the maximum number of concurrent download goroutines.
	maxChunks = 4
	// hfAPIBase is the Hugging Face API base URL.
	hfAPIBase = "https://huggingface.co"
	// hfDownloadBase is the Hugging Face file download base.
	hfDownloadBase = "https://huggingface.co"
)

// downloadState tracks partial download progress for resumability.
type downloadState struct {
	TotalSize  int64   `json:"total_size"`
	Downloaded int64   `json:"downloaded"`
	Chunks     []int64 `json:"chunks"` // chunk offsets that are completed
}

// DownloadFile downloads a model file from Hugging Face with:
// - Concurrent chunked downloads
// - Progress bar display
// - Resume support (HTTP Range)
// - SHA256 verification
func (m *Manager) DownloadFile(result *ResolveResult) error {
	manifest := &Manifest{
		Name:         result.Alias,
		RepoID:       result.RepoID,
		Filename:     result.Filename,
		Params:       result.Params,
		Quantization: result.Quantization,
		DownloadedAt: time.Now(),
		IsReady:      false,
	}

	// Ensure model directory exists
	modelDir := filepath.Join(m.cacheDir, manifest.Name)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("creating model directory: %w", err)
	}

	// Save initial manifest
	if err := SaveManifest(m.cacheDir, manifest); err != nil {
		return err
	}

	// Build download URL
	downloadURL := fmt.Sprintf("%s/%s/resolve/main/%s", hfDownloadBase, result.RepoID, result.Filename)
	fmt.Printf("\n📥 Downloading %s\n", result.Alias)
	fmt.Printf("   From: %s\n", downloadURL)
	if result.Params != "" {
		fmt.Printf("   Size: %s parameters, quantization: %s\n", result.Params, result.Quantization)
	}
	fmt.Println()

	// Perform the download
	if err := m.downloadWithProgress(downloadURL, manifest); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	manifest.IsReady = true
	manifest.DownloadedAt = time.Now()

	// Verify SHA256 if we have one
	if manifest.SHA256 != "" {
		fmt.Print("\n🔍 Verifying checksum... ")
		if err := verifySHA256(manifest.ModelPath(m.cacheDir), manifest.SHA256); err != nil {
			fmt.Println("❌ FAILED")
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		fmt.Println("✅ OK")
	}

	// Save final manifest
	if err := SaveManifest(m.cacheDir, manifest); err != nil {
		return err
	}

	// Clean up any partial files
	os.Remove(manifest.PartPath(m.cacheDir))
	os.Remove(manifest.StatePath(m.cacheDir))

	// Calculate and display final size
	info, _ := os.Stat(manifest.ModelPath(m.cacheDir))
	sizeMB := float64(info.Size()) / (1024 * 1024)
	fmt.Printf("\n✅ Download complete! (%s, %.1f MB)\n", manifest.Filename, sizeMB)

	return nil
}

// downloadWithProgress handles the actual download with progress bar.
func (m *Manager) downloadWithProgress(url string, manifest *Manifest) error {
	modelPath := manifest.ModelPath(m.cacheDir)
	partPath := manifest.PartPath(m.cacheDir)
	statePath := manifest.StatePath(m.cacheDir)

	// Check for existing partial download
	var resumeBytes int64
	if state, err := loadState(statePath); err == nil {
		resumeBytes = state.Downloaded
		fmt.Printf("   Resuming download at %.1f MB...\n", float64(resumeBytes)/(1024*1024))
	}

	// Get file size and check if we already have the complete file
	// We do a HEAD request first
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		return fmt.Errorf("connecting to Hugging Face: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("HTTP %d from Hugging Face for %s", resp.StatusCode, url)
	}

	totalSize, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if totalSize <= 0 {
		return fmt.Errorf("unable to determine file size from server")
	}

	manifest.SizeBytes = totalSize

	// If we already have the complete file at the right size, skip
	if info, err := os.Stat(modelPath); err == nil && info.Size() == totalSize {
		fmt.Println("   File already downloaded and complete.")
		return nil
	}

	// If partial file exists and matches expected size, rename and use it
	if info, err := os.Stat(partPath); err == nil && info.Size() == totalSize {
		os.Rename(partPath, modelPath)
		fmt.Println("   File already downloaded and complete.")
		return nil
	}

	// Determine start point (for resume)
	var startOffset int64
	if resumeBytes > 0 {
		startOffset = resumeBytes
	}

	// Open or create the partial file
	partFile, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("creating partial file: %w", err)
	}
	defer partFile.Close()

	// Seek to the resume position
	if startOffset > 0 {
		if _, err := partFile.Seek(startOffset, io.SeekStart); err != nil {
			return fmt.Errorf("seeking to resume position: %w", err)
		}
	}

	// Download in chunks concurrently
	remaining := totalSize - startOffset
	numChunks := int(remaining / chunkSize)
	if remaining%chunkSize != 0 {
		numChunks++
	}
	if numChunks > maxChunks {
		numChunks = maxChunks
	}
	if numChunks < 1 {
		numChunks = 1
	}

	chunkSizeActual := remaining / int64(numChunks)
	if remaining%int64(numChunks) != 0 {
		chunkSizeActual++
	}

	// Track downloaded bytes
	var downloaded int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, numChunks)

	progress := newProgressBar(totalSize, startOffset)

	for i := 0; i < numChunks; i++ {
		wg.Add(1)
		offset := startOffset + int64(i)*chunkSizeActual
		end := offset + chunkSizeActual - 1
		if end >= totalSize {
			end = totalSize - 1
		}

		go func(offset, end int64) {
			defer wg.Done()
			data, err := downloadRange(url, offset, end)
			if err != nil {
				errChan <- fmt.Errorf("chunk %d-%d: %w", offset, end, err)
				return
			}

			mu.Lock()
			// Write chunk at the correct position
			if _, err := partFile.WriteAt(data, offset); err != nil {
				mu.Unlock()
				errChan <- fmt.Errorf("writing chunk at %d: %w", offset, err)
				return
			}
			downloaded += int64(len(data))
			progress.update(downloaded)
			mu.Unlock()
		}(offset, end)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	progress.done()

	// Rename partial file to final
	partFile.Close()
	if err := os.Rename(partPath, modelPath); err != nil {
		return fmt.Errorf("finalizing download: %w", err)
	}

	return nil
}

// downloadRange downloads a specific byte range from a URL.
func downloadRange(url string, start, end int64) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	req.Header.Set("User-Agent", "unbound-cli/0.1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("HTTP %d for range %d-%d", resp.StatusCode, start, end)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	expectedLen := end - start + 1
	if int64(len(data)) != expectedLen {
		return nil, fmt.Errorf("expected %d bytes, got %d", expectedLen, len(data))
	}

	return data, nil
}

// loadState reads download state from disk.
func loadState(path string) (*downloadState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state downloadState
	// Simple parsing: first line is total_size, second line is downloaded
	lines := strings.SplitN(string(data), "\n", 2)
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid state file")
	}
	fmt.Sscanf(lines[0], "%d", &state.TotalSize)
	fmt.Sscanf(lines[1], "%d", &state.Downloaded)
	return &state, nil
}

// saveState persists download state to disk.
func (m *Manager) saveState(path string, totalSize, downloaded int64) error {
	data := fmt.Sprintf("%d\n%d\n", totalSize, downloaded)
	return os.WriteFile(path, []byte(data), 0644)
}

// verifySHA256 checks that a file matches the expected SHA256 hash.
func verifySHA256(path, expectedHash string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expectedHash, actualHash)
	}
	return nil
}

// progressBar displays a simple terminal progress bar.
type progressBar struct {
	total     int64
	current   int64
	startTime time.Time
	width     int
}

func newProgressBar(total, current int64) *progressBar {
	return &progressBar{
		total:     total,
		current:   current,
		startTime: time.Now(),
		width:     40,
	}
}

func (p *progressBar) update(downloaded int64) {
	p.current = downloaded
	pct := float64(p.current) / float64(p.total) * 100
	filled := int(float64(p.width) * float64(p.current) / float64(p.total))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)

	downloadedMB := float64(p.current) / (1024 * 1024)
	totalMB := float64(p.total) / (1024 * 1024)

	elapsed := time.Since(p.startTime).Seconds()
	speed := float64(0)
	if elapsed > 0 {
		speed = float64(p.current) / (1024 * 1024) / elapsed
	}

	// ETA
	eta := ""
	if speed > 0 {
		remainingSec := float64(p.total-p.current) / (1024 * 1024) / speed
		if remainingSec > 0 {
			eta = fmt.Sprintf(" ETA: %ds", int(remainingSec))
		}
	}

	fmt.Printf("\r   [%s] %.1f%%  %.1f/%.1f MB  %.1f MB/s%s   ",
		bar, pct, downloadedMB, totalMB, speed, eta)
}

func (p *progressBar) done() {
	elapsed := time.Since(p.startTime).Seconds()
	speed := float64(p.total) / (1024 * 1024) / elapsed
	totalMB := float64(p.total) / (1024 * 1024)
	fmt.Printf("\r   [%s] 100%%  %.1f/%.1f MB  %.1f MB/s  (%ds)\n",
		strings.Repeat("█", p.width), totalMB, totalMB, speed, int(elapsed))
}
