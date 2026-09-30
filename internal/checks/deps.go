package checks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/mod/modfile"

	"github.com/user/mcp-pr-readiness/internal/gitops"
	"github.com/user/mcp-pr-readiness/internal/osv"
	"github.com/user/mcp-pr-readiness/internal/runner"
)

// AuditDependencies scans the project's go.mod for known vulnerabilities.
func AuditDependencies(ctx context.Context, git *gitops.Ops, depFile string, osvClient *osv.Client, timeout time.Duration, maxOutput int) *CheckResult {
	start := time.Now()
	result := &CheckResult{
		Check: "dependencies",
		Counts: map[string]int{
			"critical": 0, "high": 0, "moderate": 0,
			"low": 0, "unknown": 0, "modules_scanned": 0,
		},
	}

	branch, _ := git.GetBranch(ctx)
	sha, _ := git.GetShortSHA(ctx)
	result.Branch = branch
	result.Commit = sha

	modPath := filepath.Join(git.RepoPath, depFile)
	data, err := os.ReadFile(modPath)
	if err != nil {
		result.Status = StatusERROR
		result.Summary = fmt.Sprintf("Failed to read %s: %v", depFile, err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	f, err := modfile.Parse(depFile, data, nil)
	if err != nil {
		result.Status = StatusERROR
		result.Summary = fmt.Sprintf("Failed to parse %s: %v", depFile, err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	var queries []osv.QueryBatchEntry
	for _, req := range f.Require {
		version := osv.FormatGoVersion(req.Mod.Version)
		queries = append(queries, osv.QueryBatchEntry{
			Package: osv.QueryPackage{Name: req.Mod.Path, Ecosystem: "Go"},
			Version: version,
		})
	}

	result.Counts["modules_scanned"] = len(queries)

	if len(queries) == 0 {
		result.Status = StatusPASS
		result.Summary = "No dependencies to audit"
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	batchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	batchResp, err := osvClient.QueryBatch(batchCtx, queries)
	if err != nil {
		result = tryGovulncheck(ctx, git, timeout, maxOutput, result)
		if result.Status != StatusUNAVAILABLE {
			result.DurationMs = time.Since(start).Milliseconds()
			return result
		}
		result.Status = StatusUNAVAILABLE
		result.Summary = fmt.Sprintf("OSV.dev unreachable: %v", err)
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	var vulnIDs []string
	vulnToModule := make(map[string]string)
	for i, res := range batchResp.Results {
		for _, v := range res.Vulns {
			vulnIDs = append(vulnIDs, v.ID)
			if i < len(queries) {
				vulnToModule[v.ID] = queries[i].Package.Name + "@" + queries[i].Version
			}
		}
	}

	if len(vulnIDs) == 0 {
		result.Status = StatusPASS
		result.Summary = fmt.Sprintf("No vulnerabilities found in %d modules", len(queries))
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	detailCtx, detailCancel := context.WithTimeout(ctx, timeout)
	defer detailCancel()

	vulns, _ := osvClient.FetchVulnDetails(detailCtx, vulnIDs)

	for _, v := range vulns {
		sev := osv.ClassifySeverity(v.Severity)
		switch sev {
		case "CRITICAL":
			result.Counts["critical"]++
		case "HIGH":
			result.Counts["high"]++
		case "MODERATE":
			result.Counts["moderate"]++
		case "LOW":
			result.Counts["low"]++
		default:
			result.Counts["unknown"]++
		}

		module := vulnToModule[v.VulnID]
		detail := fmt.Sprintf("%s: %s (%s) %s", module, v.VulnID, sev, v.Summary)
		result.Details = append(result.Details, detail)
	}
	result.TruncateDetails()

	if result.Counts["critical"] > 0 || result.Counts["high"] > 0 {
		result.Status = StatusFAIL
		result.Summary = fmt.Sprintf("Found vulnerabilities: %d critical, %d high, %d moderate, %d low, %d unknown",
			result.Counts["critical"], result.Counts["high"],
			result.Counts["moderate"], result.Counts["low"], result.Counts["unknown"])
	} else if result.Counts["moderate"] > 0 || result.Counts["low"] > 0 || result.Counts["unknown"] > 0 {
		result.Status = StatusWARN
		result.Summary = fmt.Sprintf("Found low-severity vulnerabilities: %d moderate, %d low, %d unknown",
			result.Counts["moderate"], result.Counts["low"], result.Counts["unknown"])
	} else {
		result.Status = StatusPASS
		result.Summary = fmt.Sprintf("No vulnerabilities found in %d modules", len(queries))
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

func tryGovulncheck(ctx context.Context, git *gitops.Ops, timeout time.Duration, maxOutput int, result *CheckResult) *CheckResult {
	r := runner.Run(ctx, git.RepoPath, timeout, maxOutput, "govulncheck", "./...")
	if r.Err != nil {
		result.Status = StatusUNAVAILABLE
		result.Summary = "Neither OSV.dev nor govulncheck available"
		return result
	}
	if r.ExitCode == 0 {
		result.Status = StatusPASS
		result.Summary = "govulncheck: no vulnerabilities found"
	} else {
		result.Status = StatusWARN
		result.Summary = "govulncheck found issues"
		lines := strings.Split(strings.TrimSpace(r.Stdout), "\n")
		result.Details = lines
		result.TruncateDetails()
	}
	return result
}
