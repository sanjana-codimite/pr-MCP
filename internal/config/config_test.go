package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// createTestRepo creates a temporary git repository for testing.
func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	return dir
}

// writeConfig writes a JSON config file and returns its path.
func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	repoDir := createTestRepo(t)
	cfgDir := t.TempDir()
	cfgPath := writeConfig(t, cfgDir, `{
		"repo_path": "`+filepath.ToSlash(repoDir)+`",
		"base_branch": "develop",
		"test_cmd": ["go", "test", "./..."],
		"test_format": "plain",
		"lint_cmd": ["golangci-lint", "run"],
		"dependency_file": "go.mod",
		"report_dir": "./reports",
		"timeouts_seconds": {"tests": 60, "lint": 30, "audit": 15, "git": 30},
		"max_output_bytes": 4096,
		"osv_base_url": "https://custom.osv.dev"
	}`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.BaseBranch != "develop" {
		t.Errorf("BaseBranch = %q, want %q", cfg.BaseBranch, "develop")
	}
	if cfg.TestFormat != "plain" {
		t.Errorf("TestFormat = %q, want %q", cfg.TestFormat, "plain")
	}
	if cfg.Timeouts.Tests != 60 {
		t.Errorf("Timeouts.Tests = %d, want 60", cfg.Timeouts.Tests)
	}
	if cfg.MaxOutputBytes != 4096 {
		t.Errorf("MaxOutputBytes = %d, want 4096", cfg.MaxOutputBytes)
	}
	if cfg.OSVBaseURL != "https://custom.osv.dev" {
		t.Errorf("OSVBaseURL = %q, want %q", cfg.OSVBaseURL, "https://custom.osv.dev")
	}
}

func TestLoad_Defaults(t *testing.T) {
	repoDir := createTestRepo(t)
	cfgDir := t.TempDir()
	cfgPath := writeConfig(t, cfgDir, `{
		"repo_path": "`+filepath.ToSlash(repoDir)+`"
	}`)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.BaseBranch != "main" {
		t.Errorf("default BaseBranch = %q, want %q", cfg.BaseBranch, "main")
	}
	if cfg.TestFormat != "gotest-json" {
		t.Errorf("default TestFormat = %q, want %q", cfg.TestFormat, "gotest-json")
	}
	if cfg.Timeouts.Tests != 120 {
		t.Errorf("default Timeouts.Tests = %d, want 120", cfg.Timeouts.Tests)
	}
	if cfg.MaxOutputBytes != 8192 {
		t.Errorf("default MaxOutputBytes = %d, want 8192", cfg.MaxOutputBytes)
	}
	if cfg.OSVBaseURL != "https://api.osv.dev" {
		t.Errorf("default OSVBaseURL = %q, want %q", cfg.OSVBaseURL, "https://api.osv.dev")
	}
}

func TestLoad_MissingRepoPath(t *testing.T) {
	cfgDir := t.TempDir()
	cfgPath := writeConfig(t, cfgDir, `{}`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for missing repo_path, got nil")
	}
}

func TestLoad_RelativeRepoPath(t *testing.T) {
	cfgDir := t.TempDir()
	cfgPath := writeConfig(t, cfgDir, `{"repo_path": "./relative/path"}`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for relative repo_path, got nil")
	}
}

func TestLoad_BadTimeouts(t *testing.T) {
	repoDir := createTestRepo(t)
	cfgDir := t.TempDir()
	cfgPath := writeConfig(t, cfgDir, `{
		"repo_path": "`+filepath.ToSlash(repoDir)+`",
		"timeouts_seconds": {"tests": -1, "lint": 60, "audit": 30, "git": 60}
	}`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for negative timeout, got nil")
	}
}

func TestLoad_BadTestFormat(t *testing.T) {
	repoDir := createTestRepo(t)
	cfgDir := t.TempDir()
	cfgPath := writeConfig(t, cfgDir, `{
		"repo_path": "`+filepath.ToSlash(repoDir)+`",
		"test_format": "invalid"
	}`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid test_format, got nil")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	cfgDir := t.TempDir()
	cfgPath := writeConfig(t, cfgDir, `{not valid json}`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoad_NotGitRepo(t *testing.T) {
	// Create a directory that is NOT a git repo.
	dir := t.TempDir()
	cfgDir := t.TempDir()
	cfgPath := writeConfig(t, cfgDir, `{
		"repo_path": "`+filepath.ToSlash(dir)+`"
	}`)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
}
