// Package report provides saving and loading of PR readiness reports.
package report

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Save writes the report markdown to two files:
//   - report_dir/latest.md (always overwritten)
//   - report_dir/pr-<n>-<timestamp>.md (unique per run)
//
// It creates the directory if missing with mode 0700, and writes files with mode 0600.
// The paths are fixed by the server; the caller controls only the content.
func Save(reportDir string, prNumber int, markdown string) (string, error) {
	// Create directory if needed.
	if err := os.MkdirAll(reportDir, 0700); err != nil {
		return "", fmt.Errorf("creating report directory: %w", err)
	}

	// Validate content size (max 100 KB).
	if len(markdown) > 100*1024 {
		return "", fmt.Errorf("report too large: %d bytes (max 102400)", len(markdown))
	}

	// Write latest.md.
	latestPath := filepath.Join(reportDir, "latest.md")
	if err := os.WriteFile(latestPath, []byte(markdown), 0600); err != nil {
		return "", fmt.Errorf("writing latest report: %w", err)
	}

	// Write timestamped report.
	ts := time.Now().UTC().Format("20060102-150405")
	stampedName := fmt.Sprintf("pr-%d-%s.md", prNumber, ts)
	stampedPath := filepath.Join(reportDir, stampedName)
	if err := os.WriteFile(stampedPath, []byte(markdown), 0600); err != nil {
		return "", fmt.Errorf("writing timestamped report: %w", err)
	}

	return stampedPath, nil
}

// LoadLatest reads the latest report, or returns a fallback message.
func LoadLatest(reportDir string) string {
	path := filepath.Join(reportDir, "latest.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "No report has been generated yet."
	}
	return string(data)
}
