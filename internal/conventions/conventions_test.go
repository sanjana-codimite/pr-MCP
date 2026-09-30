package conventions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRead_BothFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CONTRIBUTING.md"), []byte("## Rules\n- test required\n"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My Project\nDescription\n"), 0644)

	content := Read(dir)
	if !strings.Contains(content, "# CONTRIBUTING.md") {
		t.Error("missing CONTRIBUTING.md heading")
	}
	if !strings.Contains(content, "## Rules") {
		t.Error("missing CONTRIBUTING.md content")
	}
	if !strings.Contains(content, "# README.md") {
		t.Error("missing README.md heading")
	}
	if !strings.Contains(content, "# My Project") {
		t.Error("missing README.md content")
	}
}

func TestRead_MissingFiles(t *testing.T) {
	dir := t.TempDir()

	content := Read(dir)
	if !strings.Contains(content, "not found") {
		t.Error("missing 'not found' message for absent files")
	}
}

func TestRead_OnlyContributing(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CONTRIBUTING.md"), []byte("rules here\n"), 0644)

	content := Read(dir)
	if !strings.Contains(content, "rules here") {
		t.Error("missing CONTRIBUTING.md content")
	}
	if !strings.Contains(content, "README.md") {
		t.Error("should mention README.md even if missing")
	}
}
