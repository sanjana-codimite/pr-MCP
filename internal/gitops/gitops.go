// Package gitops provides safe git operations on the target repository.
//
// All operations work on repo_path only. Branch names are regex-validated
// to prevent injection. The dirty-tree check ensures checkout_pr refuses
// when uncommitted changes exist.
package gitops

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/user/mcp-pr-readiness/internal/runner"
)

// branchNamePattern validates branch names to prevent injection.
var branchNamePattern = regexp.MustCompile(`^[A-Za-z0-9._/\-]+$`)

// Ops provides git operations on a specific repository.
type Ops struct {
	RepoPath   string
	BaseBranch string
	Timeout    time.Duration
	MaxOutput  int
}

// RepoStatus holds the result of a get_repo_status call.
type RepoStatus struct {
	Branch           string
	Commit           string
	UncommittedFiles int
	DirtyFiles       []string
	Error            string
}

// CheckoutResult holds the result of a checkout_pr call.
type CheckoutResult struct {
	Branch  string
	Commit  string
	Status  string // PASS, FAIL, WARN, ERROR
	Summary string
}

// GetBranch returns the current branch name.
func (g *Ops) GetBranch(ctx context.Context) (string, error) {
	r := runner.Run(ctx, g.RepoPath, g.Timeout, g.MaxOutput, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if r.Err != nil {
		return "", fmt.Errorf("git rev-parse branch: %w", r.Err)
	}
	if r.ExitCode != 0 {
		return "", fmt.Errorf("git rev-parse branch failed: %s", r.Stderr)
	}
	return strings.TrimSpace(r.Stdout), nil
}

// GetShortSHA returns the short SHA of HEAD.
func (g *Ops) GetShortSHA(ctx context.Context) (string, error) {
	r := runner.Run(ctx, g.RepoPath, g.Timeout, g.MaxOutput, "git", "rev-parse", "--short", "HEAD")
	if r.Err != nil {
		return "", fmt.Errorf("git rev-parse SHA: %w", r.Err)
	}
	if r.ExitCode != 0 {
		return "", fmt.Errorf("git rev-parse SHA failed: %s", r.Stderr)
	}
	return strings.TrimSpace(r.Stdout), nil
}

// IsDirty returns true if the working tree has uncommitted changes.
func (g *Ops) IsDirty(ctx context.Context) (bool, []string, error) {
	r := runner.Run(ctx, g.RepoPath, g.Timeout, g.MaxOutput, "git", "status", "--porcelain")
	if r.Err != nil {
		return false, nil, fmt.Errorf("git status: %w", r.Err)
	}
	if r.ExitCode != 0 {
		return false, nil, fmt.Errorf("git status failed: %s", r.Stderr)
	}

	output := strings.TrimSpace(r.Stdout)
	if output == "" {
		return false, nil, nil
	}

	files := strings.Split(output, "\n")
	return true, files, nil
}

// GetRepoStatus returns the current branch, commit, and dirty state.
func (g *Ops) GetRepoStatus(ctx context.Context) *RepoStatus {
	status := &RepoStatus{}

	branch, err := g.GetBranch(ctx)
	if err != nil {
		status.Error = fmt.Sprintf("Failed to get branch: %v", err)
		return status
	}
	status.Branch = branch

	sha, err := g.GetShortSHA(ctx)
	if err != nil {
		status.Error = fmt.Sprintf("Failed to get SHA: %v", err)
		return status
	}
	status.Commit = sha

	dirty, files, err := g.IsDirty(ctx)
	if err != nil {
		status.Error = fmt.Sprintf("Failed to check dirty state: %v", err)
		return status
	}

	if dirty {
		status.UncommittedFiles = len(files)
		for _, f := range files {
			status.DirtyFiles = append(status.DirtyFiles, strings.TrimSpace(f))
		}
	}

	return status
}

// ValidateBranchName checks that a branch name matches the safe pattern.
func ValidateBranchName(name string) error {
	if !branchNamePattern.MatchString(name) {
		return fmt.Errorf("invalid branch name %q: must match %s", name, branchNamePattern.String())
	}
	return nil
}

// ValidateSHA checks that a SHA string is valid hex, max 40 chars.
func ValidateSHA(sha string) error {
	if len(sha) > 40 {
		return fmt.Errorf("SHA too long: %d chars (max 40)", len(sha))
	}
	for _, c := range sha {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("SHA contains invalid character: %c", c)
		}
	}
	return nil
}

