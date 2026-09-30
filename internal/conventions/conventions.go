// Package conventions reads CONTRIBUTING.md and README.md from the target repo
// to provide convention context.
//
// MCP concept: This data is exposed as a Resource (user/app-attached context),
// not a Tool. Resources are read-only context that the host can attach to
// the conversation.
package conventions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaxSize is the maximum total size of the conventions content (64 KB).
const MaxSize = 64 * 1024

// Read concatenates CONTRIBUTING.md and README.md from the repo.
// Each is placed under a heading with its file name.
// If a file is missing, a note is included instead.
func Read(repoPath string) string {
	var sb strings.Builder

	for _, name := range []string{"CONTRIBUTING.md", "README.md"} {
		sb.WriteString(fmt.Sprintf("# %s\n\n", name))

		path := filepath.Join(repoPath, name)
		data, err := os.ReadFile(path)
		if err != nil {
			sb.WriteString(fmt.Sprintf("*File %s not found in the repository.*\n\n", name))
			continue
		}

		content := string(data)
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	result := sb.String()
	if len(result) > MaxSize {
		result = result[:MaxSize]
	}
	return result
}
