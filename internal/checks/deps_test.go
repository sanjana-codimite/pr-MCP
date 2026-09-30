package checks

import (
	"testing"

	"golang.org/x/mod/modfile"
)

func TestGoModParsing(t *testing.T) {
	goModContent := `module example.com/demo

go 1.21

require (
	golang.org/x/text v0.3.0
	golang.org/x/crypto v0.14.0
)

require (
	golang.org/x/sys v0.13.0 // indirect
)

replace golang.org/x/text => golang.org/x/text v0.3.1
`

	f, err := modfile.Parse("go.mod", []byte(goModContent), nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(f.Require) != 3 {
		t.Errorf("got %d requires, want 3", len(f.Require))
	}

	// Check that we find direct and indirect.
	var hasText, hasCrypto, hasSys bool
	for _, req := range f.Require {
		switch req.Mod.Path {
		case "golang.org/x/text":
			hasText = true
			if req.Mod.Version != "v0.3.0" {
				t.Errorf("text version = %q, want %q", req.Mod.Version, "v0.3.0")
			}
		case "golang.org/x/crypto":
			hasCrypto = true
		case "golang.org/x/sys":
			hasSys = true
			if !req.Indirect {
				t.Error("sys should be marked indirect")
			}
		}
	}

	if !hasText || !hasCrypto || !hasSys {
		t.Errorf("missing expected modules: text=%v crypto=%v sys=%v", hasText, hasCrypto, hasSys)
	}

	// Check replace directive.
	if len(f.Replace) != 1 {
		t.Errorf("got %d replaces, want 1", len(f.Replace))
	}
}