// CheckoutPR checks out a PR branch. It refuses if the working tree is dirty.
func (g *Ops) CheckoutPR(ctx context.Context, prNumber int, branch string, expectedSHA string) *CheckoutResult {
	result := &CheckoutResult{}

	// Step 1: Refuse if dirty.
	dirty, _, err := g.IsDirty(ctx)
	if err != nil {
		result.Status = "ERROR"
		result.Summary = fmt.Sprintf("Failed to check dirty state: %v", err)
		return result
	}
	if dirty {
		result.Status = "FAIL"
		result.Summary = "Working tree is dirty; commit or stash changes before checking out a PR"
		return result
	}

	// Step 2: Fetch and checkout.
	var localBranch string
	if branch == "" {
		localBranch = fmt.Sprintf("pr-%d", prNumber)
		refSpec := fmt.Sprintf("pull/%d/head:%s", prNumber, localBranch)
		r := runner.Run(ctx, g.RepoPath, g.Timeout, g.MaxOutput, "git", "fetch", "origin", refSpec)
		if r.Err != nil || r.ExitCode != 0 {
			errMsg := r.Stderr
			if r.Err != nil {
				errMsg = r.Err.Error()
			}
			result.Status = "ERROR"
			result.Summary = fmt.Sprintf("Failed to fetch PR #%d: %s", prNumber, strings.TrimSpace(errMsg))
			return result
		}
	} else {
		if err := ValidateBranchName(branch); err != nil {
			result.Status = "FAIL"
			result.Summary = err.Error()
			return result
		}
		localBranch = branch

		r := runner.Run(ctx, g.RepoPath, g.Timeout, g.MaxOutput, "git", "fetch", "origin", branch)
		if r.Err != nil || r.ExitCode != 0 {
			errMsg := r.Stderr
			if r.Err != nil {
				errMsg = r.Err.Error()
			}
			result.Status = "ERROR"
			result.Summary = fmt.Sprintf("Failed to fetch branch %q: %s", branch, strings.TrimSpace(errMsg))
			return result
		}
	}

	r := runner.Run(ctx, g.RepoPath, g.Timeout, g.MaxOutput, "git", "checkout", localBranch)
	if r.Err != nil || r.ExitCode != 0 {
		errMsg := r.Stderr
		if r.Err != nil {
			errMsg = r.Err.Error()
		}
		result.Status = "ERROR"
		result.Summary = fmt.Sprintf("Failed to checkout %s: %s", localBranch, strings.TrimSpace(errMsg))
		return result
	}

	currentBranch, _ := g.GetBranch(ctx)
	currentSHA, _ := g.GetShortSHA(ctx)
	result.Branch = currentBranch
	result.Commit = currentSHA

	// Step 3: SHA comparison if expected.
	if expectedSHA != "" {
		if err := ValidateSHA(expectedSHA); err != nil {
			result.Status = "WARN"
			result.Summary = fmt.Sprintf("Checked out %s @ %s; SHA validation error: %v", localBranch, currentSHA, err)
			return result
		}

		fullR := runner.Run(ctx, g.RepoPath, g.Timeout, g.MaxOutput, "git", "rev-parse", "HEAD")
		fullSHA := strings.TrimSpace(fullR.Stdout)

		if strings.HasPrefix(fullSHA, expectedSHA) || strings.HasPrefix(expectedSHA, currentSHA) {
			result.Status = "PASS"
			result.Summary = fmt.Sprintf("Checked out %s @ %s; SHA match", localBranch, currentSHA)
		} else {
			result.Status = "WARN"
			result.Summary = fmt.Sprintf("Checked out %s @ %s; SHA mismatch (expected prefix %s)", localBranch, currentSHA, expectedSHA)
		}
	} else {
		result.Status = "PASS"
		result.Summary = fmt.Sprintf("Checked out %s @ %s", localBranch, currentSHA)
	}

	return result
}

// DiffAddedLines returns added lines from git diff base_branch...HEAD -U0.
func (g *Ops) DiffAddedLines(ctx context.Context) ([]string, error) {
	r := runner.Run(ctx, g.RepoPath, g.Timeout, g.MaxOutput,
		"git", "diff", g.BaseBranch+"...HEAD", "-U0")
	if r.Err != nil {
		return nil, fmt.Errorf("git diff: %w", r.Err)
	}
	if r.ExitCode != 0 {
		return nil, fmt.Errorf("git diff failed: %s", r.Stderr)
	}

	var added []string
	for _, line := range strings.Split(r.Stdout, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added = append(added, line[1:])
		}
	}
	return added, nil
}
