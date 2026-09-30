package checks

import (
	"testing"
)

func TestParseGoTestJSON_AllPass(t *testing.T) {
	input := `{"Action":"run","Package":"example.com/pkg","Test":"TestAdd"}
{"Action":"output","Package":"example.com/pkg","Test":"TestAdd","Output":"--- PASS: TestAdd (0.00s)\n"}
{"Action":"pass","Package":"example.com/pkg","Test":"TestAdd","Elapsed":0.01}
{"Action":"run","Package":"example.com/pkg","Test":"TestSub"}
{"Action":"output","Package":"example.com/pkg","Test":"TestSub","Output":"--- PASS: TestSub (0.00s)\n"}
{"Action":"pass","Package":"example.com/pkg","Test":"TestSub","Elapsed":0.01}
{"Action":"pass","Package":"example.com/pkg","Elapsed":0.02}
`

	result := &CheckResult{
		Check:  "tests",
		Counts: map[string]int{"passed": 0, "failed": 0, "skipped": 0},
	}

	parseGoTestJSON(input, result)

	if result.Status != StatusPASS {
		t.Errorf("Status = %q, want %q", result.Status, StatusPASS)
	}
	if result.Counts["passed"] != 2 {
		t.Errorf("passed = %d, want 2", result.Counts["passed"])
	}
	if result.Counts["failed"] != 0 {
		t.Errorf("failed = %d, want 0", result.Counts["failed"])
	}
}

func TestParseGoTestJSON_WithFailures(t *testing.T) {
	input := `{"Action":"run","Package":"example.com/pkg","Test":"TestAdd"}
{"Action":"output","Package":"example.com/pkg","Test":"TestAdd","Output":"--- PASS: TestAdd (0.00s)\n"}
{"Action":"pass","Package":"example.com/pkg","Test":"TestAdd","Elapsed":0.01}
{"Action":"run","Package":"example.com/pkg","Test":"TestBroken"}
{"Action":"output","Package":"example.com/pkg","Test":"TestBroken","Output":"    broken_test.go:10: expected 4, got 5\n"}
{"Action":"fail","Package":"example.com/pkg","Test":"TestBroken","Elapsed":0.01}
{"Action":"run","Package":"example.com/pkg","Test":"TestSkipped"}
{"Action":"output","Package":"example.com/pkg","Test":"TestSkipped","Output":"--- SKIP: TestSkipped (0.00s)\n"}
{"Action":"skip","Package":"example.com/pkg","Test":"TestSkipped","Elapsed":0.01}
{"Action":"fail","Package":"example.com/pkg","Elapsed":0.03}
`

	result := &CheckResult{
		Check:  "tests",
		Counts: map[string]int{"passed": 0, "failed": 0, "skipped": 0},
	}

	parseGoTestJSON(input, result)

	if result.Status != StatusFAIL {
		t.Errorf("Status = %q, want %q", result.Status, StatusFAIL)
	}
	if result.Counts["passed"] != 1 {
		t.Errorf("passed = %d, want 1", result.Counts["passed"])
	}
	if result.Counts["failed"] != 1 {
		t.Errorf("failed = %d, want 1", result.Counts["failed"])
	}
	if result.Counts["skipped"] != 1 {
		t.Errorf("skipped = %d, want 1", result.Counts["skipped"])
	}
	if len(result.Details) == 0 {
		t.Error("expected details for failed test")
	}
}

func TestParseGoTestJSON_CompileFailure(t *testing.T) {
	input := `{"Action":"output","Package":"example.com/pkg","Output":"# example.com/pkg\n"}
{"Action":"output","Package":"example.com/pkg","Output":"./main.go:10:5: undefined: foo\n"}
{"Action":"fail","Package":"example.com/pkg","Elapsed":0.1}
`

	result := &CheckResult{
		Check:  "tests",
		Counts: map[string]int{"passed": 0, "failed": 0, "skipped": 0},
	}

	parseGoTestJSON(input, result)

	if result.Status != StatusFAIL {
		t.Errorf("Status = %q, want %q", result.Status, StatusFAIL)
	}
}
