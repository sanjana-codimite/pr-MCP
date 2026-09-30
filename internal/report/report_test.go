package report

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSave_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	reportDir := filepath.Join(dir, "reports")

	path, err := Save(reportDir, 42, "# Test Report\nAll good.")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Check latest.md exists.
	latestPath := filepath.Join(reportDir, "latest.md")
	data, err := os.ReadFile(latestPath)
	if err != nil {
		t.Fatalf("reading latest.md: %v", err)
	}
	if string(data) != "# Test Report\nAll good." {
		t.Errorf("latest.md content = %q, want test content", string(data))
	}

	// Check timestamped file exists.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("timestamped file %q not found", path)
	}
}

func TestSave_TooLarge(t *testing.T) {
	dir := t.TempDir()
	bigContent := make([]byte, 200*1024) // 200 KB
	for i := range bigContent {
		bigContent[i] = 'A'
	}

	_, err := Save(dir, 1, string(bigContent))
	if err == nil {
		t.Fatal("expected error for oversized report, got nil")
	}
}

func TestLoadLatest_NoReport(t *testing.T) {
	dir := t.TempDir()
	content := LoadLatest(dir)
	if content != "No report has been generated yet." {
		t.Errorf("got %q, want fallback message", content)
	}
}

func TestLoadLatest_WithReport(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "latest.md"), []byte("# Report"), 0600)

	content := LoadLatest(dir)
	if content != "# Report" {
		t.Errorf("got %q, want %q", content, "# Report")
	}
}
