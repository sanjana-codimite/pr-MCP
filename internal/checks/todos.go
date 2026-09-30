package checks

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/user/mcp-pr-readiness/internal/gitops"
)

// todoPattern matches TODO and FIXME comments in code.
var todoPattern = regexp.MustCompile(`(?i)\b(TODO|FIXME)\b`)

// FindTodos scans added lines in the PR diff for TODO/FIXME comments.
func FindTodos(ctx context.Context, git *gitops.Ops) *CheckResult {
	start := time.Now()
	result := &CheckResult{
		Check:  "todos",
		Counts: map[string]int{"todos": 0},
	}

	branch, _ := git.GetBranch(ctx)
	sha, _ := git.GetShortSHA(ctx)
	result.Branch = branch
	result.Commit = sha

	addedLines, err := git.DiffAddedLines(ctx)
	if err != nil {
		result.Status = StatusERROR
		result.Summary = fmt.Sprintf("Failed to get diff: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	count := 0
	for _, line := range addedLines {
		if todoPattern.MatchString(line) {
			count++
			result.Details = append(result.Details, strings.TrimSpace(line))
		}
	}
	result.TruncateDetails()
	result.Counts["todos"] = count

	if count > 0 {
		result.Status = StatusWARN
		result.Summary = fmt.Sprintf("Found %d TODO/FIXME in added lines", count)
	} else {
		result.Status = StatusPASS
		result.Summary = "No TODO/FIXME found in added lines"
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}
