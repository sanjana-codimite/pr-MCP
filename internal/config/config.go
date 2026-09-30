// Package config provides loading and validation of the PR Readiness server configuration.
//
// The server reads a single JSON file whose path is given by the --config flag
// (default "./.prready.json"). This package validates all fields: repo_path must
// be an absolute path to an existing git repository, timeouts must be positive,
// and commands must be non-empty arrays.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// TimeoutsConfig holds timeout values in seconds for each operation type.
type TimeoutsConfig struct {
	Tests int `json:"tests"`
	Lint  int `json:"lint"`
	Audit int `json:"audit"`
	Git   int `json:"git"`
}

// Config holds the complete server configuration loaded from JSON.
type Config struct {
	// RepoPath is an absolute path to the target git repository.
	RepoPath string `json:"repo_path"`
	// BaseBranch is the branch to diff against (default "main").
	BaseBranch string `json:"base_branch"`
	// TestCmd is the command to run tests, as an argument array (no shell).
	TestCmd []string `json:"test_cmd"`
	// TestFormat is either "gotest-json" or "plain".
	TestFormat string `json:"test_format"`
	// LintCmd is the command to run the linter, as an argument array.
	LintCmd []string `json:"lint_cmd"`
	// DependencyFile is the path to the dependency manifest (relative to repo_path).
	DependencyFile string `json:"dependency_file"`
	// ReportDir is the directory to save reports to (relative or absolute).
	ReportDir string `json:"report_dir"`
	// Timeouts holds timeout values in seconds for each operation.
	Timeouts TimeoutsConfig `json:"timeouts_seconds"`
	// MaxOutputBytes caps captured stdout/stderr from commands.
	MaxOutputBytes int `json:"max_output_bytes"`
	// OSVBaseURL is the base URL for the OSV.dev API.
	OSVBaseURL string `json:"osv_base_url"`
}

// Load reads a configuration file from the given path and returns a validated Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	applyDefaults(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// applyDefaults fills in default values for any optional fields that are empty or zero.
func applyDefaults(cfg *Config) {
	if cfg.BaseBranch == "" {
		cfg.BaseBranch = "main"
	}
	if len(cfg.TestCmd) == 0 {
		cfg.TestCmd = []string{"go", "test", "./...", "-json"}
	}
	if cfg.TestFormat == "" {
		cfg.TestFormat = "gotest-json"
	}
	if len(cfg.LintCmd) == 0 {
		cfg.LintCmd = []string{"go", "vet", "./..."}
	}
	if cfg.DependencyFile == "" {
		cfg.DependencyFile = "go.mod"
	}
	if cfg.ReportDir == "" {
		cfg.ReportDir = "./reports"
	}
	if cfg.Timeouts.Tests == 0 {
		cfg.Timeouts.Tests = 120
	}
	if cfg.Timeouts.Lint == 0 {
		cfg.Timeouts.Lint = 60
	}
	if cfg.Timeouts.Audit == 0 {
		cfg.Timeouts.Audit = 30
	}
	if cfg.Timeouts.Git == 0 {
		cfg.Timeouts.Git = 60
	}
	if cfg.MaxOutputBytes == 0 {
		cfg.MaxOutputBytes = 8192
	}
	if cfg.OSVBaseURL == "" {
		cfg.OSVBaseURL = "https://api.osv.dev"
	}
}

// validate checks that all configuration values are valid.
func validate(cfg *Config) error {
	// repo_path must be absolute and point to an existing git repository.
	if cfg.RepoPath == "" {
		return fmt.Errorf("repo_path is required")
	}
	if !filepath.IsAbs(cfg.RepoPath) {
		return fmt.Errorf("repo_path must be an absolute path, got %q", cfg.RepoPath)
	}

	// Resolve symlinks and verify the path exists.
	resolved, err := filepath.EvalSymlinks(cfg.RepoPath)
	if err != nil {
		return fmt.Errorf("repo_path %q does not exist or is inaccessible: %w", cfg.RepoPath, err)
	}
	cfg.RepoPath = resolved

	// Check that it's a git repository (has a .git directory or file).
	gitPath := filepath.Join(cfg.RepoPath, ".git")
	if _, err := os.Stat(gitPath); os.IsNotExist(err) {
		return fmt.Errorf("repo_path %q is not a git repository (no .git found)", cfg.RepoPath)
	}

	// Validate test_format.
	if cfg.TestFormat != "gotest-json" && cfg.TestFormat != "plain" {
		return fmt.Errorf("test_format must be \"gotest-json\" or \"plain\", got %q", cfg.TestFormat)
	}

	// Validate commands are non-empty.
	if len(cfg.TestCmd) == 0 {
		return fmt.Errorf("test_cmd must be a non-empty array")
	}
	if len(cfg.LintCmd) == 0 {
		return fmt.Errorf("lint_cmd must be a non-empty array")
	}

	// Validate timeouts are positive.
	if cfg.Timeouts.Tests <= 0 {
		return fmt.Errorf("timeouts_seconds.tests must be positive, got %d", cfg.Timeouts.Tests)
	}
	if cfg.Timeouts.Lint <= 0 {
		return fmt.Errorf("timeouts_seconds.lint must be positive, got %d", cfg.Timeouts.Lint)
	}
	if cfg.Timeouts.Audit <= 0 {
		return fmt.Errorf("timeouts_seconds.audit must be positive, got %d", cfg.Timeouts.Audit)
	}
	if cfg.Timeouts.Git <= 0 {
		return fmt.Errorf("timeouts_seconds.git must be positive, got %d", cfg.Timeouts.Git)
	}

	// Validate max_output_bytes is positive.
	if cfg.MaxOutputBytes <= 0 {
		return fmt.Errorf("max_output_bytes must be positive, got %d", cfg.MaxOutputBytes)
	}

	// Resolve report_dir: if relative, make it relative to repo_path.
	if !filepath.IsAbs(cfg.ReportDir) {
		cfg.ReportDir = filepath.Join(cfg.RepoPath, cfg.ReportDir)
	}

	// On Windows, normalize paths.
	if runtime.GOOS == "windows" {
		cfg.RepoPath = filepath.Clean(cfg.RepoPath)
		cfg.ReportDir = filepath.Clean(cfg.ReportDir)
	}

	return nil
}
