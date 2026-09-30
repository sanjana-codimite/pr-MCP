// Package checks provides the common CheckResult type and check implementations
// (tests, lint, dependencies, todos) for the PR Readiness server.
//
// MCP concept: each check is exposed as a Tool — model-controlled actions that
// the LLM can invoke to gather information about the PR.
package checks

// CheckResult is the common structure returned by every check tool.
// Field names are stable and serialized with json tags for the MCP protocol.
type CheckResult struct {
	// Check identifies which check produced this result.
	// Values: "checkout", "tests", "lint", "dependencies", "todos", "report", "repo_status".
	Check string `json:"check"`
	// Status is the outcome: PASS, FAIL, WARN, ERROR, or UNAVAILABLE.
	Status string `json:"status"`
	// Summary is a single human-readable line describing the outcome.
	Summary string `json:"summary"`
	// Counts holds numeric metrics (e.g., {"passed": 42, "failed": 0}).
	Counts map[string]int `json:"counts"`
	// Details is a capped list of findings (max 20 entries).
	Details []string `json:"details"`
	// Branch is the git branch at the time of the check.
	Branch string `json:"branch"`
	// Commit is the short SHA of HEAD when the check ran.
	Commit string `json:"commit"`
	// DurationMs is how long the check took in milliseconds.
	DurationMs int64 `json:"duration_ms"`
}

// Status constants.
const (
	StatusPASS        = "PASS"
	StatusFAIL        = "FAIL"
	StatusWARN        = "WARN"
	StatusERROR       = "ERROR"
	StatusUNAVAILABLE = "UNAVAILABLE"
)

// MaxDetails is the maximum number of entries in the Details slice.
const MaxDetails = 20

// TruncateDetails ensures Details has at most MaxDetails entries.
func (r *CheckResult) TruncateDetails() {
	if len(r.Details) > MaxDetails {
		r.Details = r.Details[:MaxDetails]
	}
}
