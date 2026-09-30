package checks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/user/mcp-pr-readiness/internal/gitops"
	"github.com/user/mcp-pr-readiness/internal/runner"
)

// RunLinter executes the configured lint command and parses the output.
//
// MCP concept: This is a Tool (model-controlled). The LLM calls it to check
// for code quality issues in the target repository.
//
// Parsing: each output line matching "file:line:col: message" is one finding.
// Status: PASS if no findings, FAIL if errors found, UNAVAILABLE if the
// executable is not installed.
func RunLinter(ctx context.Context, git *gitops.Ops, lintCmd []string, timeout time.Duration, maxOutput int) *CheckResult {
	start := time.Now()
	result := &CheckResult{
		Check:  "lint",
		Counts: map[string]int{"errors": 0, "warnings": 0},
	}

	branch, _ := git.GetBranch(ctx)
	sha, _ := git.GetShortSHA(ctx)
	result.Branch = branch
	result.Commit = sha

	if len(lintCmd) == 0 {
		result.Status = StatusERROR
		result.Summary = "No lint command configured"
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	r := runner.Run(ctx, git.RepoPath, timeout, maxOutput, lintCmd[0], lintCmd[1:]...)

	if r.Err != nil {
		result.Status = StatusUNAVAILABLE
		result.Summary = fmt.Sprintf("Linter unavailable: %v", r.Err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	if r.TimedOut {
		result.Status = StatusERROR
		result.Summary = fmt.Sprintf("Linter timed out after %v", timeout)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Parse output: combine stdout and stderr.
	output := r.Stdout + "\n" + r.Stderr
	findings := parseLintOutput(output)

	result.Counts["errors"] = len(findings)
	result.Details = findings
	result.TruncateDetails()

	if len(findings) == 0 && r.ExitCode == 0 {
		result.Status = StatusPASS
		result.Summary = "No lint findings"
	} else if len(findings) > 0 {
		result.Status = StatusFAIL
		result.Summary = fmt.Sprintf("%d lint finding(s)", len(findings))
	} else if r.ExitCode != 0 {
		// Exit code non-zero but no parseable findings.
		result.Status = StatusFAIL
		result.Summary = fmt.Sprintf("Linter exited with code %d", r.ExitCode)
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				result.Details = append(result.Details, l)
			}
		}
		result.TruncateDetails()
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// parseLintOutput extracts findings from lint output.
func parseLintOutput(output string) []string {
	var findings []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) >= 3 {
			lineNum := strings.TrimSpace(parts[1])
			isLineNum := true
			for _, c := range lineNum {
				if c < '0' || c > '9' {
					isLineNum = false
					break
				}
			}
			if isLineNum && lineNum != "" {
				findings = append(findings, line)
			}
		}
	}
	return findings
}
