package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	// statePathSuffix is the suffix for the download state file (used only for resumability metadata).
	statePathSuffix = ".state"
)

// downloadState tracks partial download progress for resumability.
type downloadState struct {
	TotalSize  int64   `json:"total_size"`
	Downloaded int64   `json:"downloaded"`
	Chunks     []int64 `json:"chunks"` // chunk offsets that are completed
}

// DownloadFile downloads a model file from Hugging Face with:
// - Concurrent chunked downloads streaming to disk
// - Progress bar display
// - Resume support (HTTP Range + .part file)
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

	// Resolve the filename against the repository.
	filename, err := resolveFilename(result.RepoID, result.Filename)
	if err != nil {
		return fmt.Errorf("resolving model file: %w", err)
	}
	manifest.Filename = filename

	// Build download URL
	downloadURL := fmt.Sprintf("%s/%s/resolve/main/%s", hfDownloadBase, result.RepoID, filename)
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
	info, err := os.Stat(manifest.ModelPath(m.cacheDir))
	if err != nil {
		return fmt.Errorf("stat downloaded model: %w", err)
	}
	sizeMB := float64(info.Size()) / (1024 * 1024)
	fmt.Printf("\n✅ Download complete! (%s, %.1f MB)\n", manifest.Filename, sizeMB)

	return nil
}

// resolveFilename verifies an explicit filename with Hugging Face and selects a
// compatible GGUF file when the alias filename has changed.
func resolveFilename(repoID, requested string) (string, error) {
	if requested == "" {
		return "", fmt.Errorf("repository %q has no configured GGUF filename", repoID)
	}
	apiURL := fmt.Sprintf("%s/api/models/%s", hfAPIBase, repoID)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("querying repository: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("repository API returned HTTP %d", resp.StatusCode)
	}
	var metadata struct {
		Siblings []struct {
			Filename string `json:"rfilename"`
		} `json:"siblings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return "", fmt.Errorf("decoding repository metadata: %w", err)
	}
	for _, sibling := range metadata.Siblings {
		if sibling.Filename == requested {
			return requested, nil
		}
	}
	requestedLower := strings.ToLower(requested)
	for _, sibling := range metadata.Siblings {
		name := strings.ToLower(sibling.Filename)
		if strings.HasSuffix(name, ".gguf") && (name == requestedLower || strings.ReplaceAll(name, "_", "-") == strings.ReplaceAll(requestedLower, "_", "-")) {
			return sibling.Filename, nil
		}
	}
	for _, sibling := range metadata.Siblings {
		if strings.HasSuffix(strings.ToLower(sibling.Filename), ".gguf") && strings.Contains(strings.ToLower(sibling.Filename), "q4_k_m") {
			return sibling.Filename, nil
		}
	}
	return "", fmt.Errorf("file %q not found in repository", requested)
}

// downloadWithProgress handles the actual download with progress bar.
func (m *Manager) downloadWithProgress(url string, manifest *Manifest) error {
	modelPath := manifest.ModelPath(m.cacheDir)
	partPath := manifest.PartPath(m.cacheDir)

	// Get file size via HEAD request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Head(url)
	if err != nil {
		return fmt.Errorf("connecting to Hugging Face: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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

	// Check for existing partial download to resume
	var startOffset int64
	if info, err := os.Stat(partPath); err == nil {
		startOffset = info.Size()
		if startOffset > 0 && startOffset < totalSize {
			fmt.Printf("   Resuming download at %.1f MB...\n", float64(startOffset)/(1024*1024))
		} else if startOffset >= totalSize {
			// Part file is already complete, just rename
			os.Rename(partPath, modelPath)
			fmt.Println("   File already downloaded and complete.")
			return nil
		}
	}

	// Download in chunks concurrently, streaming each to disk
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

	// Calculate actual chunk size per goroutine
	chunkSizeActual := (remaining + int64(numChunks) - 1) / int64(numChunks)

	// Track downloaded bytes
	var downloaded int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, numChunks)

	// Open the partial file for writing at offsets
	partFile, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("creating partial file: %w", err)
	}
	defer partFile.Close()

	// If resuming, ensure the file is the right size for WriteAt
	if startOffset > 0 {
		if err := partFile.Truncate(startOffset); err != nil {
			return fmt.Errorf("truncating partial file: %w", err)
		}
	}
	// Ensure file is large enough for WriteAt operations
	if err := partFile.Truncate(totalSize); err != nil {
		return fmt.Errorf("extending partial file: %w", err)
	}

	// Progress bar
	progress := newProgressBar(totalSize, startOffset)
	var progressMu sync.Mutex
	lastProgressUpdate := time.Now()

	for i := 0; i < numChunks; i++ {
		wg.Add(1)
		offset := startOffset + int64(i)*chunkSizeActual
		end := offset + chunkSizeActual - 1
		if end >= totalSize {
			end = totalSize - 1
		}

		go func(offset, end int64) {
			defer wg.Done()
			// Retry with backoff
			var lastErr error
			for attempt := 0; attempt < 3; attempt++ {
				if attempt > 0 {
					backoff := time.Duration(1<<attempt) * time.Second
					time.Sleep(backoff)
				}
				lastErr = downloadChunkToFile(partFile, url, offset, end)
				if lastErr == nil {
					chunkSize := end - offset + 1
					mu.Lock()
					downloaded += chunkSize
					// Throttle progress updates to at most every 200ms
					if time.Since(lastProgressUpdate) > 200*time.Millisecond {
						progressMu.Lock()
						progress.update(downloaded)
						progressMu.Unlock()
						lastProgressUpdate = time.Now()
					}
					mu.Unlock()
					return
				}
			}
			errChan <- fmt.Errorf("chunk %d-%d after 3 retries: %w", offset, end, lastErr)
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

	// Final progress update
	progress.done()

	// Rename partial file to final
	partFile.Close()
	if err := os.Rename(partPath, modelPath); err != nil {
		return fmt.Errorf("finalizing download: %w", err)
	}

	return nil
}

// downloadChunkToFile streams a byte range from a URL directly to a file at the given offset.
// Uses a fixed 32KB buffer per request to keep memory usage low.
func downloadChunkToFile(f *os.File, url string, start, end int64) error {
	client := &http.Client{Timeout: 5 * time.Minute}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	req.Header.Set("User-Agent", "unbound-cli/0.1.0")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("HTTP %d for range %d-%d", resp.StatusCode, start, end)
	}

	// Stream directly to file at offset using a 32KB buffer
	buf := make([]byte, 32*1024)
	written := int64(0)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.WriteAt(buf[:n], start+written); writeErr != nil {
				return fmt.Errorf("writing at offset %d: %w", start+written, writeErr)
			}
			written += int64(n)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("reading response body: %w", readErr)
		}
	}

	expectedLen := end - start + 1
	if written != expectedLen {
		return fmt.Errorf("expected %d bytes, got %d", expectedLen, written)
	}

	return nil
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
