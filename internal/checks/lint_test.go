package checks

import (
	"testing"
)

func TestParseLintOutput_GoVet(t *testing.T) {
	output := `# example.com/pkg
./main.go:10:5: Printf format %d has arg str of wrong type string
./main.go:15:2: unreachable code
`
	findings := parseLintOutput(output)
	if len(findings) != 2 {
		t.Errorf("got %d findings, want 2: %v", len(findings), findings)
	}
}

func TestParseLintOutput_NoFindings(t *testing.T) {
	output := ``
	findings := parseLintOutput(output)
	if len(findings) != 0 {
		t.Errorf("got %d findings, want 0", len(findings))
	}
}

func TestParseLintOutput_MixedOutput(t *testing.T) {
	output := `Some info line
./file.go:5: shadow: declaration of "err" shadows declaration
Not a finding line
./file.go:12:3: can't use X as Y
`
	findings := parseLintOutput(output)
	if len(findings) != 2 {
		t.Errorf("got %d findings, want 2: %v", len(findings), findings)
	}
}
