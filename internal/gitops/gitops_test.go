package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// initGitRepo creates a minimal git repo with one commit for testing.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("git command %v failed: %v", args, err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("git command %v failed: %v", args, err)
		}
	}
	return dir
}

func TestGetBranch(t *testing.T) {
	dir := initGitRepo(t)
	g := &Ops{RepoPath: dir, Timeout: 10 * time.Second, MaxOutput: 8192}

	branch, err := g.GetBranch(context.Background())
	if err != nil {
		t.Fatalf("GetBranch: %v", err)
	}
	if branch == "" {
		t.Error("expected non-empty branch name")
	}
}

func TestGetShortSHA(t *testing.T) {
	dir := initGitRepo(t)
	g := &Ops{RepoPath: dir, Timeout: 10 * time.Second, MaxOutput: 8192}

	sha, err := g.GetShortSHA(context.Background())
	if err != nil {
		t.Fatalf("GetShortSHA: %v", err)
	}
	if sha == "" || len(sha) > 40 {
		t.Errorf("unexpected SHA %q", sha)
	}
}

func TestIsDirty_Clean(t *testing.T) {
	dir := initGitRepo(t)
	g := &Ops{RepoPath: dir, Timeout: 10 * time.Second, MaxOutput: 8192}

	dirty, files, err := g.IsDirty(context.Background())
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if dirty {
		t.Errorf("expected clean, got dirty with files: %v", files)
	}
}

func TestIsDirty_DirtyTree(t *testing.T) {
	dir := initGitRepo(t)
	g := &Ops{RepoPath: dir, Timeout: 10 * time.Second, MaxOutput: 8192}

	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}

	dirty, _, err := g.IsDirty(context.Background())
	if err != nil {
		t.Fatalf("IsDirty: %v", err)
	}
	if !dirty {
		t.Error("expected dirty, got clean")
	}
}

func TestCheckoutPR_DirtyTreeRefusal(t *testing.T) {
	dir := initGitRepo(t)
	g := &Ops{RepoPath: dir, BaseBranch: "main", Timeout: 10 * time.Second, MaxOutput: 8192}

	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}

	result := g.CheckoutPR(context.Background(), 1, "", "")
	if result.Status != "FAIL" {
		t.Errorf("expected FAIL for dirty tree, got %s: %s", result.Status, result.Summary)
	}
}

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"main", false},
		{"feature/my-branch", false},
		{"pr-123", false},
		{"v1.2.3", false},
		{"branch_name", false},
		{"branch with space", true},
		{"branch;rm -rf /", true},
		{"branch$(cmd)", true},
		{"branch`cmd`", true},
		{"", true},
	}

	for _, tt := range tests {
		err := ValidateBranchName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateBranchName(%q): err=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}

func TestValidateSHA(t *testing.T) {
	tests := []struct {
		sha     string
		wantErr bool
	}{
		{"abc123", false},
		{"abcdef1234567890abcdef1234567890abcdef12", false},
		{"ABC123", false},
		{"0123456789abcdef0123456789abcdef01234567", false},
		{"abcdef1234567890abcdef1234567890abcdef12345", true},
		{"xyz123", true},
		{"abc 123", true},
	}

	for _, tt := range tests {
		err := ValidateSHA(tt.sha)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateSHA(%q): err=%v, wantErr=%v", tt.sha, err, tt.wantErr)
		}
	}
}

func TestGetRepoStatus(t *testing.T) {
	dir := initGitRepo(t)
	g := &Ops{RepoPath: dir, BaseBranch: "main", Timeout: 10 * time.Second, MaxOutput: 8192}

	status := g.GetRepoStatus(context.Background())
	if status.Error != "" {
		t.Errorf("unexpected error: %s", status.Error)
	}
	if status.Branch == "" {
		t.Error("expected non-empty branch")
	}
	if status.Commit == "" {
		t.Error("expected non-empty commit")
	}
	if status.UncommittedFiles != 0 {
		t.Errorf("expected 0 uncommitted files, got %d", status.UncommittedFiles)
	}
}
