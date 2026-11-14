package wineruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ThallesP/macron/internal/failure"
)

const downloadAttempts = 3

func Download(ctx context.Context, cacheDir string, archive Archive) (string, error) {
	path := filepath.Join(cacheDir, archive.SHA256, archive.Name)
	if sum, err := fileSHA256(path); err == nil && sum == archive.SHA256 {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	var lastErr error
	for attempt := range downloadAttempts {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		lastErr = fetch(ctx, archive, path)
		if lastErr == nil || failure.CodeOf(lastErr) == failure.CodeChecksumMismatch {
			break
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return path, nil
}

func fetch(ctx context.Context, archive Archive, path string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, archive.URL, nil)
	if err != nil {
		return failure.Errorf(failure.CodeDownloadFailed, "bad URL %s: %s", archive.URL, err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return failure.Errorf(failure.CodeDownloadFailed, "downloading %s: %s", archive.Name, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return failure.Errorf(failure.CodeDownloadFailed, "downloading %s: HTTP %d", archive.Name, response.StatusCode)
	}

	partial := path + ".partial"
	file, err := os.Create(partial)
	if err != nil {
		return err
	}
	defer os.Remove(partial)
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(file, hash), response.Body)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return failure.Errorf(failure.CodeDownloadFailed, "downloading %s: %s", archive.Name, err)
	}

	sum := hex.EncodeToString(hash.Sum(nil))
	if sum != archive.SHA256 {
		return failure.Errorf(failure.CodeChecksumMismatch, "%s has SHA-256 %s, expected %s", archive.Name, sum, archive.SHA256)
	}
	return os.Rename(partial, path)
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hashing %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
