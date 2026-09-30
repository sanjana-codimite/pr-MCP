package checks

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/user/mcp-pr-readiness/internal/gitops"
	"github.com/user/mcp-pr-readiness/internal/runner"
)

// testEvent represents a single event from `go test -json` output.
type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

// RunTests executes the test suite and returns a CheckResult.
func RunTests(ctx context.Context, git *gitops.Ops, testCmd []string, testFormat string, timeout time.Duration, maxOutput int) *CheckResult {
	start := time.Now()
	result := &CheckResult{
		Check:  "tests",
		Counts: map[string]int{"passed": 0, "failed": 0, "skipped": 0},
	}

	branch, _ := git.GetBranch(ctx)
	sha, _ := git.GetShortSHA(ctx)
	result.Branch = branch
	result.Commit = sha

	if len(testCmd) == 0 {
		result.Status = StatusERROR
		result.Summary = "No test command configured"
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	r := runner.Run(ctx, git.RepoPath, timeout, maxOutput, testCmd[0], testCmd[1:]...)

	if r.Err != nil {
		result.Status = StatusUNAVAILABLE
		result.Summary = fmt.Sprintf("Test command unavailable: %v", r.Err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	if r.TimedOut {
		result.Status = StatusERROR
		result.Summary = fmt.Sprintf("Tests timed out after %v", timeout)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	if testFormat == "gotest-json" {
		parseGoTestJSON(r.Stdout, result)
	} else {
		if r.ExitCode == 0 {
			result.Status = StatusPASS
			result.Summary = "All tests passed"
		} else {
			result.Status = StatusFAIL
			result.Summary = fmt.Sprintf("Tests failed with exit code %d", r.ExitCode)
			lines := strings.Split(strings.TrimSpace(r.Stdout+"\n"+r.Stderr), "\n")
			tail := lines
			if len(tail) > MaxDetails {
				tail = tail[len(tail)-MaxDetails:]
			}
			result.Details = tail
		}
	}

	result.TruncateDetails()
	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// parseGoTestJSON parses go test -json output to extract test results.
func parseGoTestJSON(output string, result *CheckResult) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	failedTests := make(map[string][]string)
	hasCompileError := false

	var pkgOutputs []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event testEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			if strings.Contains(line, "FAIL") || strings.Contains(line, "build") {
				hasCompileError = true
				result.Details = append(result.Details, line)
			}
			continue
		}

		if event.Test == "" {
			if event.Action == "output" && strings.TrimSpace(event.Output) != "" {
				pkgOutputs = append(pkgOutputs, strings.TrimSpace(event.Output))
			} else if event.Action == "fail" {
				hasCompileError = true
			}
			continue
		}

		switch event.Action {
		case "pass":
			result.Counts["passed"]++
		case "fail":
			result.Counts["failed"]++
			testKey := event.Package + "/" + event.Test
			if outputs, ok := failedTests[testKey]; ok {
				result.Details = append(result.Details, fmt.Sprintf("FAIL %s: %s", testKey, strings.Join(outputs, " ")))
			} else {
				result.Details = append(result.Details, fmt.Sprintf("FAIL %s", testKey))
			}
		case "skip":
			result.Counts["skipped"]++
		case "output":
			if event.Test != "" {
				testKey := event.Package + "/" + event.Test
				failedTests[testKey] = append(failedTests[testKey], strings.TrimSpace(event.Output))
				if len(failedTests[testKey]) > 5 {
					failedTests[testKey] = failedTests[testKey][len(failedTests[testKey])-5:]
				}
			}
		}
	}

	if hasCompileError && result.Counts["passed"] == 0 && result.Counts["failed"] == 0 {
		result.Status = StatusFAIL
		result.Summary = "Compilation failed"
		if len(result.Details) == 0 && len(pkgOutputs) > 0 {
			result.Details = append(result.Details, pkgOutputs...)
		}
		return
	}

	if result.Counts["failed"] > 0 {
		result.Status = StatusFAIL
		result.Summary = fmt.Sprintf("Tests: %d passed, %d failed, %d skipped",
			result.Counts["passed"], result.Counts["failed"], result.Counts["skipped"])
	} else {
		result.Status = StatusPASS
		result.Summary = fmt.Sprintf("Tests: %d passed, %d skipped",
			result.Counts["passed"], result.Counts["skipped"])
	}
}
